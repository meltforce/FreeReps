// Package alerts turns the contents of `import_logs` into the alert conditions
// FreeReps reports, and owns the `monitor_id` for each.
//
// The rules read the log table rather than hooking into the sync loops. Two
// reasons, and the second is the one that matters: a syncer that stopped running
// writes no log row at all, and a rule that only fires on a failed run would
// stay silent for exactly that case.
package alerts

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/claude/freereps/internal/notify"
	"github.com/claude/freereps/internal/storage"
)

// The ids FreeReps sends under. They sit in the block 9200 to 9299 so they cannot
// collide with the monitor ids of an Uptime Kuma instance writing to the same
// topic, which numbers its monitors from 1 upwards.
//
// An id stands for one condition permanently, never for one firing: a consumer
// groups messages by it, so reusing an id merges two conditions into one row or
// thread there.
const (
	// MonitorChannelTest is the id the Settings screen's "Send test message"
	// button uses. It is a real id in the block rather than a reused one, so a
	// test does not land in the thread of a condition that might be firing.
	MonitorChannelTest  = 9200
	MonitorWithingsSync = 9201
	MonitorOuraSync     = 9202
	MonitorHevySync     = 9203
	MonitorAppleIngest  = 9204
	// MonitorAppleWorkouts is its own id because the two Health Auto Export
	// automations stop independently; see checkAppleIngest.
	MonitorAppleWorkouts = 9205
)

// maxMsgLen keeps a message readable wherever it is displayed. The upstream error
// strings carry full URLs and struct paths.
const maxMsgLen = 300

// sourceCondition is one polled data source and the id its failures report to.
type sourceCondition struct {
	source    string // import_logs.source
	monitorID int
	service   string // `service` in the payload
}

var sourceConditions = []sourceCondition{
	{source: "withings_sync", monitorID: MonitorWithingsSync, service: ServiceNames[MonitorWithingsSync]},
	{source: "oura_sync", monitorID: MonitorOuraSync, service: ServiceNames[MonitorOuraSync]},
	{source: "hevy_sync", monitorID: MonitorHevySync, service: ServiceNames[MonitorHevySync]},
}

// appleIngestSource is the source the Health Auto Export REST path logs under.
// It is not polled by this server: the iPhone posts when its automation runs, so
// the condition is silence rather than a failed run.
const appleIngestSource = "hae_rest"

// Store is the part of storage the rules read and write.
type Store interface {
	GetAlertSettings(ctx context.Context) (storage.AlertSettings, bool, error)
	RecentRunsBySource(ctx context.Context, source string, limit int) (map[int][]storage.SourceRun, error)
	LastStoredMetricRunAt(ctx context.Context, source string) (time.Time, bool, error)
	LastWorkoutDeliveryAt(ctx context.Context, source string) (time.Time, bool, error)
	GetAlertState(ctx context.Context, monitorID int) (storage.AlertState, error)
	SetAlertState(ctx context.Context, monitorID int, firing bool, since time.Time, msg string) error
}

// ServiceNames maps a monitor id to the `service` field the payload carries, so
// the Settings screen labels a condition with the same name the consumer sees.
var ServiceNames = map[int]string{
	MonitorChannelTest:   "freereps - channel test",
	MonitorWithingsSync:  "freereps - withings sync",
	MonitorOuraSync:      "freereps - oura sync",
	MonitorHevySync:      "freereps - hevy sync",
	MonitorAppleIngest:   "freereps - apple health metrics",
	MonitorAppleWorkouts: "freereps - apple health workouts",
}

// fallbackInterval is how often the watcher looks again when the channel is
// switched off or misconfigured. It has to keep running in that state, because a
// change made in the Settings UI is what it is waiting for.
const fallbackInterval = time.Minute

// Watcher evaluates the conditions on a ticker. Every cycle reads the settings
// row first, so a change in the Settings UI takes effect on the next cycle
// without a restart.
type Watcher struct {
	store  Store
	sender notify.Sender
	log    *slog.Logger
}

// NewWatcher wires the rules to a store and a sender.
func NewWatcher(store Store, sender notify.Sender, log *slog.Logger) *Watcher {
	return &Watcher{store: store, sender: sender, log: log}
}

// Run evaluates once immediately and then on the interval the stored settings
// name, until ctx is cancelled.
func (w *Watcher) Run(ctx context.Context) {
	for {
		next := w.Check(ctx)
		if next <= 0 {
			next = fallbackInterval
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(next):
		}
	}
}

// Check evaluates every condition once and returns how long to wait before the
// next cycle.
//
// An error on one condition is logged and the remaining conditions are still
// evaluated, because a failure to report one is not a reason to stop reporting
// the others.
func (w *Watcher) Check(ctx context.Context) time.Duration {
	st, known, err := w.store.GetAlertSettings(ctx)
	if err != nil {
		w.log.Warn("reading alert settings", "error", err)
		return fallbackInterval
	}
	if !known || !st.Enabled || st.NtfyURL == "" {
		return w.interval(st)
	}

	for _, c := range sourceConditions {
		if err := w.checkSource(ctx, st, c); err != nil {
			w.log.Warn("alert rule failed", "source", c.source, "error", err)
		}
	}
	if err := w.checkAppleIngest(ctx, st); err != nil {
		w.log.Warn("alert rule failed", "source", appleIngestSource, "channel", "metrics", "error", err)
	}
	if err := w.checkAppleWorkouts(ctx, st); err != nil {
		w.log.Warn("alert rule failed", "source", appleIngestSource, "channel", "workouts", "error", err)
	}
	return w.interval(st)
}

func (w *Watcher) interval(st storage.AlertSettings) time.Duration {
	if st.CheckInterval < fallbackInterval {
		return fallbackInterval
	}
	return st.CheckInterval
}

// SendTest posts one message on MonitorChannelTest so the Settings screen can
// prove the path end to end. It carries `status: 1`, because a test must not
// leave an open problem on the consumer's side.
func (w *Watcher) SendTest(ctx context.Context, st storage.AlertSettings, msg string) error {
	return w.sender.Send(ctx, notify.Target{URL: st.NtfyURL, Hostname: st.Hostname}, notify.Payload{
		MonitorID: MonitorChannelTest,
		Service:   ServiceNames[MonitorChannelTest],
		Status:    notify.StatusResolved,
		Msg:       msg,
	})
}

// checkSource fires when any user's most recent runs of this source are failures
// and there are at least FailureThreshold of them, and resolves once that user's
// newest run succeeded again.
func (w *Watcher) checkSource(ctx context.Context, st storage.AlertSettings, c sourceCondition) error {
	threshold := st.FailureThreshold
	if threshold < 1 {
		threshold = 1
	}
	runsByUser, err := w.store.RecentRunsBySource(ctx, c.source, threshold)
	if err != nil {
		return err
	}

	firing := false
	var reasons []string
	for _, uid := range sortedKeys(runsByUser) {
		runs := runsByUser[uid]
		if len(runs) < threshold {
			continue
		}
		allFailed := true
		for _, r := range runs {
			if r.Status == "success" {
				allFailed = false
				break
			}
		}
		if !allFailed {
			continue
		}
		firing = true
		// runs[0] is the newest; its message is the current reason.
		reasons = append(reasons, fmt.Sprintf("user %d since %s: %s",
			uid,
			runs[len(runs)-1].CreatedAt.UTC().Format(time.RFC3339),
			firstLine(runs[0].ErrorMsg)))
	}

	msg := ""
	if firing {
		msg = truncate(fmt.Sprintf("%d consecutive failed runs — %s",
			threshold, strings.Join(reasons, "; ")), maxMsgLen)
	} else {
		msg = "sync succeeded again"
	}

	return w.report(ctx, st, c.monitorID, c.service, firing, msg)
}

// checkAppleIngest fires when the Health Auto Export path has stored no new
// health metric for longer than AppleSilence.
//
// The condition counts runs that wrote at least one row, not requests. An
// automation exports a fixed window — "previous 7 days" on the phone this was
// measured on — and repeats it on every run, so a phone that stopped producing
// new samples keeps posting payloads whose rows are all duplicates. On
// 2026-09-21 that state lasted 33 hours: three deliveries of the same 24
// workouts, 0 inserted each, and the rule counting requests reported "export
// received 49m ago" throughout.
//
// It counts the metric channel alone. Health Auto Export runs one automation
// per data type and they stop independently: from 2026-09-20 10:25 the metric
// automation delivered nothing for 46 hours while the workout one kept posting
// every few hours, and the rule that accepted any stored row read those
// deliveries as proof of a live ingress and reported "export stored 12h30m ago"
// throughout. checkAppleWorkouts watches the other channel.
//
// A path that has never stored anything does not fire: on a fresh deployment
// that is the expected state, and an alert for it would arrive before the
// automation has been configured at all.
func (w *Watcher) checkAppleIngest(ctx context.Context, st storage.AlertSettings) error {
	last, ok, err := w.store.LastStoredMetricRunAt(ctx, appleIngestSource)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	silence := time.Since(last)
	firing := st.AppleSilence > 0 && silence > st.AppleSilence
	msg := fmt.Sprintf("last stored export %s (%s ago), threshold %s",
		last.UTC().Format(time.RFC3339), silence.Round(time.Minute), st.AppleSilence)
	if !firing {
		msg = fmt.Sprintf("export stored %s ago", silence.Round(time.Minute))
	}

	return w.report(ctx, st, MonitorAppleIngest, ServiceNames[MonitorAppleIngest], firing, msg)
}

// checkAppleWorkouts fires when the workout automation has posted nothing for
// longer than AppleSilence.
//
// It counts deliveries where checkAppleIngest counts stored rows, and the two
// are right for opposite reasons. Metric samples accrue continuously, so an
// absence of newly stored rows means the phone stopped producing them. Workouts
// are sporadic and the export resends a fixed window, so most deliveries store
// nothing and a stored-row rule would fire through every quiet week. What is
// observable here is whether the automation still posts.
func (w *Watcher) checkAppleWorkouts(ctx context.Context, st storage.AlertSettings) error {
	last, ok, err := w.store.LastWorkoutDeliveryAt(ctx, appleIngestSource)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	silence := time.Since(last)
	firing := st.AppleSilence > 0 && silence > st.AppleSilence
	msg := fmt.Sprintf("last workout delivery %s (%s ago), threshold %s",
		last.UTC().Format(time.RFC3339), silence.Round(time.Minute), st.AppleSilence)
	if !firing {
		msg = fmt.Sprintf("workouts delivered %s ago", silence.Round(time.Minute))
	}

	return w.report(ctx, st, MonitorAppleWorkouts, ServiceNames[MonitorAppleWorkouts], firing, msg)
}

// report sends a message only on a transition — entering the problem state or
// leaving it — and stores the new state afterwards. Repeating status 0 on every
// cycle adds nothing for a consumer that groups by monitor_id.
//
// The order matters: the state is written after the send succeeded, so a failed
// send leaves the previous state and the next check retries instead of dropping
// the alert. A check that finds no transition still refreshes `last_msg`, which
// is what a later diagnosis reads.
func (w *Watcher) report(ctx context.Context, st storage.AlertSettings, monitorID int, service string, firing bool, msg string) error {
	prev, err := w.store.GetAlertState(ctx, monitorID)
	if err != nil {
		return err
	}

	// An unknown condition that is not firing is not a recovery: nothing was
	// ever announced for it.
	changed := (prev.Known && firing != prev.Firing) || (!prev.Known && firing)

	since := prev.Since
	if changed || !prev.Known {
		since = time.Now().UTC()
	}

	if !changed {
		return w.store.SetAlertState(ctx, monitorID, firing, since, msg)
	}

	status := notify.StatusResolved
	if firing {
		status = notify.StatusProblem
	}

	p := notify.Payload{
		MonitorID: monitorID,
		Service:   service,
		Status:    status,
		Since:     since.UTC().Format(time.RFC3339),
		Msg:       msg,
	}
	target := notify.Target{URL: st.NtfyURL, Hostname: st.Hostname}
	if err := w.sender.Send(ctx, target, p); err != nil {
		return fmt.Errorf("sending alert for %s: %w", service, err)
	}
	if err := w.store.SetAlertState(ctx, monitorID, firing, since, msg); err != nil {
		return err
	}
	w.log.Info("alert state changed", "service", service, "firing", firing, "msg", msg)
	return nil
}

func sortedKeys(m map[int][]storage.SourceRun) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}

func firstLine(s string) string {
	if s == "" {
		return "no error message recorded"
	}
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
