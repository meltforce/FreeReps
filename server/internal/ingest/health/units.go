package health

import "github.com/claude/freereps/internal/models"

// canonicalUnit names the unit a metric is stored in, for the metrics where
// Health Auto Export has been observed to deliver a second one.
//
// The delivered unit follows the aggregation setting of the export, not the
// metric: an aggregated export writes `active_energy` in kcal and
// `time_in_daylight` in min, an export of individual samples writes the
// HealthKit units kJ and s for the same two. On 2026-09-22 both forms stood in
// the table under one metric name, and a query summing them produced a number
// without a unit, because the aggregation in
// `internal/storage/health_metrics.go` takes `MAX(units)` and adds `qty` up
// regardless.
//
// This is deliberately not `metric_allowlist.display_unit`. That column names
// the unit a value is *shown* in — `km` for `distance_walking_running`, whose
// rows hold metres — so converting towards it would rescale correct data.
var canonicalUnit = map[string]string{
	"active_energy":              "kcal",
	"basal_energy_burned":        "kcal",
	"time_in_daylight":           "min",
	"walking_heart_rate_average": "bpm",
}

// unitFactor gives the factor from a delivered unit to the canonical one. A
// pair that is absent leaves the row untouched, so an unknown unit is stored as
// delivered rather than rescaled by a guess.
var unitFactor = map[[2]string]float64{
	{"kJ", "kcal"}:       1 / 4.184,
	{"kcal", "kJ"}:       4.184,
	{"s", "min"}:         1.0 / 60,
	{"min", "s"}:         60,
	{"count/min", "bpm"}: 1,
	{"bpm", "count/min"}: 1,
}

// normalizeUnits rewrites a row into the canonical unit of its metric and
// reports whether it did. A metric without a canonical unit, a row that already
// carries it, and a pair with no known factor are all left alone; the caller
// logs the last case, because it marks a unit the export has not sent before.
func normalizeUnits(row *models.HealthMetricRow) (converted, unknown bool) {
	want, ok := canonicalUnit[row.MetricName]
	if !ok || row.Units == want {
		return false, false
	}
	factor, ok := unitFactor[[2]string{row.Units, want}]
	if !ok {
		return false, true
	}
	for _, v := range []*float64{row.Qty, row.MinVal, row.AvgVal, row.MaxVal} {
		if v != nil {
			*v *= factor
		}
	}
	row.Units = want
	return true, false
}
