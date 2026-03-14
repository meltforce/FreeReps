package storage

import (
	"context"
	"fmt"
	"math"
	"time"
)

// SleepSummaryPeriod holds aggregated sleep stats for one time period.
type SleepSummaryPeriod struct {
	Period                    string  `json:"period"`
	Nights                    int     `json:"nights"`
	AvgTotalSleepHr           float64 `json:"avg_total_sleep_hr"`
	AvgDeepHr                 float64 `json:"avg_deep_hr"`
	AvgREMHr                  float64 `json:"avg_rem_hr"`
	AvgCoreHr                 float64 `json:"avg_core_hr"`
	AvgInBedHr                float64 `json:"avg_in_bed_hr"`
	AvgEfficiencyPct          float64 `json:"avg_efficiency_pct"`
	AvgDeepPct                float64 `json:"avg_deep_pct"`
	AvgREMPct                 float64 `json:"avg_rem_pct"`
	AvgBedtime                string  `json:"avg_bedtime"`
	AvgWaketime               string  `json:"avg_waketime"`
	BedtimeConsistencyStdHr   float64 `json:"bedtime_consistency_stddev_hr"`
	WaketimeConsistencyStdHr  float64 `json:"waketime_consistency_stddev_hr"`
}

// sleepTimingRow holds raw timing data from the DB for circular mean computation.
type sleepTimingRow struct {
	period     time.Time
	sleepStart time.Time
	sleepEnd   time.Time
}

// GetSleepSummary returns aggregated sleep stats per period with circular bedtime/waketime averages.
func (db *DB) GetSleepSummary(ctx context.Context, start, end time.Time, bucket string, userID int) ([]SleepSummaryPeriod, error) {
	trunc := truncInterval(bucket)

	// Query 1: Aggregated duration/stage stats per period
	aggRows, err := db.Pool.Query(ctx,
		`SELECT date_trunc($1, date)::date AS period,
		        COUNT(*)::int AS nights,
		        AVG(total_sleep),
		        AVG(deep),
		        AVG(rem),
		        AVG(core),
		        AVG(in_bed),
		        AVG(CASE WHEN in_bed > 0 THEN total_sleep / in_bed * 100 ELSE 0 END),
		        AVG(CASE WHEN total_sleep > 0 THEN deep / total_sleep * 100 ELSE 0 END),
		        AVG(CASE WHEN total_sleep > 0 THEN rem / total_sleep * 100 ELSE 0 END)
		 FROM sleep_sessions
		 WHERE date >= $2 AND date < $3 AND user_id = $4
		 GROUP BY period
		 ORDER BY period DESC`,
		trunc, start, end, userID)
	if err != nil {
		return nil, fmt.Errorf("querying sleep summary: %w", err)
	}
	defer aggRows.Close()

	periodMap := make(map[string]*SleepSummaryPeriod)
	var periodOrder []string

	for aggRows.Next() {
		var periodTime time.Time
		var sp SleepSummaryPeriod
		if err := aggRows.Scan(&periodTime, &sp.Nights,
			&sp.AvgTotalSleepHr, &sp.AvgDeepHr, &sp.AvgREMHr, &sp.AvgCoreHr, &sp.AvgInBedHr,
			&sp.AvgEfficiencyPct, &sp.AvgDeepPct, &sp.AvgREMPct); err != nil {
			return nil, fmt.Errorf("scanning sleep summary: %w", err)
		}
		sp.Period = periodTime.Format("2006-01-02")
		periodMap[sp.Period] = &sp
		periodOrder = append(periodOrder, sp.Period)
	}
	if err := aggRows.Err(); err != nil {
		return nil, err
	}

	// Query 2: Raw sleep_start/sleep_end for circular mean computation
	timingRows, err := db.Pool.Query(ctx,
		`SELECT date_trunc($1, date)::date AS period, sleep_start, sleep_end
		 FROM sleep_sessions
		 WHERE date >= $2 AND date < $3 AND user_id = $4
		 ORDER BY period, date`,
		trunc, start, end, userID)
	if err != nil {
		return nil, fmt.Errorf("querying sleep timing: %w", err)
	}
	defer timingRows.Close()

	// Group timing rows by period
	timingByPeriod := make(map[string][]sleepTimingRow)
	for timingRows.Next() {
		var t sleepTimingRow
		if err := timingRows.Scan(&t.period, &t.sleepStart, &t.sleepEnd); err != nil {
			return nil, fmt.Errorf("scanning sleep timing: %w", err)
		}
		key := t.period.Format("2006-01-02")
		timingByPeriod[key] = append(timingByPeriod[key], t)
	}
	if err := timingRows.Err(); err != nil {
		return nil, err
	}

	// Compute circular mean bedtime/waketime per period
	for key, timings := range timingByPeriod {
		sp, ok := periodMap[key]
		if !ok {
			continue
		}

		bedtimeHours := make([]float64, 0, len(timings))
		waketimeHours := make([]float64, 0, len(timings))
		for _, t := range timings {
			bedtimeHours = append(bedtimeHours, timeToHourOfDay(t.sleepStart))
			waketimeHours = append(waketimeHours, timeToHourOfDay(t.sleepEnd))
		}

		avgBed, stdBed := circularMeanStd(bedtimeHours)
		avgWake, stdWake := circularMeanStd(waketimeHours)

		sp.AvgBedtime = hoursToHHMM(avgBed)
		sp.AvgWaketime = hoursToHHMM(avgWake)
		sp.BedtimeConsistencyStdHr = math.Round(stdBed*100) / 100
		sp.WaketimeConsistencyStdHr = math.Round(stdWake*100) / 100
	}

	// Assemble result
	result := make([]SleepSummaryPeriod, 0, len(periodOrder))
	for _, key := range periodOrder {
		result = append(result, *periodMap[key])
	}
	return result, nil
}

// timeToHourOfDay extracts fractional hour of day from a time.Time.
func timeToHourOfDay(t time.Time) float64 {
	return float64(t.Hour()) + float64(t.Minute())/60.0 + float64(t.Second())/3600.0
}

// circularMeanStd computes the circular mean and standard deviation for times
// expressed as hours (0–24). This handles the midnight wrap correctly
// (e.g., 23:00 and 01:00 average to 00:00, not 12:00).
func circularMeanStd(hours []float64) (mean, std float64) {
	if len(hours) == 0 {
		return 0, 0
	}

	var sinSum, cosSum float64
	for _, h := range hours {
		rad := h / 24.0 * 2 * math.Pi
		sinSum += math.Sin(rad)
		cosSum += math.Cos(rad)
	}

	n := float64(len(hours))
	sinAvg := sinSum / n
	cosAvg := cosSum / n

	// Circular mean
	meanRad := math.Atan2(sinAvg, cosAvg)
	if meanRad < 0 {
		meanRad += 2 * math.Pi
	}
	mean = meanRad / (2 * math.Pi) * 24.0

	// Circular standard deviation
	r := math.Sqrt(sinAvg*sinAvg + cosAvg*cosAvg)
	if r > 1 {
		r = 1
	}
	// Circular variance = 1 - R, std = sqrt(-2 * ln(R)) converted to hours
	if r > 0 {
		std = math.Sqrt(-2*math.Log(r)) / (2 * math.Pi) * 24.0
	}

	return mean, std
}

// hoursToHHMM formats fractional hours (0–24) as "HH:MM".
func hoursToHHMM(h float64) string {
	h = math.Mod(h, 24)
	if h < 0 {
		h += 24
	}
	hours := int(h)
	minutes := int(math.Round((h - float64(hours)) * 60))
	if minutes == 60 {
		hours++
		minutes = 0
	}
	if hours >= 24 {
		hours -= 24
	}
	return fmt.Sprintf("%02d:%02d", hours, minutes)
}
