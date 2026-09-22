package server

import (
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/claude/freereps/internal/alerts"
	"github.com/claude/freereps/internal/storage"
)

// alertSettingsBody is the wire form of the settings. Durations are seconds
// rather than Go duration strings, so the form can use number inputs and no
// parsing happens in the browser.
type alertSettingsBody struct {
	Enabled          bool   `json:"enabled"`
	NtfyURL          string `json:"ntfy_url"`
	Hostname         string `json:"hostname"`
	CheckIntervalSec int    `json:"check_interval_sec"`
	FailureThreshold int    `json:"failure_threshold"`
	AppleSilenceSec  int    `json:"apple_silence_sec"`
}

// Bounds on the intervals. The lower limits are not arbitrary: a check interval
// under a minute polls `import_logs` for nothing, and a silence threshold under
// an hour reports an Apple Health path that is merely between two automation
// runs.
const (
	minCheckIntervalSec = 60
	minAppleSilenceSec  = 3600
)

// handleAlertSettings returns the stored channel configuration together with the
// state of every condition, so the Settings screen shows what is configured and
// what that configuration currently reports.
func (s *Server) handleAlertSettings(w http.ResponseWriter, r *http.Request) {
	if _, ok := mustUserID(w, r); !ok {
		return
	}

	st, known, err := s.db.GetAlertSettings(r.Context())
	if err != nil {
		s.log.Error("reading alert settings", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	states, err := s.db.AlertStates(r.Context())
	if err != nil {
		s.log.Warn("reading alert states", "error", err)
	}

	type condition struct {
		MonitorID int        `json:"monitor_id"`
		Service   string     `json:"service"`
		Firing    bool       `json:"firing"`
		Since     *time.Time `json:"since,omitempty"`
		LastMsg   string     `json:"last_msg,omitempty"`
		Checked   *time.Time `json:"checked_at,omitempty"`
	}

	byID := map[int]storage.AlertConditionState{}
	for _, c := range states {
		byID[c.MonitorID] = c
	}

	// Every condition the watcher knows is listed, including the ones that have
	// not been evaluated yet: a screen that hides them cannot show that the
	// watcher is not running.
	ids := []int{
		alerts.MonitorWithingsSync,
		alerts.MonitorOuraSync,
		alerts.MonitorHevySync,
		alerts.MonitorAppleIngest,
		alerts.MonitorAppleWorkouts,
	}
	conditions := make([]condition, 0, len(ids))
	for _, id := range ids {
		c := condition{MonitorID: id, Service: alerts.ServiceNames[id]}
		if stored, ok := byID[id]; ok {
			c.Firing = stored.Firing
			since := stored.Since
			checked := stored.UpdatedAt
			c.Since = &since
			c.Checked = &checked
			c.LastMsg = stored.LastMsg
		}
		conditions = append(conditions, c)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"configured": known,
		"settings": alertSettingsBody{
			Enabled:          st.Enabled,
			NtfyURL:          st.NtfyURL,
			Hostname:         st.Hostname,
			CheckIntervalSec: int(st.CheckInterval.Seconds()),
			FailureThreshold: st.FailureThreshold,
			AppleSilenceSec:  int(st.AppleSilence.Seconds()),
		},
		"updated_at": st.UpdatedAt,
		"conditions": conditions,
	})
}

// handleSaveAlertSettings replaces the channel configuration.
func (s *Server) handleSaveAlertSettings(w http.ResponseWriter, r *http.Request) {
	if _, ok := mustUserID(w, r); !ok {
		return
	}

	var body alertSettingsBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	if msg := validateAlertSettings(body); msg != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": msg})
		return
	}

	if body.Hostname == "" {
		body.Hostname = "freereps"
	}

	st := storage.AlertSettings{
		Enabled:          body.Enabled,
		NtfyURL:          body.NtfyURL,
		Hostname:         body.Hostname,
		CheckInterval:    time.Duration(body.CheckIntervalSec) * time.Second,
		FailureThreshold: body.FailureThreshold,
		AppleSilence:     time.Duration(body.AppleSilenceSec) * time.Second,
	}
	if err := s.db.SetAlertSettings(r.Context(), st); err != nil {
		s.log.Error("writing alert settings", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	s.log.Info("alert settings saved",
		"enabled", st.Enabled,
		"target", st.NtfyURL,
		"interval", st.CheckInterval,
		"failure_threshold", st.FailureThreshold,
		"apple_silence", st.AppleSilence)
	writeJSON(w, http.StatusOK, map[string]any{"saved": true})
}

// validateAlertSettings returns an empty string when the body is acceptable, and
// the reason otherwise. An enabled channel is validated strictly; a disabled one
// only has to be storable.
func validateAlertSettings(b alertSettingsBody) string {
	if b.FailureThreshold < 1 {
		return "failure_threshold must be at least 1"
	}
	if b.CheckIntervalSec < minCheckIntervalSec {
		return "check_interval_sec must be at least 60"
	}
	if b.AppleSilenceSec != 0 && b.AppleSilenceSec < minAppleSilenceSec {
		return "apple_silence_sec must be 0 (off) or at least 3600"
	}
	if !b.Enabled {
		return ""
	}
	if b.NtfyURL == "" {
		return "ntfy_url is required while alerts are enabled"
	}
	u, err := url.Parse(b.NtfyURL)
	if err != nil {
		return "ntfy_url is not a URL: " + err.Error()
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "ntfy_url must be an http or https URL"
	}
	if u.Host == "" {
		return "ntfy_url names no host"
	}
	if u.Path == "" || u.Path == "/" {
		return "ntfy_url must include the topic, e.g. https://ntfy.example.com/freereps-alerts"
	}
	return ""
}

// handleTestAlert posts one message on the channel-test id, so the path can be
// proven from the Settings screen rather than by waiting for a real failure.
//
// It sends whatever is stored, not the form's unsaved contents: a test that
// passes has then proven the configuration the watcher will use.
func (s *Server) handleTestAlert(w http.ResponseWriter, r *http.Request) {
	if _, ok := mustUserID(w, r); !ok {
		return
	}
	if s.alerts == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "the alert watcher is not running"})
		return
	}

	st, known, err := s.db.GetAlertSettings(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if !known || st.NtfyURL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no ntfy url stored; save the settings first"})
		return
	}

	info := userInfoFromContext(r)
	msg := "channel test from the FreeReps settings screen, sent by " + info.Login
	if err := s.alerts.SendTest(r.Context(), st, msg); err != nil {
		s.log.Warn("alert channel test failed", "error", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"sent":       true,
		"monitor_id": alerts.MonitorChannelTest,
		"target":     st.NtfyURL,
	})
}
