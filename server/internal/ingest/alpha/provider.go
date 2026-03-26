package alpha

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/claude/freereps/internal/ingest"
	"github.com/claude/freereps/internal/models"
	"github.com/claude/freereps/internal/storage"
	"github.com/google/uuid"
)

// alphaWorkoutNamespace is the UUID namespace for deterministic Alpha Progression workout IDs.
var alphaWorkoutNamespace = uuid.MustParse("7ba7b810-9dad-11d1-80b4-00c04fd430c8")

// Provider processes Alpha Progression CSV exports.
type Provider struct {
	db  *storage.DB
	log *slog.Logger
}

// NewProvider creates a new Alpha Progression ingest provider.
func NewProvider(db *storage.DB, log *slog.Logger) *Provider {
	return &Provider{db: db, log: log}
}

// Ingest parses a CSV export and stores the workout set data.
func (p *Provider) Ingest(ctx context.Context, r io.Reader, userID int) (*ingest.Result, error) {
	sessions, err := Parse(r)
	if err != nil {
		return nil, fmt.Errorf("parsing CSV: %w", err)
	}

	result := &ingest.Result{}
	var allRows []models.WorkoutSetRow

	for _, s := range sessions {
		for _, ex := range s.Exercises {
			for _, set := range ex.Sets {
				allRows = append(allRows, models.WorkoutSetRow{
					UserID:           userID,
					SessionName:      s.Name,
					SessionDate:      s.Date,
					SessionDuration:  s.Duration,
					ExerciseNumber:   ex.Number,
					ExerciseName:     ex.Name,
					Equipment:        ex.Equipment,
					TargetReps:       ex.TargetReps,
					IsWarmup:         set.IsWarmup,
					SetNumber:        set.Number,
					WeightKg:         set.WeightKg,
					IsBodyweightPlus: set.IsBodyweightPlus,
					Reps:             set.Reps,
					RIR:              set.RIR,
				})
			}
		}
	}

	if len(allRows) > 0 {
		inserted, err := p.db.InsertWorkoutSets(ctx, allRows)
		if err != nil {
			return nil, fmt.Errorf("inserting sets: %w", err)
		}
		result.SetsReceived = len(allRows)
		result.SetsInserted = inserted
	}

	// Create workout entries so Alpha sessions appear on the workouts page.
	for _, s := range sessions {
		dur := parseDuration(s.Duration)
		indoor := true
		workout := models.WorkoutRow{
			ID:          uuid.NewSHA1(alphaWorkoutNamespace, []byte("alpha:"+s.Date.Format(time.RFC3339)+":"+s.Name)),
			UserID:      userID,
			Name:        s.Name,
			Source:      "Alpha Progression",
			StartTime:   s.Date,
			EndTime:     s.Date.Add(dur),
			DurationSec: dur.Seconds(),
			IsIndoor:    &indoor,
		}
		if _, err := p.db.InsertWorkout(ctx, workout); err != nil {
			p.log.Warn("inserting alpha workout", "error", err)
		} else {
			result.WorkoutsInserted++
		}
	}

	return result, nil
}

// parseDuration parses Alpha Progression duration strings like "1:02 hr" or "0:45 hr".
func parseDuration(s string) time.Duration {
	s = strings.TrimSpace(strings.TrimSuffix(s, "hr"))
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return 0
	}
	hours, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
	mins, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
	return time.Duration(hours)*time.Hour + time.Duration(mins)*time.Minute
}
