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

	// Delete existing sets per session so re-imports always reflect the latest parser output.
	for _, s := range sessions {
		if err := p.db.DeleteWorkoutSets(ctx, s.Date, userID); err != nil {
			return nil, fmt.Errorf("deleting existing sets for session %s: %w", s.Date.Format("2006-01-02"), err)
		}
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
		result.MetricsReceived = len(allRows)
		result.MetricsInserted = inserted
		result.MetricsSkipped = int64(len(allRows)) - inserted
	}

	return result, nil
}
