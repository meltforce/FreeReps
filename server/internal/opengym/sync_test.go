package opengym

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/claude/freereps/internal/models"
	"github.com/claude/freereps/internal/storage"
)

// fakeStore keeps the openGym tables and workout_sets in memory.
type fakeStore struct {
	creds    *storage.OpenGymCredentials
	state    *storage.OpenGymSyncState
	hashes   map[string]string
	sets     map[string][]models.WorkoutSetRow // by external id
	catalog  map[string]storage.MappedExercise
	logs     []storage.ImportLog
	rebuilds int
}

func newFakeStore(t *testing.T) *fakeStore {
	cat := map[string]storage.MappedExercise{}
	for k, e := range testCatalog {
		cat[k] = storage.MappedExercise{TemplateID: e.TemplateID, Title: e.Title, ExerciseType: e.ExerciseType}
	}
	return &fakeStore{
		creds: &storage.OpenGymCredentials{
			UserID: 1, BaseURL: "https://gym.test", Token: "tok-1",
			SyncFrom: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		},
		hashes:  map[string]string{},
		sets:    map[string][]models.WorkoutSetRow{},
		catalog: cat,
	}
}

func (f *fakeStore) ListOpenGymCredentialUsers(context.Context) ([]int, error) { return []int{1}, nil }
func (f *fakeStore) GetOpenGymCredentials(context.Context, int) (*storage.OpenGymCredentials, error) {
	return f.creds, nil
}
func (f *fakeStore) UpdateOpenGymToken(_ context.Context, _ int, token string) error {
	f.creds.Token = token
	return nil
}
func (f *fakeStore) GetOpenGymSyncState(context.Context, int) (*storage.OpenGymSyncState, error) {
	return f.state, nil
}
func (f *fakeStore) UpsertOpenGymSyncState(_ context.Context, _ int, rev int64, wid string) error {
	f.state = &storage.OpenGymSyncState{Rev: rev, WID: wid}
	return nil
}
func (f *fakeStore) ListOpenGymWorkoutHashes(context.Context, int) (map[string]string, error) {
	out := map[string]string{}
	for k, v := range f.hashes {
		out[k] = v
	}
	return out, nil
}
func (f *fakeStore) UpsertOpenGymWorkoutHash(_ context.Context, _ int, id, hash string) error {
	f.hashes[id] = hash
	return nil
}
func (f *fakeStore) DeleteOpenGymWorkoutHash(_ context.Context, _ int, id string) error {
	delete(f.hashes, id)
	return nil
}
func (f *fakeStore) MappedExercisesBySource(context.Context, string) (map[string]storage.MappedExercise, error) {
	return f.catalog, nil
}
func (f *fakeStore) DeleteWorkoutSetsByExternalID(_ context.Context, _ int, _ string, id string) (int64, error) {
	n := int64(len(f.sets[id]))
	delete(f.sets, id)
	return n, nil
}
func (f *fakeStore) InsertWorkoutSets(_ context.Context, rows []models.WorkoutSetRow) (int64, error) {
	for _, r := range rows {
		f.sets[r.ExternalID] = append(f.sets[r.ExternalID], r)
	}
	return int64(len(rows)), nil
}
func (f *fakeStore) RebuildTrainingMetrics(context.Context, int, time.Time, time.Time) (int64, error) {
	f.rebuilds++
	return 1, nil
}
func (f *fakeStore) InsertImportLog(_ context.Context, l storage.ImportLog) (int64, error) {
	f.logs = append(f.logs, l)
	return int64(len(f.logs)), nil
}

func (f *fakeStore) lastLog(t *testing.T) storage.ImportLog {
	t.Helper()
	if len(f.logs) == 0 {
		t.Fatal("no import log written")
	}
	return f.logs[len(f.logs)-1]
}

// fakeAPI serves a state and counts the requests that fetch it.
type fakeAPI struct {
	me        *Me
	meErr     error
	rev       Rev
	state     State
	dataCalls int
	tokens    []string
}

func (a *fakeAPI) Me(_ context.Context, token string) (*Me, error) {
	a.tokens = append(a.tokens, token)
	if a.meErr != nil {
		return nil, a.meErr
	}
	if a.me != nil {
		return a.me, nil
	}
	return &Me{}, nil
}
func (a *fakeAPI) Rev(context.Context, string) (*Rev, error) { r := a.rev; return &r, nil }
func (a *fakeAPI) Data(context.Context, string) (*DataResponse, error) {
	a.dataCalls++
	st := a.state
	st.Rev, st.WID = a.rev.Rev, a.rev.WID
	return &DataResponse{State: st, Rev: a.rev.Rev}, nil
}

func newTestSyncer(t *testing.T, store *fakeStore, api *fakeAPI) *Syncer {
	s := NewSyncer(store, time.Minute, berlin(t), slog.New(slog.NewTextHandler(io.Discard, nil)))
	s.newClient = func(string) API { return api }
	return s
}

// stateWithFixture returns the fixture's two workouts (2026-10-08, 2026-11-05)
// plus one dated before the cutoff of 2026-10-01.
func stateWithFixture(t *testing.T) State {
	st := *loadState(t)
	old := workoutByID(t, &st, "iw2do12qenuxn")
	old.ID = "iw-before-cutoff"
	old.Start = int64p(time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC).UnixMilli())
	st.Workouts = append(st.Workouts, old)
	return st
}

func TestSyncFirstCycleStoresWorkoutsAfterCutoff(t *testing.T) {
	store := newFakeStore(t)
	api := &fakeAPI{rev: Rev{Rev: 3, WID: "w1"}, state: stateWithFixture(t)}
	newTestSyncer(t, store, api).SyncUser(context.Background(), 1)

	l := store.lastLog(t)
	if l.Status != "success" || l.Source != ImportSource {
		t.Fatalf("log = %s/%s (%v), want success/%s", l.Status, l.Source, l.ErrorMessage, ImportSource)
	}
	if l.WorkoutsReceived != 3 || l.WorkoutsInserted != 2 || l.SetsInserted != 14 {
		t.Errorf("received/inserted/sets = %d/%d/%d, want 3/2/14", l.WorkoutsReceived, l.WorkoutsInserted, l.SetsInserted)
	}
	if _, ok := store.sets["iw-before-cutoff"]; ok {
		t.Error("a workout before sync_from was stored")
	}
	if store.state == nil || store.state.Rev != 3 || store.state.WID != "w1" {
		t.Errorf("sync state = %+v, want rev 3 wid w1", store.state)
	}
	if store.rebuilds != 1 {
		t.Errorf("training metrics rebuilt %d times, want 1", store.rebuilds)
	}
}

func TestSyncUnchangedRevisionSkipsFetchButLogsRun(t *testing.T) {
	store := newFakeStore(t)
	api := &fakeAPI{rev: Rev{Rev: 3, WID: "w1"}, state: stateWithFixture(t)}
	s := newTestSyncer(t, store, api)
	s.SyncUser(context.Background(), 1)
	s.SyncUser(context.Background(), 1)

	if api.dataCalls != 1 {
		t.Errorf("state fetched %d times, want 1", api.dataCalls)
	}
	if len(store.logs) != 2 {
		t.Fatalf("logs = %d, want one per cycle", len(store.logs))
	}
	l := store.lastLog(t)
	if l.Status != "success" || l.WorkoutsInserted != 0 || !strings.Contains(string(*l.Metadata), `"unchanged":true`) {
		t.Errorf("second log = %s inserted %d metadata %s, want an unchanged success", l.Status, l.WorkoutsInserted, *l.Metadata)
	}
}

// A backup restore changes wid and may bring rev back to an earlier value.
func TestSyncChangedWIDAtSameRevisionFetches(t *testing.T) {
	store := newFakeStore(t)
	api := &fakeAPI{rev: Rev{Rev: 3, WID: "w1"}, state: stateWithFixture(t)}
	s := newTestSyncer(t, store, api)
	s.SyncUser(context.Background(), 1)
	api.rev.WID = "w2"
	s.SyncUser(context.Background(), 1)
	if api.dataCalls != 2 {
		t.Errorf("state fetched %d times, want 2", api.dataCalls)
	}
}

// A write that touches no workout (a weigh-in) moves rev; nothing is rewritten,
// so the stale alert keeps measuring new training.
func TestSyncNewRevisionWithoutWorkoutChangeStoresNothing(t *testing.T) {
	store := newFakeStore(t)
	api := &fakeAPI{rev: Rev{Rev: 3, WID: "w1"}, state: stateWithFixture(t)}
	s := newTestSyncer(t, store, api)
	s.SyncUser(context.Background(), 1)
	api.rev.Rev = 4
	s.SyncUser(context.Background(), 1)

	l := store.lastLog(t)
	if l.WorkoutsInserted != 0 || l.SetsInserted != 0 {
		t.Errorf("inserted %d workouts, %d sets, want 0/0", l.WorkoutsInserted, l.SetsInserted)
	}
	if store.rebuilds != 1 {
		t.Errorf("training metrics rebuilt %d times, want 1", store.rebuilds)
	}
}

func TestSyncEditedSetRewritesOnlyThatWorkout(t *testing.T) {
	store := newFakeStore(t)
	api := &fakeAPI{rev: Rev{Rev: 3, WID: "w1"}, state: stateWithFixture(t)}
	s := newTestSyncer(t, store, api)
	s.SyncUser(context.Background(), 1)

	// Edit the top chin-up set in a copy, so the fixture state stays untouched.
	st := stateWithFixture(t)
	for i, w := range st.Workouts {
		if w.ID == "iw2do12qenuxn" {
			entries := append([]Entry(nil), w.Entries...)
			sets := append([]Set(nil), entries[0].Sets...)
			sets[3].Reps = intp(10)
			entries[0].Sets = sets
			st.Workouts[i].Entries = entries
		}
	}
	api.state, api.rev.Rev = st, 4
	s.SyncUser(context.Background(), 1)

	l := store.lastLog(t)
	if l.WorkoutsInserted != 1 || l.SetsInserted != 4 {
		t.Errorf("inserted %d workouts, %d sets, want 1/4", l.WorkoutsInserted, l.SetsInserted)
	}
	if got := store.sets["iw2do12qenuxn"][3].Reps; got != 10 {
		t.Errorf("edited set reps = %d, want 10", got)
	}
}

func TestSyncDeletedWorkoutIsRemoved(t *testing.T) {
	store := newFakeStore(t)
	api := &fakeAPI{rev: Rev{Rev: 3, WID: "w1"}, state: stateWithFixture(t)}
	s := newTestSyncer(t, store, api)
	s.SyncUser(context.Background(), 1)

	st := stateWithFixture(t)
	kept := st.Workouts[:0]
	for _, w := range st.Workouts {
		if w.ID != "iwfp6ie5n94d" {
			kept = append(kept, w)
		}
	}
	st.Workouts = kept
	api.state, api.rev.Rev = st, 4
	s.SyncUser(context.Background(), 1)

	if _, ok := store.sets["iwfp6ie5n94d"]; ok {
		t.Error("sets of the deleted workout are still stored")
	}
	if _, ok := store.hashes["iwfp6ie5n94d"]; ok {
		t.Error("hash of the deleted workout is still stored")
	}
	if !strings.Contains(string(*store.lastLog(t).Metadata), `"workouts_deleted":1`) {
		t.Errorf("metadata = %s, want workouts_deleted 1", *store.lastLog(t).Metadata)
	}
}

func TestSyncEmptyStateDoesNotDeleteEverything(t *testing.T) {
	store := newFakeStore(t)
	api := &fakeAPI{rev: Rev{Rev: 3, WID: "w1"}, state: stateWithFixture(t)}
	s := newTestSyncer(t, store, api)
	s.SyncUser(context.Background(), 1)

	api.state, api.rev = State{}, Rev{Rev: 1, WID: "w9"}
	s.SyncUser(context.Background(), 1)

	if len(store.sets) != 2 {
		t.Errorf("stored workouts = %d, want the 2 from before", len(store.sets))
	}
	if l := store.lastLog(t); l.Status != "error" {
		t.Errorf("status = %s, want error", l.Status)
	}
}

// A refused workout makes the cycle an error and leaves (rev, wid) unset, so the
// next cycle fails again and the failure alert can count consecutive runs. The
// other workouts are stored.
func TestSyncRefusedWorkoutFailsTheCycle(t *testing.T) {
	store := newFakeStore(t)
	st := stateWithFixture(t)
	for i, w := range st.Workouts {
		if w.ID == "iw2do12qenuxn" {
			entries := append([]Entry(nil), w.Entries...)
			entries[0].ID = "0572"
			st.Workouts[i].Entries = entries
		}
	}
	api := &fakeAPI{rev: Rev{Rev: 3, WID: "w1"}, state: st}
	s := newTestSyncer(t, store, api)
	s.SyncUser(context.Background(), 1)

	l := store.lastLog(t)
	if l.Status != "error" || l.ErrorMessage == nil || !strings.Contains(*l.ErrorMessage, "assisted") {
		t.Fatalf("log = %s %v, want an error naming the assisted exercise", l.Status, l.ErrorMessage)
	}
	if l.WorkoutsInserted != 1 {
		t.Errorf("workouts inserted = %d, want the 1 that maps", l.WorkoutsInserted)
	}
	if store.state != nil {
		t.Errorf("sync state = %+v, want unset after an error", store.state)
	}

	s.SyncUser(context.Background(), 1)
	if api.dataCalls != 2 {
		t.Errorf("state fetched %d times, want 2 — the failed cycle must be repeated", api.dataCalls)
	}
}

func TestSyncStoresRenewedToken(t *testing.T) {
	store := newFakeStore(t)
	api := &fakeAPI{rev: Rev{Rev: 3, WID: "w1"}, state: stateWithFixture(t), me: &Me{Token: "tok-2"}}
	newTestSyncer(t, store, api).SyncUser(context.Background(), 1)

	if store.creds.Token != "tok-2" {
		t.Errorf("stored token = %q, want the renewed tok-2", store.creds.Token)
	}
	if !strings.Contains(string(*store.lastLog(t).Metadata), `"token_renewed":true`) {
		t.Errorf("metadata = %s, want token_renewed", *store.lastLog(t).Metadata)
	}
}

func TestSyncRejectedTokenIsAnError(t *testing.T) {
	store := newFakeStore(t)
	api := &fakeAPI{meErr: &APIError{Path: "/api/me", StatusCode: 401, Message: "not signed in"}}
	newTestSyncer(t, store, api).SyncUser(context.Background(), 1)

	l := store.lastLog(t)
	if l.Status != "error" || l.ErrorMessage == nil || !strings.Contains(*l.ErrorMessage, "pair FreeReps again") {
		t.Errorf("log = %s %v, want an error asking to pair again", l.Status, l.ErrorMessage)
	}
	if api.dataCalls != 0 {
		t.Error("state fetched after a rejected token")
	}
}

func TestIsUnauthorized(t *testing.T) {
	if !IsUnauthorized(&APIError{StatusCode: 401}) {
		t.Error("401 not reported as unauthorized")
	}
	if IsUnauthorized(errors.New("dial tcp: lookup gym.test: no such host")) {
		t.Error("a DNS failure reported as unauthorized")
	}
}
