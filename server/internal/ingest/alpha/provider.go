package alpha

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/claude/freereps/internal/ingest"
	"github.com/claude/freereps/internal/models"
	"github.com/claude/freereps/internal/storage"
)

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

	return result, nil
}
