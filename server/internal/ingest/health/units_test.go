package health

import (
	"math"
	"testing"

	"github.com/claude/freereps/internal/models"
)

func f(v float64) *float64 { return &v }

func TestNormalizeUnits(t *testing.T) {
	tests := []struct {
		name      string
		row       models.HealthMetricRow
		wantQty   float64
		wantUnits string
		converted bool
		unknown   bool
	}{
		{
			name:      "kJ becomes kcal",
			row:       models.HealthMetricRow{MetricName: "active_energy", Units: "kJ", Qty: f(4.184)},
			wantQty:   1,
			wantUnits: "kcal",
			converted: true,
		},
		{
			name:      "basal energy uses the same factor",
			row:       models.HealthMetricRow{MetricName: "basal_energy_burned", Units: "kJ", Qty: f(41.84)},
			wantQty:   10,
			wantUnits: "kcal",
			converted: true,
		},
		{
			name:      "seconds become minutes",
			row:       models.HealthMetricRow{MetricName: "time_in_daylight", Units: "s", Qty: f(90)},
			wantQty:   1.5,
			wantUnits: "min",
			converted: true,
		},
		{
			name:      "count per minute is renamed, not rescaled",
			row:       models.HealthMetricRow{MetricName: "walking_heart_rate_average", Units: "count/min", Qty: f(94)},
			wantQty:   94,
			wantUnits: "bpm",
			converted: true,
		},
		{
			name:      "a row already canonical is left alone",
			row:       models.HealthMetricRow{MetricName: "active_energy", Units: "kcal", Qty: f(500)},
			wantQty:   500,
			wantUnits: "kcal",
		},
		{
			name:      "a metric without a canonical unit is left alone",
			row:       models.HealthMetricRow{MetricName: "distance_walking_running", Units: "m", Qty: f(8544.2)},
			wantQty:   8544.2,
			wantUnits: "m",
		},
		{
			name:      "an unknown unit is stored as delivered and reported",
			row:       models.HealthMetricRow{MetricName: "active_energy", Units: "J", Qty: f(4184)},
			wantQty:   4184,
			wantUnits: "J",
			unknown:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := tt.row
			converted, unknown := normalizeUnits(&row)
			if converted != tt.converted || unknown != tt.unknown {
				t.Fatalf("converted=%v unknown=%v, want converted=%v unknown=%v",
					converted, unknown, tt.converted, tt.unknown)
			}
			if row.Units != tt.wantUnits {
				t.Errorf("units = %q, want %q", row.Units, tt.wantUnits)
			}
			if math.Abs(*row.Qty-tt.wantQty) > 1e-9 {
				t.Errorf("qty = %v, want %v", *row.Qty, tt.wantQty)
			}
		})
	}
}

// Min/Avg/Max carry the same unit as Qty, so a shape that uses them has to be
// rescaled with it. heart_rate has no canonical unit today; the check is on the
// conversion itself rather than on a metric that happens to use that shape.
func TestNormalizeUnitsScalesMinAvgMax(t *testing.T) {
	row := models.HealthMetricRow{
		MetricName: "active_energy",
		Units:      "kJ",
		Qty:        f(4.184),
		MinVal:     f(8.368),
		AvgVal:     f(12.552),
		MaxVal:     f(16.736),
	}
	if converted, _ := normalizeUnits(&row); !converted {
		t.Fatal("expected a conversion")
	}
	for name, got := range map[string]float64{
		"qty": *row.Qty, "min": *row.MinVal, "avg": *row.AvgVal, "max": *row.MaxVal,
	} {
		want := map[string]float64{"qty": 1, "min": 2, "avg": 3, "max": 4}[name]
		if math.Abs(got-want) > 1e-9 {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}
}
