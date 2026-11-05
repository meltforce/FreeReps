package opengym

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/claude/freereps/internal/models"
	"github.com/claude/freereps/internal/storage"
)

// ImportSource is the import_logs.source of a sync cycle. The alert conditions
// key on it.
const ImportSource = "opengym_sync"

// defaultInterval applies when the configured interval is not positive, which
// time.NewTicker rejects with a panic.
const defaultInterval = 30 * time.Minute

// Store is the part of storage the sync reads and writes.
type Store interface {
	ListOpenGymCredentialUsers(ctx context.Context) ([]int, error)
	GetOpenGymCredentials(ctx context.Context, userID int) (*storage.OpenGymCredentials, error)
	UpdateOpenGymToken(ctx context.Context, userID int, token string) error
	GetOpenGymSyncState(ctx context.Context, userID int) (*storage.OpenGymSyncState, error)
	UpsertOpenGymSyncState(ctx context.Context, userID int, rev int64, wid string) error
	ListOpenGymWorkoutHashes(ctx context.Context, userID int) (map[string]string, error)
	UpsertOpenGymWorkoutHash(ctx context.Context, userID int, workoutID, hash string) error
	DeleteOpenGymWorkoutHash(ctx context.Context, userID int, workoutID string) error
	MappedExercisesBySource(ctx context.Context, source string) (map[string]storage.MappedExercise, error)
	DeleteWorkoutSetsByExternalID(ctx context.Context, userID int, source, externalID string) (int64, error)
	InsertWorkoutSets(ctx context.Context, rows []models.WorkoutSetRow) (int64, error)
	RebuildTrainingMetrics(ctx context.Context, userID int, from, to time.Time) (int64, error)
	InsertImportLog(ctx context.Context, l storage.ImportLog) (int64, error)
}

// API is the part of the openGym client the sync uses, one instance per base URL.
type API interface {
	Me(ctx context.Context, token string) (*Me, error)
	Rev(ctx context.Context, token string) (*Rev, error)
	Data(ctx context.Context, token string) (*DataResponse, error)
}

// syncStats accumulates counts across one cycle for the import log.
type syncStats struct {
	unchanged        bool // (rev, wid) matched; the state was not fetched
	workoutsReceived int
	workoutsInserted int
	workoutsDeleted  int
	workoutsSkipped  int // before sync_from
	setsReceived     int
	setsInserted     int64
	tokenRenewed     bool
	errors           []string
}

// Syncer polls openGym instances and stores their workouts in workout_sets.
type Syncer struct {
	store     Store
	newClient func(baseURL string) API
	interval  time.Duration
	location  *time.Location
	log       *slog.Logger

	// mu serialises cycles: the ticker and a sync started from the Settings
	// screen would otherwise reconcile the same user concurrently.
	mu sync.Mutex
}

// NewSyncer creates the openGym sync. location is the configured session
// timezone; it places sync_from and workouts that carry only their day.
func NewSyncer(store Store, interval time.Duration, location *time.Location, log *slog.Logger) *Syncer {
	if interval <= 0 {
		interval = defaultInterval
	}
	return &Syncer{
		store:     store,
		newClient: func(baseURL string) API { return NewClient(baseURL) },
		interval:  interval,
		location:  location,
		log:       log,
	}
}

// Run syncs once immediately and then on the interval, until ctx is cancelled.
func (s *Syncer) Run(ctx context.Context) {
	s.SyncOnce(ctx)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			s.log.Info("openGym sync stopped")
			return
		case <-ticker.C:
			s.SyncOnce(ctx)
		}
	}
}

// SyncOnce runs one cycle for every user with a stored connection.
func (s *Syncer) SyncOnce(ctx context.Context) {
	users, err := s.store.ListOpenGymCredentialUsers(ctx)
	if err != nil {
		s.log.Error("listing openGym users", "error", err)
		return
	}
	for _, uid := range users {
		s.SyncUser(ctx, uid)
	}
}

// SyncUser runs one cycle for one user and writes its import log row. Every
// cycle writes one, including a cycle that found the state unchanged: the stale
// alert reads a newest run older than its threshold as a stopped sync and stays
// silent, so an unchanged cycle has to be visible as a run.
func (s *Syncer) SyncUser(ctx context.Context, userID int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	start := time.Now()
	stats := &syncStats{}
	creds, err := s.store.GetOpenGymCredentials(ctx, userID)
	if err != nil {
		s.logImport(ctx, userID, start, stats, fmt.Errorf("getting credentials: %w", err))
		return
	}
	if creds == nil {
		return
	}
	err = s.syncUser(ctx, creds, stats)
	s.logImport(ctx, userID, start, stats, err)
}

func (s *Syncer) syncUser(ctx context.Context, creds *storage.OpenGymCredentials, stats *syncStats) error {
	userID := creds.UserID
	client := s.newClient(creds.BaseURL)
	token := creds.Token

	// /api/me first: it fails fast on a revoked or expired token, and it is
	// where openGym hands out the renewed token.
	me, err := client.Me(ctx, token)
	if err != nil {
		if IsUnauthorized(err) {
			return fmt.Errorf("openGym rejected the token (expired or revoked); pair FreeReps again in Settings: %w", err)
		}
		return fmt.Errorf("validating token: %w", err)
	}
	if me.Token != "" && me.Token != token {
		if err := s.store.UpdateOpenGymToken(ctx, userID, me.Token); err != nil {
			return fmt.Errorf("storing renewed token: %w", err)
		}
		token = me.Token
		stats.tokenRenewed = true
		s.log.Info("openGym token renewed", "user_id", userID)
	}

	rev, err := client.Rev(ctx, token)
	if err != nil {
		return fmt.Errorf("reading revision: %w", err)
	}
	prev, err := s.store.GetOpenGymSyncState(ctx, userID)
	if err != nil {
		return err
	}
	if prev != nil && prev.Rev == rev.Rev && prev.WID == rev.WID {
		stats.unchanged = true
		return nil
	}

	data, err := client.Data(ctx, token)
	if err != nil {
		return fmt.Errorf("reading state: %w", err)
	}
	if err := s.reconcile(ctx, creds, &data.State, stats); err != nil {
		return err
	}

	// The pair is stored only after a cycle without errors. A refused or failed
	// workout leaves it unset, so the next cycle reads the state again and fails
	// again, which is what lets the failure reach the alert.
	if len(stats.errors) == 0 {
		// The state's own mirror is used for the pair: /api/data answers with the
		// document it read, which may be newer than the /api/data/rev answer.
		wid := data.State.WID
		if wid == "" {
			wid = rev.WID
		}
		if err := s.store.UpsertOpenGymSyncState(ctx, userID, data.Rev, wid); err != nil {
			return err
		}
	}

	if stats.setsInserted > 0 || stats.workoutsDeleted > 0 {
		from := s.cutoff(creds)
		if n, err := s.store.RebuildTrainingMetrics(ctx, userID, from, time.Now()); err != nil {
			stats.errors = append(stats.errors, err.Error())
		} else {
			s.log.Info("training metrics rebuilt", "user_id", userID, "days", n)
		}
	}
	return nil
}

// cutoff is the start of sync_from in the session timezone.
func (s *Syncer) cutoff(creds *storage.OpenGymCredentials) time.Time {
	loc := s.location
	if loc == nil {
		loc = time.UTC
	}
	y, m, d := creds.SyncFrom.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}

// reconcile brings the stored openGym workouts in line with the state: a
// workout whose mapped rows changed is rewritten, a stored workout missing from
// the state is deleted, an unchanged one is left alone.
func (s *Syncer) reconcile(ctx context.Context, creds *storage.OpenGymCredentials, st *State, stats *syncStats) error {
	userID := creds.UserID
	known, err := s.store.ListOpenGymWorkoutHashes(ctx, userID)
	if err != nil {
		return err
	}
	// openGym refuses a write that empties the state, so an empty workout list
	// next to stored workouts is a fault on that side (a failed restore, a wrong
	// profile), not a deletion of every session.
	if len(st.Workouts) == 0 && len(known) > 0 {
		return fmt.Errorf("the state lists no workouts while %d are stored; refusing to delete them", len(known))
	}

	mapped, err := s.store.MappedExercisesBySource(ctx, SourceName)
	if err != nil {
		return err
	}
	catalog := make(map[string]Exercise, len(mapped))
	for k, e := range mapped {
		catalog[k] = Exercise{TemplateID: e.TemplateID, Title: e.Title, ExerciseType: e.ExerciseType}
	}
	mapper := Mapper{UserID: userID, Location: s.location, Catalog: catalog}
	cutoff := s.cutoff(creds)

	inState := make(map[string]bool, len(st.Workouts))
	for _, w := range st.Workouts {
		if w.ID == "" {
			stats.errors = append(stats.errors, "a workout without id")
			continue
		}
		inState[w.ID] = true
		stats.workoutsReceived++

		startAt, err := mapper.SessionStart(w)
		if err != nil {
			stats.errors = append(stats.errors, err.Error())
			continue
		}
		if startAt.Before(cutoff) {
			stats.workoutsSkipped++
			continue
		}
		if err := s.storeWorkout(ctx, userID, mapper, st, w, known[w.ID], stats); err != nil {
			stats.errors = append(stats.errors, err.Error())
			s.log.Warn("openGym workout not stored", "user_id", userID, "workout_id", w.ID, "error", err)
		}
	}

	for id := range known {
		if inState[id] {
			continue
		}
		if _, err := s.store.DeleteWorkoutSetsByExternalID(ctx, userID, SourceName, id); err != nil {
			stats.errors = append(stats.errors, err.Error())
			continue
		}
		if err := s.store.DeleteOpenGymWorkoutHash(ctx, userID, id); err != nil {
			stats.errors = append(stats.errors, err.Error())
			continue
		}
		stats.workoutsDeleted++
	}
	return nil
}

// storeWorkout maps one workout and rewrites its sets when the result differs
// from what was stored. The hash covers the mapped rows rather than the raw
// workout, so a new map row or a mapper change rewrites the sets it affects.
func (s *Syncer) storeWorkout(ctx context.Context, userID int, mapper Mapper, st *State, w Workout, knownHash string, stats *syncStats) error {
	rows, err := mapper.MapWorkout(st, w)
	if err != nil {
		return err
	}
	hash, err := rowsHash(rows)
	if err != nil {
		return err
	}
	if hash == knownHash {
		return nil
	}

	// Delete before insert: ON CONFLICT DO NOTHING alone would keep a corrected
	// set's old values, and a set removed in the app would survive.
	if _, err := s.store.DeleteWorkoutSetsByExternalID(ctx, userID, SourceName, w.ID); err != nil {
		return err
	}
	inserted, err := s.store.InsertWorkoutSets(ctx, rows)
	if err != nil {
		return err
	}
	if err := s.store.UpsertOpenGymWorkoutHash(ctx, userID, w.ID, hash); err != nil {
		return err
	}
	stats.setsReceived += len(rows)
	stats.setsInserted += inserted
	stats.workoutsInserted++
	return nil
}

func rowsHash(rows []models.WorkoutSetRow) (string, error) {
	raw, err := json.Marshal(rows)
	if err != nil {
		return "", fmt.Errorf("hashing mapped rows: %w", err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// logImport writes the import log row of one cycle. A cycle with a workout that
// was refused or failed is logged as an error, so that the failure alert
// (checkSource) reports it; the workouts that did store are counted either way.
func (s *Syncer) logImport(ctx context.Context, userID int, start time.Time, stats *syncStats, syncErr error) {
	durationMs := int(time.Since(start).Milliseconds())
	status := "success"
	var errMsg *string

	if syncErr == nil && len(stats.errors) > 0 {
		syncErr = fmt.Errorf("%d workout(s) not stored, first: %s", len(stats.errors), stats.errors[0])
	}
	if syncErr != nil {
		status = "error"
		msg := syncErr.Error()
		errMsg = &msg
		s.log.Error("openGym sync failed", "user_id", userID, "error", syncErr)
	}

	payload := map[string]any{}
	if stats.unchanged {
		payload["unchanged"] = true
	}
	if len(stats.errors) > 0 {
		payload["workout_errors"] = stats.errors
	}
	if stats.workoutsDeleted > 0 {
		payload["workouts_deleted"] = stats.workoutsDeleted
	}
	if stats.workoutsSkipped > 0 {
		payload["workouts_before_cutoff"] = stats.workoutsSkipped
	}
	if stats.tokenRenewed {
		payload["token_renewed"] = true
	}
	var metadata *json.RawMessage
	if len(payload) > 0 {
		raw, _ := json.Marshal(payload)
		rm := json.RawMessage(raw)
		metadata = &rm
	}

	if _, err := s.store.InsertImportLog(ctx, storage.ImportLog{
		UserID:           userID,
		Source:           ImportSource,
		Status:           status,
		WorkoutsReceived: stats.workoutsReceived,
		WorkoutsInserted: stats.workoutsInserted,
		SetsReceived:     stats.setsReceived,
		SetsInserted:     stats.setsInserted,
		DurationMs:       &durationMs,
		ErrorMessage:     errMsg,
		Metadata:         metadata,
	}); err != nil {
		s.log.Error("failed to log openGym import", "error", err)
	}
}

// Location is the session timezone the sync places days in.
func (s *Syncer) Location() *time.Location { return s.location }
