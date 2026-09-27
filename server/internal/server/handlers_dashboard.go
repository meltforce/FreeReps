package server

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/claude/freereps/internal/models"
	"github.com/claude/freereps/internal/storage"
)

// defaultFrontPageWindowDays is the span shown when the request names no range.
const defaultFrontPageWindowDays = 30

// frontPageRanges maps the range control's options to a number of days. Trends
// reads the same endpoint, which is why 6m is here but absent from the
// dashboard's own control.
var frontPageRanges = map[string]int{
	"1d": 1, "7d": 7, "30d": 30, "90d": 90, "6m": 182, "1y": 365,
}

// frontPageMetric is one row of the front page: the metric's metadata, its
// latest reading, and everything derived from the window in one object.
type frontPageMetric struct {
	MetricName   string    `json:"metric_name"`
	Label        string    `json:"label"`
	Category     string    `json:"category"`
	Unit         string    `json:"unit"`
	IsCumulative bool      `json:"is_cumulative"`
	Multiplier   float64   `json:"multiplier"`
	Source       string    `json:"source"`
	Time         time.Time `json:"time"`

	Latest     *float64   `json:"latest"`
	Delta7d    *float64   `json:"delta_7d"`
	Delta7dPct *float64   `json:"delta_7d_pct"`
	RangeLow   *float64   `json:"range_low"`
	RangeHigh  *float64   `json:"range_high"`
	Series     []*float64 `json:"series"`
}

type frontPageResponse struct {
	Metrics []frontPageMetric `json:"metrics"`
	// The four metrics shown as hero numbers, in order.
	Heroes []string `json:"heroes"`
	// How many metrics the user has data for, visible or not. The footer states
	// "12 of 34 metrics shown" from this.
	TotalAvailable int       `json:"total_available"`
	WindowDays     int       `json:"window_days"`
	WindowStart    time.Time `json:"window_start"`
	// When the most recent ingest ran, and which sources it covered.
	LastSync    *time.Time `json:"last_sync"`
	LastSources []string   `json:"last_sources"`
}

// handleLatestMetrics returns the whole front page in one response: per visible
// metric the latest value, unit, source, timestamp, 7-day delta, percentile
// range and a daily series for the sparkline.
//
// The series arrays are what let the dashboard drop both its other requests —
// carrying ~30 floats per metric costs less than a second round trip, and the
// payload already names which metrics exist.
func (s *Server) handleLatestMetrics(w http.ResponseWriter, r *http.Request) {
	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}
	ctx := r.Context()

	available, err := s.db.GetAvailableMetrics(ctx, uid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	visible := make([]storage.AllowedMetric, 0, len(available))
	names := make([]string, 0, len(available))
	for _, m := range available {
		if m.Visible {
			visible = append(visible, m)
			names = append(names, m.MetricName)
		}
	}

	windowDays := defaultFrontPageWindowDays
	if d, ok := frontPageRanges[r.URL.Query().Get("range")]; ok {
		windowDays = d
	}
	// The delta compares two 7-day windows, so a short selection still queries
	// enough history to compute one.
	bufferDays := windowDays
	if bufferDays < storage.DeltaWindowDays {
		bufferDays = storage.DeltaWindowDays
	}

	end := time.Now().Truncate(24*time.Hour).AddDate(0, 0, 1)
	start := end.AddDate(0, 0, -bufferDays)
	windowStart := end.AddDate(0, 0, -windowDays)

	var (
		latest    []models.HealthMetricRow
		series    map[string][]storage.DailyPoint
		heroes    []string
		logs      []storage.ImportLog
		errLatest error
		errSeries error
		errHeroes error
		wg        sync.WaitGroup
	)

	wg.Add(4)
	go func() {
		defer wg.Done()
		// Naming the visible metrics turns a walk across every index entry
		// into one bounded lookup each.
		latest, errLatest = s.db.GetLatestMetricsFor(ctx, uid, names)
	}()
	go func() {
		defer wg.Done()
		series, errSeries = s.db.GetDailySeries(ctx, uid, names, start, end)
	}()
	go func() {
		defer wg.Done()
		heroes, errHeroes = s.db.GetFrontPageHeroes(ctx, uid)
	}()
	go func() {
		defer wg.Done()
		// A failed log query costs the sync line, not the page.
		logs, _ = s.db.QueryImportLogs(ctx, uid, 5)
	}()
	wg.Wait()

	for _, err := range []error{errLatest, errSeries, errHeroes} {
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	latestByName := make(map[string]models.HealthMetricRow, len(latest))
	for _, row := range latest {
		latestByName[row.MetricName] = row
	}

	metrics := make([]frontPageMetric, 0, len(visible))
	for _, meta := range visible {
		points := series[meta.MetricName]
		s, delta, deltaPct, low, high := storage.BuildDashboardMetric(points, start, bufferDays, windowDays)

		fp := frontPageMetric{
			MetricName:   meta.MetricName,
			Label:        meta.DisplayLabel,
			Category:     meta.Category,
			Unit:         meta.DisplayUnit,
			IsCumulative: meta.IsCumulative,
			Multiplier:   meta.DisplayMultiplier,
			Delta7d:      delta,
			Delta7dPct:   deltaPct,
			RangeLow:     low,
			RangeHigh:    high,
			Series:       s,
		}
		if row, ok := latestByName[meta.MetricName]; ok {
			fp.Source = row.Source
			fp.Time = row.Time
			fp.Latest = latestValue(row, meta, points)
		}
		metrics = append(metrics, fp)
	}

	resp := frontPageResponse{
		Metrics:        metrics,
		Heroes:         heroes,
		TotalAvailable: len(available),
		WindowDays:     windowDays,
		WindowStart:    windowStart,
	}
	if len(logs) > 0 {
		t := logs[0].CreatedAt
		resp.LastSync = &t
		seen := map[string]bool{}
		for _, l := range logs {
			if l.Source != "" && !seen[l.Source] {
				seen[l.Source] = true
				resp.LastSources = append(resp.LastSources, l.Source)
			}
		}
	}

	w.Header().Set("Cache-Control", "private, max-age=60")
	writeJSON(w, http.StatusOK, resp)
}

// handleWorkoutZones returns the per-zone share of each workout in the range,
// plus the heart rates and bpm edges the zone bands are derived from. Kept separate
// from the workout list so that response shape stays as the iOS app expects it.
func (s *Server) handleWorkoutZones(w http.ResponseWriter, r *http.Request) {
	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}
	start, end, err := parseTimeRange(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	maxHR, err := s.db.GetMaxHeartRate(r.Context(), uid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	zones, err := s.db.GetWorkoutZones(r.Context(), uid, start, end, maxHR.ZoneEdges)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Cache-Control", "private, max-age=60")
	writeJSON(w, http.StatusOK, map[string]any{
		"max_heart_rate":          maxHR.BPM,
		"max_heart_rate_origin":   maxHR.Origin,
		"observed_max_heart_rate": maxHR.Observed,
		"resting_heart_rate":      maxHR.Resting,
		"zone_edges":              maxHR.ZoneEdges,
		"zones":                   zones,
	})
}

// handleMaxHeartRate reports the figure the zones derive from and where it came
// from, so the settings screen can offer the measured value as a starting point.
func (s *Server) handleMaxHeartRate(w http.ResponseWriter, r *http.Request) {
	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}
	maxHR, err := s.db.GetMaxHeartRate(r.Context(), uid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, maxHR)
}

// handleSaveMaxHeartRate stores the user's own maximum. A zero clears it and
// returns the zones to the measured figure.
func (s *Server) handleSaveMaxHeartRate(w http.ResponseWriter, r *http.Request) {
	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}

	var body struct {
		BPM float64 `json:"bpm"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	// Outside this range the value is a typo, not a heart rate, and it would
	// silently distort every zone bar in the app.
	if body.BPM != 0 && (body.BPM < 100 || body.BPM > 250) {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "a maximum heart rate is between 100 and 250 bpm; send 0 to clear it",
		})
		return
	}

	if err := s.db.SetPreference(r.Context(), uid, storage.PrefMaxHeartRate, body.BPM); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

// handleSaveBirthDate stores the date of birth the max heart rate estimate
// derives from. An empty string clears it.
func (s *Server) handleSaveBirthDate(w http.ResponseWriter, r *http.Request) {
	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}

	var body struct {
		BirthDate string `json:"birth_date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if body.BirthDate != "" {
		d, err := time.Parse("2006-01-02", body.BirthDate)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "expected a date as YYYY-MM-DD",
			})
			return
		}
		age := storage.AgeYears(d, time.Now())
		// Outside this range the entry is a typo, and it would feed a nonsense
		// maximum into every zone bar.
		if age < 10 || age > 110 {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "that date gives an implausible age; send an empty string to clear it",
			})
			return
		}
	}

	if err := s.db.SetPreference(r.Context(), uid, storage.PrefBirthDate, body.BirthDate); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

// handleBirthDate reports the stored date of birth, empty when unset.
func (s *Server) handleBirthDate(w http.ResponseWriter, r *http.Request) {
	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}
	birth, found, err := s.db.GetBirthDate(r.Context(), uid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	resp := map[string]any{"birth_date": "", "age": 0}
	if found {
		resp["birth_date"] = birth.Format("2006-01-02")
		resp["age"] = storage.AgeYears(birth, time.Now())
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleSaveFrontPageHeroes stores the four metrics the dashboard shows as hero
// numbers.
func (s *Server) handleSaveFrontPageHeroes(w http.ResponseWriter, r *http.Request) {
	uid, ok := mustUserID(w, r)
	if !ok {
		return
	}

	var heroes []string
	if err := json.NewDecoder(r.Body).Decode(&heroes); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if len(heroes) != 4 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "exactly four metrics required"})
		return
	}

	if err := s.db.SetPreference(r.Context(), uid, storage.PrefFrontPageHeroes, heroes); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

// latestValue picks what the row should show. A cumulative metric's latest
// sample is one increment, so the current day's total is the meaningful figure;
// everything else reports its own last reading.
func latestValue(row models.HealthMetricRow, meta storage.AllowedMetric, points []storage.DailyPoint) *float64 {
	if meta.IsCumulative && len(points) > 0 {
		v := points[len(points)-1].Value
		return &v
	}
	if row.AvgVal != nil {
		return row.AvgVal
	}
	return row.Qty
}
