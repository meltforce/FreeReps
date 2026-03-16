package demo

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"time"

	"github.com/claude/freereps/internal/models"
	"github.com/claude/freereps/internal/storage"
	"github.com/google/uuid"
)

const (
	userID    = 1
	source    = "demo"
	daysBack  = 90
	randSeed  = 42
)

// Seed populates the database with 90 days of realistic health data for
// testing and App Store review. Data is deterministic (seeded RNG) and
// idempotent (all inserts use ON CONFLICT DO NOTHING).
func Seed(ctx context.Context, db *storage.DB, log *slog.Logger) error {
	rng := rand.New(rand.NewSource(randSeed))
	now := time.Now().Truncate(24 * time.Hour)
	start := now.AddDate(0, 0, -daysBack)

	log.Info("demo: seeding database", "days", daysBack, "start", start.Format("2006-01-02"))

	// Health metrics (batch to stay under PostgreSQL 65535 param limit; 12 params/row → 5000 rows/batch)
	metrics := generateHealthMetrics(rng, start, now)
	var totalInserted int64
	const metricBatchSize = 5000
	for i := 0; i < len(metrics); i += metricBatchSize {
		end := i + metricBatchSize
		if end > len(metrics) {
			end = len(metrics)
		}
		n, err := db.InsertHealthMetrics(ctx, metrics[i:end])
		if err != nil {
			return fmt.Errorf("demo: insert health metrics batch %d: %w", i/metricBatchSize, err)
		}
		totalInserted += n
	}
	log.Info("demo: health metrics", "generated", len(metrics), "inserted", totalInserted)

	// Sleep
	sessions, stages := generateSleep(rng, start, now)
	for _, s := range sessions {
		if err := db.InsertSleepSession(ctx, s); err != nil {
			return fmt.Errorf("demo: insert sleep session: %w", err)
		}
	}
	stagesInserted, err := db.InsertSleepStages(ctx, stages)
	if err != nil {
		return fmt.Errorf("demo: insert sleep stages: %w", err)
	}
	log.Info("demo: sleep", "sessions", len(sessions), "stages_inserted", stagesInserted)

	// Workouts
	workouts, workoutHR := generateWorkouts(rng, start, now)
	for _, w := range workouts {
		if _, err := db.InsertWorkout(ctx, w); err != nil {
			return fmt.Errorf("demo: insert workout: %w", err)
		}
	}
	var hrInserted int64
	const hrBatchSize = 9000
	for i := 0; i < len(workoutHR); i += hrBatchSize {
		end := i + hrBatchSize
		if end > len(workoutHR) {
			end = len(workoutHR)
		}
		n, err := db.InsertWorkoutHeartRate(ctx, workoutHR[i:end])
		if err != nil {
			return fmt.Errorf("demo: insert workout HR batch %d: %w", i/hrBatchSize, err)
		}
		hrInserted += n
	}
	log.Info("demo: workouts", "count", len(workouts), "hr_samples", hrInserted)

	// Activity summaries
	activities := generateActivitySummaries(rng, start, now)
	actInserted, err := db.InsertActivitySummaries(ctx, activities)
	if err != nil {
		return fmt.Errorf("demo: insert activity summaries: %w", err)
	}
	log.Info("demo: activity summaries", "generated", len(activities), "inserted", actInserted)

	log.Info("demo: seeding complete")
	return nil
}

// floatPtr returns a pointer to a float64.
func floatPtr(v float64) *float64 { return &v }

// generateHealthMetrics creates heart_rate, resting_heart_rate, step_count,
// active_energy, weight_body_mass, blood_oxygen_saturation, and
// respiratory_rate data points.
func generateHealthMetrics(rng *rand.Rand, start, end time.Time) []models.HealthMetricRow {
	var rows []models.HealthMetricRow

	baseWeight := 75.0
	weightDrift := 0.0

	for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
		dayOfYear := d.YearDay()

		// Heart rate — every 5 minutes during waking hours (8am–11pm)
		for h := 8; h < 23; h++ {
			for m := 0; m < 60; m += 5 {
				t := d.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute)
				// Base HR varies by time of day: lower in morning/evening, higher midday
				baseHR := 65.0 + 10.0*math.Sin(float64(h-8)/15.0*math.Pi)
				hr := baseHR + rng.Float64()*20 - 10
				hr = math.Max(50, math.Min(100, hr))
				rows = append(rows, models.HealthMetricRow{
					Time:       t,
					UserID:     userID,
					MetricName: "heart_rate",
					Source:     source,
					Units:      "bpm",
					Qty:        floatPtr(math.Round(hr)),
				})
			}
		}

		// Resting heart rate — daily
		rhr := 55.0 + rng.Float64()*10
		rows = append(rows, models.HealthMetricRow{
			Time:       d.Add(8 * time.Hour),
			UserID:     userID,
			MetricName: "resting_heart_rate",
			Source:     source,
			Units:      "bpm",
			Qty:        floatPtr(math.Round(rhr)),
		})

		// Step count — hourly during waking hours
		dailyStepTarget := 4000.0 + rng.Float64()*8000
		for h := 7; h < 22; h++ {
			t := d.Add(time.Duration(h) * time.Hour)
			// More steps during commute hours and lunch
			hourWeight := 1.0
			if h == 8 || h == 9 || h == 12 || h == 13 || h == 17 || h == 18 {
				hourWeight = 2.0
			}
			steps := (dailyStepTarget / 21.0) * hourWeight * (0.5 + rng.Float64())
			rows = append(rows, models.HealthMetricRow{
				Time:       t,
				UserID:     userID,
				MetricName: "step_count",
				Source:     source,
				Units:      "steps",
				Qty:        floatPtr(math.Round(steps)),
			})
		}

		// Active energy — hourly
		for h := 7; h < 22; h++ {
			t := d.Add(time.Duration(h) * time.Hour)
			cal := 20.0 + rng.Float64()*40
			rows = append(rows, models.HealthMetricRow{
				Time:       t,
				UserID:     userID,
				MetricName: "active_energy",
				Source:     source,
				Units:      "kcal",
				Qty:        floatPtr(math.Round(cal*10) / 10),
			})
		}

		// Weight — weekly (every Sunday)
		if d.Weekday() == time.Sunday {
			weightDrift += (rng.Float64() - 0.5) * 0.3
			weight := baseWeight + weightDrift + (rng.Float64()-0.5)*0.2
			rows = append(rows, models.HealthMetricRow{
				Time:       d.Add(7 * time.Hour),
				UserID:     userID,
				MetricName: "weight_body_mass",
				Source:     source,
				Units:      "kg",
				Qty:        floatPtr(math.Round(weight*10) / 10),
			})
		}

		// Blood oxygen — daily
		spo2 := 96.0 + rng.Float64()*3
		rows = append(rows, models.HealthMetricRow{
			Time:       d.Add(3 * time.Hour),
			UserID:     userID,
			MetricName: "blood_oxygen_saturation",
			Source:     source,
			Units:      "%",
			Qty:        floatPtr(math.Round(spo2*10) / 10),
		})

		// Respiratory rate — daily
		rr := 13.0 + rng.Float64()*4
		rows = append(rows, models.HealthMetricRow{
			Time:       d.Add(3*time.Hour + 30*time.Minute),
			UserID:     userID,
			MetricName: "respiratory_rate",
			Source:     source,
			Units:      "breaths/min",
			Qty:        floatPtr(math.Round(rr*10) / 10),
		})

		// HRV — daily (tied to resting HR inversely)
		hrv := 40.0 + rng.Float64()*30 - (rhr-55)*0.5
		hrv = math.Max(15, math.Min(80, hrv))
		rows = append(rows, models.HealthMetricRow{
			Time:       d.Add(7*time.Hour + 30*time.Minute),
			UserID:     userID,
			MetricName: "heart_rate_variability",
			Source:     source,
			Units:      "ms",
			Qty:        floatPtr(math.Round(hrv*10) / 10),
		})

		// VO2 Max — weekly on Mondays, with a slight upward trend
		if d.Weekday() == time.Monday {
			weekNum := float64(dayOfYear) / 7.0
			vo2 := 42.0 + weekNum*0.1 + (rng.Float64()-0.5)*2
			rows = append(rows, models.HealthMetricRow{
				Time:       d.Add(18 * time.Hour),
				UserID:     userID,
				MetricName: "vo2_max",
				Source:     source,
				Units:      "mL/kg/min",
				Qty:        floatPtr(math.Round(vo2*10) / 10),
			})
		}
	}

	return rows
}

// generateSleep creates sleep sessions and stage breakdowns for each night.
func generateSleep(rng *rand.Rand, start, end time.Time) ([]models.SleepSessionRow, []models.SleepStageRow) {
	var sessions []models.SleepSessionRow
	var stages []models.SleepStageRow

	for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
		// Bedtime: 22:00–23:30
		bedHour := 22.0 + rng.Float64()*1.5
		bedTime := d.Add(time.Duration(bedHour * float64(time.Hour)))

		// Total sleep: 6.5–8.5 hours
		totalHours := 6.5 + rng.Float64()*2.0
		wakeTime := bedTime.Add(time.Duration(totalHours * float64(time.Hour)))

		// In bed slightly longer than asleep (fall asleep delay + morning linger)
		fallAsleepMin := 5.0 + rng.Float64()*20
		morningLingerMin := 5.0 + rng.Float64()*15
		inBedStart := bedTime.Add(-time.Duration(fallAsleepMin * float64(time.Minute)))
		inBedEnd := wakeTime.Add(time.Duration(morningLingerMin * float64(time.Minute)))
		inBedHours := inBedEnd.Sub(inBedStart).Hours()

		// Stage breakdown (approximate percentages of total sleep)
		deepPct := 0.13 + rng.Float64()*0.07  // 13-20%
		remPct := 0.20 + rng.Float64()*0.05   // 20-25%
		awakePct := 0.02 + rng.Float64()*0.03 // 2-5%
		corePct := 1.0 - deepPct - remPct - awakePct

		deepHours := totalHours * deepPct
		remHours := totalHours * remPct
		coreHours := totalHours * corePct
		asleepHours := deepHours + remHours + coreHours

		sessionDate := d
		if bedHour >= 24 {
			sessionDate = d.AddDate(0, 0, 1)
		}

		sessions = append(sessions, models.SleepSessionRow{
			UserID:     userID,
			Date:       sessionDate,
			TotalSleep: totalHours,
			Asleep:     asleepHours,
			Core:       coreHours,
			Deep:       deepHours,
			REM:        remHours,
			InBed:      inBedHours,
			SleepStart: bedTime,
			SleepEnd:   wakeTime,
			InBedStart: inBedStart,
			InBedEnd:   inBedEnd,
		})

		// Generate stage segments — cycle through sleep stages
		cursor := bedTime
		cycleLen := totalHours / 4.5 // ~4-5 cycles per night
		for cursor.Before(wakeTime) {
			cycleFraction := wakeTime.Sub(cursor).Hours() / totalHours

			// Each cycle: Core → Deep → Core → REM, with occasional Awake
			cycleStages := []struct {
				stage    string
				fraction float64
			}{
				{models.SleepStageCore, corePct * 0.4 * cycleLen},
				{models.SleepStageDeep, deepPct * cycleLen * math.Max(0.5, cycleFraction)},
				{models.SleepStageCore, corePct * 0.6 * cycleLen},
				{models.SleepStageREM, remPct * cycleLen * math.Min(1.5, 2-cycleFraction)},
			}

			// Occasional awake period
			if rng.Float64() < 0.3 {
				cycleStages = append(cycleStages, struct {
					stage    string
					fraction float64
				}{models.SleepStageAwake, awakePct * cycleLen})
			}

			for _, cs := range cycleStages {
				durationHr := cs.fraction * (0.8 + rng.Float64()*0.4)
				if durationHr < 0.05 {
					durationHr = 0.05
				}
				stageEnd := cursor.Add(time.Duration(durationHr * float64(time.Hour)))
				if stageEnd.After(wakeTime) {
					stageEnd = wakeTime
				}
				if cursor.Equal(stageEnd) || cursor.After(stageEnd) {
					break
				}

				stages = append(stages, models.SleepStageRow{
					StartTime:  cursor,
					EndTime:    stageEnd,
					UserID:     userID,
					Stage:      cs.stage,
					DurationHr: stageEnd.Sub(cursor).Hours(),
					Source:     source,
				})
				cursor = stageEnd
			}
		}
	}

	return sessions, stages
}

// workoutTemplate defines parameters for generating a specific workout type.
type workoutTemplate struct {
	name        string
	minDuration float64 // seconds
	maxDuration float64
	minHR       float64
	maxHR       float64
	hasDistance  bool
	minDist     float64 // km
	maxDist     float64
	indoor      bool
	calPerSec   float64
}

var workoutTemplates = []workoutTemplate{
	{"Running", 1200, 3600, 130, 175, true, 3, 15, false, 0.15},
	{"Walking", 1800, 5400, 90, 130, true, 2, 8, false, 0.06},
	{"Cycling", 1800, 5400, 110, 165, true, 10, 40, false, 0.12},
	{"Traditional Strength Training", 2400, 4800, 100, 155, false, 0, 0, true, 0.08},
}

// generateWorkouts creates workout sessions with heart rate data.
func generateWorkouts(rng *rand.Rand, start, end time.Time) ([]models.WorkoutRow, []models.WorkoutHRRow) {
	var workouts []models.WorkoutRow
	var hrRows []models.WorkoutHRRow

	// Generate ~45 workouts over 90 days (every other day, roughly)
	for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
		if rng.Float64() > 0.5 {
			continue
		}

		tmpl := workoutTemplates[rng.Intn(len(workoutTemplates))]

		// Workout time: 6am–7pm
		startHour := 6.0 + rng.Float64()*13
		workoutStart := d.Add(time.Duration(startHour * float64(time.Hour)))
		duration := tmpl.minDuration + rng.Float64()*(tmpl.maxDuration-tmpl.minDuration)
		workoutEnd := workoutStart.Add(time.Duration(duration) * time.Second)

		cal := duration * tmpl.calPerSec * (0.8 + rng.Float64()*0.4)
		avgHR := tmpl.minHR + (tmpl.maxHR-tmpl.minHR)*0.5 + (rng.Float64()-0.5)*20
		maxHR := avgHR + 10 + rng.Float64()*15
		minHR := avgHR - 15 - rng.Float64()*10
		minHR = math.Max(tmpl.minHR-10, minHR)

		indoor := tmpl.indoor
		wID := uuid.NewSHA1(uuid.NameSpaceDNS, []byte(fmt.Sprintf("demo-workout-%s", workoutStart.Format(time.RFC3339))))

		w := models.WorkoutRow{
			ID:                 wID,
			UserID:             userID,
			Name:               tmpl.name,
			StartTime:          workoutStart,
			EndTime:            workoutEnd,
			DurationSec:        duration,
			ActiveEnergyBurned: floatPtr(math.Round(cal)),
			ActiveEnergyUnits:  "kcal",
			AvgHeartRate:       floatPtr(math.Round(avgHR)),
			MaxHeartRate:       floatPtr(math.Round(maxHR)),
			MinHeartRate:       floatPtr(math.Round(minHR)),
			IsIndoor:           &indoor,
		}

		if tmpl.hasDistance {
			dist := tmpl.minDist + rng.Float64()*(tmpl.maxDist-tmpl.minDist)
			w.Distance = floatPtr(math.Round(dist*100) / 100)
			w.DistanceUnits = "km"
		}

		workouts = append(workouts, w)

		// Heart rate samples every 5 seconds during workout
		for t := workoutStart; t.Before(workoutEnd); t = t.Add(5 * time.Second) {
			elapsed := t.Sub(workoutStart).Seconds()
			progress := elapsed / duration

			// HR ramps up in first 5 min, fluctuates, then drops in last 5 min
			var hr float64
			switch {
			case progress < 0.08: // warm up
				hr = minHR + (avgHR-minHR)*progress/0.08
			case progress > 0.92: // cool down
				hr = avgHR - (avgHR-minHR)*(progress-0.92)/0.08
			default:
				hr = avgHR + (rng.Float64()-0.5)*20
			}
			hr = math.Max(minHR, math.Min(maxHR, hr))

			hrRows = append(hrRows, models.WorkoutHRRow{
				Time:      t,
				WorkoutID: wID,
				UserID:    userID,
				AvgBPM:    floatPtr(math.Round(hr)),
				Source:    source,
			})
		}
	}

	return workouts, hrRows
}

// generateActivitySummaries creates daily activity ring data.
func generateActivitySummaries(rng *rand.Rand, start, end time.Time) []models.ActivitySummaryRow {
	var rows []models.ActivitySummaryRow

	for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
		isWeekend := d.Weekday() == time.Saturday || d.Weekday() == time.Sunday

		// Move goal: 500-700 kcal
		moveGoal := 500.0 + rng.Float64()*200
		moveFraction := 0.7 + rng.Float64()*0.5 // 70-120% of goal
		if isWeekend {
			moveFraction *= 0.8 + rng.Float64()*0.4
		}
		move := moveGoal * moveFraction

		// Exercise goal: 30 min
		exerciseGoal := 30.0
		exercise := 15.0 + rng.Float64()*35 // 15-50 min

		// Stand goal: 12 hours
		standGoal := 12.0
		stand := 8.0 + rng.Float64()*6 // 8-14 hours

		rows = append(rows, models.ActivitySummaryRow{
			UserID:           userID,
			Date:             d,
			ActiveEnergy:     floatPtr(math.Round(move)),
			ActiveEnergyGoal: floatPtr(moveGoal),
			ExerciseTime:     floatPtr(math.Round(exercise)),
			ExerciseTimeGoal: floatPtr(exerciseGoal),
			StandHours:       floatPtr(math.Round(stand)),
			StandHoursGoal:   floatPtr(standGoal),
		})
	}

	return rows
}
