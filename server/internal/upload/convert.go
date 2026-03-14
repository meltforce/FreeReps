package upload

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/claude/freereps/internal/models"
)

// hrDataPoint holds a heart rate measurement for in-memory HR correlation.
type hrDataPoint struct {
	Time   time.Time
	Min    float64
	Avg    float64
	Max    float64
	Units  string
	Source string
}

// convertMetric converts an HAEFileMetric to REST API HAEMetric format.
// Returns the converted metric and any heart rate data points extracted (for HR correlation).
func convertMetric(file models.HAEFileMetric, metricName string) (models.HAEMetric, []hrDataPoint, error) {
	isHeartRate := metricName == "heart_rate"
	isActiveEnergy := metricName == "active_energy"
	isSleep := metricName == "sleep_analysis"

	metric := models.HAEMetric{
		Name: metricName,
	}

	var hrPoints []hrDataPoint

	if isSleep {
		data, err := convertSleepStages(file.Data)
		if err != nil {
			return metric, nil, err
		}
		metric.Units = "hr"
		metric.Data = data
		return metric, nil, nil
	}

	var data []json.RawMessage
	for _, dp := range file.Data {
		// Active energy: filter to kcal only (skip kJ duplicates)
		if isActiveEnergy && dp.Unit != "kcal" {
			continue
		}

		t := models.AppleTimestampToTime(dp.Start)

		if isHeartRate {
			min := safeFloat(dp.Min)
			avg := safeFloat(dp.Avg)
			max := safeFloat(dp.Max)

			point := map[string]any{
				"date": formatHAETime(t),
				"Min":  min,
				"Avg":  avg,
				"Max":  max,
			}
			raw, err := json.Marshal(point)
			if err != nil {
				continue
			}
			data = append(data, raw)

			// Collect for HR correlation with workouts
			hrPoints = append(hrPoints, hrDataPoint{
				Time:   t,
				Min:    min,
				Avg:    avg,
				Max:    max,
				Units:  dp.Unit,
				Source: dp.SourceName(),
			})
		} else {
			point := map[string]any{
				"date": formatHAETime(t),
				"qty":  safeFloat(dp.Qty),
			}
			raw, err := json.Marshal(point)
			if err != nil {
				continue
			}
			data = append(data, raw)
		}

		if metric.Units == "" {
			metric.Units = dp.Unit
		}
	}

	metric.Data = data
	return metric, hrPoints, nil
}

// convertSleepStages converts .hae sleep data points to REST API unaggregated format.
func convertSleepStages(dataPoints []models.HAEFileDataPoint) ([]json.RawMessage, error) {
	var data []json.RawMessage
	for _, dp := range dataPoints {
		stageType := dp.SleepStageType()
		if stageType == "" {
			continue
		}

		stage := models.HAESleepStage{
			StartDate: models.HAETime{Time: models.AppleTimestampToTime(dp.Start)},
			EndDate:   models.HAETime{Time: models.AppleTimestampToTime(dp.End)},
			Qty:       dp.SleepStageDuration(),
			Value:     stageType,
		}

		raw, err := json.Marshal(stage)
		if err != nil {
			continue
		}
		data = append(data, raw)
	}
	return data, nil
}

// convertWorkout converts an HAEFileWorkout to REST API HAEWorkout format.
// Route data is embedded from a separate route file (if found).
// Heart rate data is correlated from in-memory hrPoints collected during metric processing.
func convertWorkout(file models.HAEFileWorkout, route *models.HAEFileRoute, hrPoints []hrDataPoint) models.HAEWorkout {
	start := models.AppleTimestampToTime(file.Start)
	end := models.AppleTimestampToTime(file.End)

	w := models.HAEWorkout{
		ID:       file.ID,
		Name:     file.Name,
		Start:    models.HAETime{Time: start},
		End:      models.HAETime{Time: end},
		Duration: file.Duration,
		Location: file.Location,
	}

	if file.ActiveEnergy != nil {
		w.ActiveEnergyBurned = &models.HAEQuantity{Qty: *file.ActiveEnergy, Units: "kcal"}
	}
	if file.TotalDistance != nil {
		w.Distance = &models.HAEQuantity{Qty: *file.TotalDistance, Units: "km"}
	}
	if file.ElevationUp != nil {
		w.ElevationUp = &models.HAEQuantity{Qty: *file.ElevationUp, Units: "m"}
	}

	// Embed route data from separate .hae file
	if route != nil && len(route.Locations) > 0 {
		routePoints := make([]models.HAERoutePoint, len(route.Locations))
		for i, loc := range route.Locations {
			routePoints[i] = models.HAERoutePoint{
				Latitude:           loc.Latitude,
				Longitude:          loc.Longitude,
				Altitude:           loc.Elevation,
				Speed:              loc.Speed,
				Course:             loc.Course,
				HorizontalAccuracy: loc.HAcc,
				VerticalAccuracy:   loc.VAcc,
				Timestamp:          models.HAETime{Time: models.AppleTimestampToTime(loc.Time)},
			}
		}
		w.Route = routePoints
	}

	// Correlate HR data from overlapping heart_rate metrics
	if len(hrPoints) > 0 {
		correlatedHR := correlateWorkoutHR(hrPoints, start, end)
		if len(correlatedHR) > 0 {
			w.HeartRateData = correlatedHR

			// Compute HR summary from correlated data
			var minHR, maxHR, sumHR float64
			minHR = correlatedHR[0].Avg
			for _, hr := range correlatedHR {
				sumHR += hr.Avg
				if hr.Avg < minHR {
					minHR = hr.Avg
				}
				if hr.Avg > maxHR {
					maxHR = hr.Avg
				}
			}
			avgHR := sumHR / float64(len(correlatedHR))
			w.HeartRate = &models.HAEHeartRateSummary{
				Min: models.HAEQuantity{Qty: minHR, Units: "bpm"},
				Avg: models.HAEQuantity{Qty: avgHR, Units: "bpm"},
				Max: models.HAEQuantity{Qty: maxHR, Units: "bpm"},
			}
		}
	}

	return w
}

// correlateWorkoutHR finds heart rate data points within the workout's time range
// using binary search on the sorted hrPoints slice.
func correlateWorkoutHR(hrPoints []hrDataPoint, start, end time.Time) []models.HAEWorkoutHRPoint {
	// Binary search for the first point >= start
	lo := sort.Search(len(hrPoints), func(i int) bool {
		return !hrPoints[i].Time.Before(start)
	})

	var result []models.HAEWorkoutHRPoint
	for i := lo; i < len(hrPoints); i++ {
		if hrPoints[i].Time.After(end) {
			break
		}
		result = append(result, models.HAEWorkoutHRPoint{
			Date:   models.HAETime{Time: hrPoints[i].Time},
			Min:    hrPoints[i].Min,
			Avg:    hrPoints[i].Avg,
			Max:    hrPoints[i].Max,
			Units:  hrPoints[i].Units,
			Source: hrPoints[i].Source,
		})
	}
	return result
}

// formatHAETime formats a time.Time as an HAE time string.
func formatHAETime(t time.Time) string {
	return t.Format(models.HAETimeLayout)
}

// safeFloat dereferences a float pointer, returning 0 if nil.
func safeFloat(f *float64) float64 {
	if f == nil {
		return 0
	}
	return *f
}
