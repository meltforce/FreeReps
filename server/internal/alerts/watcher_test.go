package alerts

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/claude/freereps/internal/notify"
	"github.com/claude/freereps/internal/storage"
)

// fakeStore serves canned import_logs rows and keeps the alert state in memory,
// so the rules are exercised without a database.
type fakeStore struct {
	settings storage.AlertSettings
	known    bool
	runs     map[string]map[int][]storage.SourceRun
	last     map[string]time.Time // per source: when a delivery last stored a row
	state    map[int]storage.AlertState
}

// newFakeStore starts from a configured, enabled channel with the thresholds the
// deployed config carries.
func newFakeStore() *fakeStore {
	return &fakeStore{
		settings: storage.AlertSettings{
			Enabled:          true,
			NtfyURL:          "https://ntfy.example.com/freereps-alerts",
			Hostname:         "freereps",
			CheckInterval:    5 * time.Minute,
			FailureThreshold: 3,
			AppleSilence:     36 * time.Hour,
		},
		known: true,
		runs:  map[string]map[int][]storage.SourceRun{},
		last:  map[string]time.Time{},
		state: map[int]storage.AlertState{},
	}
}

func (f *fakeStore) GetAlertSettings(_ context.Context) (storage.AlertSettings, bool, error) {
	return f.settings, f.known, nil
}

func (f *fakeStore) RecentRunsBySource(_ context.Context, source string, limit int) (map[int][]storage.SourceRun, error) {
	out := map[int][]storage.SourceRun{}
	for uid, runs := range f.runs[source] {
		if len(runs) > limit {
			runs = runs[:limit]
		}
		out[uid] = runs
	}
	return out, nil
}

func (f *fakeStore) LastStoredRunAt(_ context.Context, source string) (time.Time, bool, error) {
	t, ok := f.last[source]
	return t, ok, nil
}

func (f *fakeStore) GetAlertState(_ context.Context, monitorID int) (storage.AlertState, error) {
	return f.state[monitorID], nil
}

func (f *fakeStore) SetAlertState(_ context.Context, monitorID int, firing bool, since time.Time, _ string) error {
	f.state[monitorID] = storage.AlertState{Firing: firing, Since: since, Known: true}
	return nil
}

// recorder captures what would be posted to the topic.
type recorder struct {
	sent    []notify.Payload
	targets []notify.Target
	err     error
}

func (r *recorder) Send(_ context.Context, target notify.Target, p notify.Payload) error {
	if r.err != nil {
		return r.err
	}
	r.targets = append(r.targets, target)
	r.sent = append(r.sent, p)
	return nil
}

func testWatcher(store Store, sender notify.Sender) *Watcher {
	return NewWatcher(store, sender, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func failedRuns(n int, msg string) []storage.SourceRun {
	out := make([]storage.SourceRun, n)
	for i := range out {
		out[i] = storage.SourceRun{
			UserID:    2,
			Status:    "error",
			ErrorMsg:  msg,
			CreatedAt: time.Now().Add(-time.Duration(i+1) * 30 * time.Minute),
		}
	}
	return out
}

// TestSourceBelowThresholdStaysSilent is the rule that keeps the channel usable.
// The Withings sync failed twice on transient DNS errors ("server misbehaving")
// in the runs preceding the 2026-09-20 outage; alerting on the first failure puts
// those into the channel as incidents.
func TestSourceBelowThresholdStaysSilent(t *testing.T) {
	store := newFakeStore()
	store.runs["withings_sync"] = map[int][]storage.SourceRun{2: failedRuns(2, "dial tcp: lookup failed")}
	rec := &recorder{}

	testWatcher(store, rec).Check(context.Background())

	for _, p := range rec.sent {
		if p.MonitorID == MonitorWithingsSync {
			t.Errorf("sent an alert after 2 failures: %+v", p)
		}
	}
}

// TestSourceAtThresholdFiresOnce covers the required payload fields and the
// deduplication: a message on the transition into the problem state, and
// nothing on the checks that follow while the state holds.
func TestSourceAtThresholdFiresOnce(t *testing.T) {
	store := newFakeStore()
	store.runs["withings_sync"] = map[int][]storage.SourceRun{
		2: failedRuns(3, "decoding token body: cannot unmarshal number"),
	}
	rec := &recorder{}
	w := testWatcher(store, rec)

	w.Check(context.Background())
	w.Check(context.Background())

	var got []notify.Payload
	for _, p := range rec.sent {
		if p.MonitorID == MonitorWithingsSync {
			got = append(got, p)
		}
	}
	if len(got) != 1 {
		t.Fatalf("sent %d alerts for the same state, want 1", len(got))
	}
	if got[0].Status != notify.StatusProblem {
		t.Errorf("status = %d, want %d", got[0].Status, notify.StatusProblem)
	}
	if got[0].Service == "" {
		t.Error("service is empty; the adapter drops such a message")
	}
	if !strings.Contains(got[0].Msg, "user 2") {
		t.Errorf("msg does not name the affected user: %q", got[0].Msg)
	}
}

// TestSourceRecovers verifies the resolution path. A condition that only ever
// sends status 0 leaves a stale alert on the consumer's side.
func TestSourceRecovers(t *testing.T) {
	store := newFakeStore()
	store.runs["withings_sync"] = map[int][]storage.SourceRun{2: failedRuns(3, "boom")}
	rec := &recorder{}
	w := testWatcher(store, rec)
	w.Check(context.Background())

	store.runs["withings_sync"] = map[int][]storage.SourceRun{2: {
		{UserID: 2, Status: "success", CreatedAt: time.Now()},
		{UserID: 2, Status: "error", ErrorMsg: "boom", CreatedAt: time.Now().Add(-30 * time.Minute)},
		{UserID: 2, Status: "error", ErrorMsg: "boom", CreatedAt: time.Now().Add(-60 * time.Minute)},
	}}
	w.Check(context.Background())

	var last notify.Payload
	for _, p := range rec.sent {
		if p.MonitorID == MonitorWithingsSync {
			last = p
		}
	}
	if last.Status != notify.StatusResolved {
		t.Errorf("status after recovery = %d, want %d", last.Status, notify.StatusResolved)
	}
}

// TestFirstCheckOnHealthySourceAnnouncesNothing covers the fresh-database case:
// a server that starts with everything working must not post a recovery for a
// problem it never reported.
func TestFirstCheckOnHealthySourceAnnouncesNothing(t *testing.T) {
	store := newFakeStore()
	store.runs["oura_sync"] = map[int][]storage.SourceRun{2: {
		{UserID: 2, Status: "success", CreatedAt: time.Now()},
	}}
	rec := &recorder{}

	testWatcher(store, rec).Check(context.Background())

	if len(rec.sent) != 0 {
		t.Errorf("sent %d messages on a healthy first check: %+v", len(rec.sent), rec.sent)
	}
}

// TestOneUsersFailureIsNotMaskedByAnother is why the rows are grouped per user.
// FreeReps is multi-user; a second user whose sync works produces successful runs
// in the same source stream.
func TestOneUsersFailureIsNotMaskedByAnother(t *testing.T) {
	store := newFakeStore()
	store.runs["hevy_sync"] = map[int][]storage.SourceRun{
		2: failedRuns(3, "hevy rejected this API key"),
		5: {{UserID: 5, Status: "success", CreatedAt: time.Now()}},
	}
	rec := &recorder{}

	testWatcher(store, rec).Check(context.Background())

	found := false
	for _, p := range rec.sent {
		if p.MonitorID == MonitorHevySync && p.Status == notify.StatusProblem {
			found = true
		}
	}
	if !found {
		t.Error("no alert although one user's sync failed three times in a row")
	}
}

// TestAppleIngestSilence covers the threshold and, in the second half, the case
// that must stay silent: a path that never stored anything is a deployment that
// has not been configured yet, not an outage.
func TestAppleIngestSilence(t *testing.T) {
	store := newFakeStore()
	store.last["hae_rest"] = time.Now().Add(-40 * time.Hour)
	rec := &recorder{}
	testWatcher(store, rec).Check(context.Background())

	found := false
	for _, p := range rec.sent {
		if p.MonitorID == MonitorAppleIngest && p.Status == notify.StatusProblem {
			found = true
		}
	}
	if !found {
		t.Error("no alert after 40 hours of silence, threshold is 36")
	}

	never := newFakeStore()
	rec2 := &recorder{}
	testWatcher(never, rec2).Check(context.Background())
	for _, p := range rec2.sent {
		if p.MonitorID == MonitorAppleIngest {
			t.Errorf("alerted on a path that never delivered: %+v", p)
		}
	}
}

// TestAppleIngestCountsStoredRowsNotDeliveries covers the state this rule was
// rewritten for: the phone keeps posting, every row is a duplicate. The store
// answers with the last delivery that wrote a row, 40 hours old, while the
// deliveries themselves continue — the alert has to fire on the data, not on the
// traffic.
func TestAppleIngestCountsStoredRowsNotDeliveries(t *testing.T) {
	store := newFakeStore()
	store.last["hae_rest"] = time.Now().Add(-40 * time.Hour)
	rec := &recorder{}
	testWatcher(store, rec).Check(context.Background())

	var msg string
	for _, p := range rec.sent {
		if p.MonitorID == MonitorAppleIngest && p.Status == notify.StatusProblem {
			msg = p.Msg
		}
	}
	if msg == "" {
		t.Fatal("no alert although nothing has been stored for 40 hours, threshold is 36")
	}
	if !strings.Contains(msg, "last stored export") {
		t.Errorf("message = %q, want it to name the last stored export", msg)
	}
}

// TestFailedSendIsRetried checks the write order: the state is stored only after
// the send succeeded, so an ntfy outage delays the alert instead of swallowing it.
func TestFailedSendIsRetried(t *testing.T) {
	store := newFakeStore()
	store.runs["oura_sync"] = map[int][]storage.SourceRun{2: failedRuns(3, "token expired")}
	rec := &recorder{err: io.ErrUnexpectedEOF}
	w := testWatcher(store, rec)

	w.Check(context.Background())
	if st := store.state[MonitorOuraSync]; st.Firing {
		t.Error("state recorded as firing although the send failed")
	}

	rec.err = nil
	w.Check(context.Background())
	if len(rec.sent) != 1 {
		t.Fatalf("sent %d alerts after the send recovered, want 1", len(rec.sent))
	}
}

// TestDisabledChannelSendsNothing covers the switch in the Settings UI: with the
// channel off the rules must not post, however red the conditions are.
func TestDisabledChannelSendsNothing(t *testing.T) {
	store := newFakeStore()
	store.settings.Enabled = false
	store.runs["withings_sync"] = map[int][]storage.SourceRun{2: failedRuns(3, "boom")}
	rec := &recorder{}

	testWatcher(store, rec).Check(context.Background())

	if len(rec.sent) != 0 {
		t.Errorf("sent %d messages while disabled", len(rec.sent))
	}
}

// TestEnabledWithoutURLSendsNothing is the other half: a row that is enabled but
// carries no target would otherwise post into an empty URL every cycle.
func TestEnabledWithoutURLSendsNothing(t *testing.T) {
	store := newFakeStore()
	store.settings.NtfyURL = ""
	store.runs["oura_sync"] = map[int][]storage.SourceRun{2: failedRuns(3, "boom")}
	rec := &recorder{}

	testWatcher(store, rec).Check(context.Background())

	if len(rec.sent) != 0 {
		t.Errorf("sent %d messages without a target", len(rec.sent))
	}
}

// TestCheckUsesTheStoredTarget verifies that the payload goes where the stored
// settings point, which is what makes an edit in the UI effective without a
// restart.
func TestCheckUsesTheStoredTarget(t *testing.T) {
	store := newFakeStore()
	store.settings.NtfyURL = "https://ntfy.elsewhere.example.com/freereps-alerts"
	store.settings.Hostname = "freereps-test"
	store.runs["hevy_sync"] = map[int][]storage.SourceRun{2: failedRuns(3, "boom")}
	rec := &recorder{}

	testWatcher(store, rec).Check(context.Background())

	if len(rec.targets) != 1 {
		t.Fatalf("sent %d messages, want 1", len(rec.targets))
	}
	if rec.targets[0].URL != "https://ntfy.elsewhere.example.com/freereps-alerts" {
		t.Errorf("target = %q, want the stored url", rec.targets[0].URL)
	}
	if rec.targets[0].Hostname != "freereps-test" {
		t.Errorf("hostname = %q, want the stored hostname", rec.targets[0].Hostname)
	}
}

// TestThresholdFromSettings checks that the threshold is read per cycle: raising
// it in the UI has to silence a condition that was firing at the lower value.
func TestThresholdFromSettings(t *testing.T) {
	store := newFakeStore()
	store.settings.FailureThreshold = 5
	store.runs["withings_sync"] = map[int][]storage.SourceRun{2: failedRuns(3, "boom")}
	rec := &recorder{}

	testWatcher(store, rec).Check(context.Background())

	for _, p := range rec.sent {
		if p.MonitorID == MonitorWithingsSync {
			t.Errorf("alerted at 3 failures although the threshold is 5: %+v", p)
		}
	}
}

// TestCheckReturnsTheConfiguredInterval covers the pacing, including the floor:
// a stored interval below a minute would poll import_logs for nothing.
func TestCheckReturnsTheConfiguredInterval(t *testing.T) {
	store := newFakeStore()
	store.settings.CheckInterval = 7 * time.Minute
	if got := testWatcher(store, &recorder{}).Check(context.Background()); got != 7*time.Minute {
		t.Errorf("interval = %s, want 7m", got)
	}

	store.settings.CheckInterval = time.Second
	if got := testWatcher(store, &recorder{}).Check(context.Background()); got != fallbackInterval {
		t.Errorf("interval = %s, want the %s floor", got, fallbackInterval)
	}
}

// TestSendTestUsesTheResolvedStatus exists because a test message with status 0
// would leave an open problem on the consumer's side for a condition that is fine.
func TestSendTestUsesTheResolvedStatus(t *testing.T) {
	store := newFakeStore()
	rec := &recorder{}
	w := testWatcher(store, rec)

	if err := w.SendTest(context.Background(), store.settings, "hello"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rec.sent) != 1 {
		t.Fatalf("sent %d messages, want 1", len(rec.sent))
	}
	if rec.sent[0].MonitorID != MonitorChannelTest {
		t.Errorf("monitor_id = %d, want %d", rec.sent[0].MonitorID, MonitorChannelTest)
	}
	if rec.sent[0].Status != notify.StatusResolved {
		t.Errorf("status = %d, want %d", rec.sent[0].Status, notify.StatusResolved)
	}
}
