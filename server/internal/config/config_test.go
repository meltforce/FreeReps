package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const validYAML = `
server:
  host: "0.0.0.0"
  port: 8080
database:
  host: "localhost"
  port: 5432
  name: "freereps"
  user: "freereps"
  password: "secret"
  sslmode: "disable"
tailscale:
  enabled: false
`

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestLoadValid verifies that a well-formed YAML config loads with all fields populated.
func TestLoadValid(t *testing.T) {
	cfg, err := Load(writeTemp(t, validYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("server.host = %q, want %q", cfg.Server.Host, "0.0.0.0")
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("server.port = %d, want 8080", cfg.Server.Port)
	}
	if cfg.Database.Host != "localhost" {
		t.Errorf("database.host = %q, want %q", cfg.Database.Host, "localhost")
	}
	if cfg.Database.Port != 5432 {
		t.Errorf("database.port = %d, want 5432", cfg.Database.Port)
	}
	if cfg.Database.Name != "freereps" {
		t.Errorf("database.name = %q, want %q", cfg.Database.Name, "freereps")
	}
	if cfg.Tailscale.Enabled {
		t.Errorf("tailscale.enabled = true, want false")
	}
}

// TestEnvOverride verifies that FREEREPS_ env vars take precedence over YAML values.
// This ensures production deployments can override config via environment.
func TestEnvOverride(t *testing.T) {
	t.Setenv("FREEREPS_DB_HOST", "override-host")
	t.Setenv("FREEREPS_DB_PORT", "9999")
	t.Setenv("FREEREPS_TS_HOSTNAME", "custom-host")

	cfg, err := Load(writeTemp(t, validYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Database.Host != "override-host" {
		t.Errorf("database.host = %q, want %q", cfg.Database.Host, "override-host")
	}
	if cfg.Database.Port != 9999 {
		t.Errorf("database.port = %d, want 9999", cfg.Database.Port)
	}
	if cfg.Tailscale.Hostname != "custom-host" {
		t.Errorf("tailscale.hostname = %q, want %q", cfg.Tailscale.Hostname, "custom-host")
	}
	// Unchanged fields should keep YAML values
	if cfg.Database.Name != "freereps" {
		t.Errorf("database.name = %q, want %q", cfg.Database.Name, "freereps")
	}
}

// TestTailscaleDefaults verifies that Tailscale config gets sensible defaults
// even when not specified in the YAML file.
func TestTailscaleDefaults(t *testing.T) {
	yaml := `
server:
  port: 8080
database:
  host: "localhost"
  port: 5432
  name: "freereps"
  user: "freereps"
`
	cfg, err := Load(writeTemp(t, yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Tailscale.Enabled {
		t.Error("tailscale.enabled should default to true")
	}
	if cfg.Tailscale.Hostname != "freereps" {
		t.Errorf("tailscale.hostname = %q, want %q", cfg.Tailscale.Hostname, "freereps")
	}
	if cfg.Tailscale.StateDir != "tsnet-state" {
		t.Errorf("tailscale.state_dir = %q, want %q", cfg.Tailscale.StateDir, "tsnet-state")
	}
}

// TestValidationMissingPort verifies that missing port fails when tailscale is disabled.
// When tsnet is disabled, server.port is required for plain HTTP.
func TestValidationMissingPort(t *testing.T) {
	yaml := `
server:
  host: "0.0.0.0"
database:
  host: "localhost"
  port: 5432
  name: "freereps"
  user: "freereps"
tailscale:
  enabled: false
`
	_, err := Load(writeTemp(t, yaml))
	if err == nil {
		t.Fatal("expected validation error for missing port when tailscale disabled")
	}
}

// TestValidationPortNotRequiredWithTsnet verifies that server.port is optional
// when Tailscale is enabled (tsnet listens on port 80 via tailnet).
func TestValidationPortNotRequiredWithTsnet(t *testing.T) {
	yaml := `
database:
  host: "localhost"
  port: 5432
  name: "freereps"
  user: "freereps"
tailscale:
  enabled: true
`
	cfg, err := Load(writeTemp(t, yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Tailscale.Enabled {
		t.Error("tailscale.enabled should be true")
	}
}

// TestTsEnabledEnvOverride verifies that FREEREPS_TS_ENABLED env var correctly
// overrides the YAML tailscale.enabled value.
func TestTsEnabledEnvOverride(t *testing.T) {
	t.Setenv("FREEREPS_TS_ENABLED", "false")

	yaml := `
server:
  port: 8080
database:
  host: "localhost"
  port: 5432
  name: "freereps"
  user: "freereps"
tailscale:
  enabled: true
`
	cfg, err := Load(writeTemp(t, yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Tailscale.Enabled {
		t.Error("tailscale.enabled should be false after env override")
	}
}

// TestDSN verifies the PostgreSQL connection string is built correctly.
func TestDSN(t *testing.T) {
	d := DatabaseConfig{
		Host:     "db.example.com",
		Port:     5432,
		Name:     "mydb",
		User:     "admin",
		Password: "pass",
		SSLMode:  "require",
	}
	want := "postgres://admin:pass@db.example.com:5432/mydb?sslmode=require"
	if got := d.DSN(); got != want {
		t.Errorf("DSN() = %q, want %q", got, want)
	}
}

// TestDSNDefaultSSLMode verifies that an empty sslmode defaults to "disable".
func TestDSNDefaultSSLMode(t *testing.T) {
	d := DatabaseConfig{
		Host: "localhost", Port: 5432, Name: "db", User: "u", Password: "p",
	}
	got := d.DSN()
	if want := "postgres://u:p@localhost:5432/db?sslmode=disable"; got != want {
		t.Errorf("DSN() = %q, want %q", got, want)
	}
}

// TestLoadMissingFile verifies that a missing config file returns a clear error.
func TestLoadMissingFile(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

// TestOuraDefaults verifies that Oura config gets sensible defaults when not
// specified in YAML (sync_interval and backfill_days are server-wide settings).
func TestOuraDefaults(t *testing.T) {
	cfg, err := Load(writeTemp(t, validYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Oura.SyncInterval != 30*60*1e9 { // 30 minutes
		t.Errorf("oura.sync_interval = %v, want 30m", cfg.Oura.SyncInterval)
	}
	if cfg.Oura.BackfillDays != 90 {
		t.Errorf("oura.backfill_days = %d, want 90", cfg.Oura.BackfillDays)
	}
}

// TestSourcePriorityDefault verifies the default source priority list ensures
// the direct provider reads win over the same measurements relayed through
// HealthKit, which arrive only when the Health app has synced.
func TestSourcePriorityDefault(t *testing.T) {
	cfg, err := Load(writeTemp(t, validYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"Withings", "Oura", ""}
	if len(cfg.SourcePriority) != len(want) {
		t.Fatalf("source_priority = %v, want %v", cfg.SourcePriority, want)
	}
	for i, w := range want {
		if cfg.SourcePriority[i] != w {
			t.Errorf("source_priority[%d] = %q, want %q", i, cfg.SourcePriority[i], w)
		}
	}
}

// TestWithingsDefaults verifies the Withings sync settings fall back to the same
// server-wide defaults as Oura when the YAML omits the block entirely — an
// existing config file must not leave the syncer with a zero ticker interval.
func TestWithingsDefaults(t *testing.T) {
	cfg, err := Load(writeTemp(t, validYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Withings.SyncInterval != 30*time.Minute {
		t.Errorf("withings.sync_interval = %v, want 30m", cfg.Withings.SyncInterval)
	}
	if cfg.Withings.BackfillDays != 90 {
		t.Errorf("withings.backfill_days = %d, want 90", cfg.Withings.BackfillDays)
	}
}

// TestOuraCustomSyncInterval verifies that a custom sync_interval is parsed correctly.
func TestOuraCustomSyncInterval(t *testing.T) {
	yaml := `
server:
  port: 8080
database:
  host: "localhost"
  port: 5432
  name: "freereps"
  user: "freereps"
tailscale:
  enabled: false
oura:
  sync_interval: "15m"
  backfill_days: 30
`
	cfg, err := Load(writeTemp(t, yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Oura.SyncInterval != 15*60*1e9 {
		t.Errorf("oura.sync_interval = %v, want 15m", cfg.Oura.SyncInterval)
	}
	if cfg.Oura.BackfillDays != 30 {
		t.Errorf("oura.backfill_days = %d, want 30", cfg.Oura.BackfillDays)
	}
}

// TestIngestTimezoneDefault verifies that a config without an ingest block
// resolves to Europe/Berlin. The default is load-bearing: the stored Alpha
// history was written in that zone, and session_date is part of a workout_sets
// row's natural key, so a different resolved zone would insert every
// re-imported session a second time (INCIDENTS.md, 2026-08-10).
func TestIngestTimezoneDefault(t *testing.T) {
	cfg, err := Load(writeTemp(t, validYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Ingest.SessionTimezone != "Europe/Berlin" {
		t.Errorf("ingest.session_timezone = %q, want Europe/Berlin", cfg.Ingest.SessionTimezone)
	}
	if cfg.Ingest.Location == nil {
		t.Fatal("ingest location is nil")
	}
	// Resolved against a date in CET, so the offset is unambiguous.
	winter := time.Date(2026, 1, 2, 9, 22, 0, 0, cfg.Ingest.Location)
	if got := winter.UTC().Format(time.RFC3339); got != "2026-01-02T08:22:00Z" {
		t.Errorf("9:22 local = %s, want 2026-01-02T08:22:00Z", got)
	}
}

// TestIngestTimezoneRejectsEnvironmentDependentValues verifies that the two
// spellings time.LoadLocation accepts as "whatever this host happens to be"
// fail startup rather than resolving. Both would restore the behaviour that
// caused the duplicate import: "" resolves to UTC, "Local" to the process zone.
func TestIngestTimezoneRejectsEnvironmentDependentValues(t *testing.T) {
	for _, tz := range []string{`""`, `"Local"`} {
		if _, err := Load(writeTemp(t, validYAML+"ingest:\n  session_timezone: "+tz+"\n")); err == nil {
			t.Errorf("session_timezone %s loaded without error, want a failure", tz)
		}
	}
}

// TestIngestTimezoneUnknownZoneFails verifies that a typo fails startup instead
// of silently leaving the location at UTC.
func TestIngestTimezoneUnknownZoneFails(t *testing.T) {
	_, err := Load(writeTemp(t, validYAML+"ingest:\n  session_timezone: \"Europe/Berlim\"\n"))
	if err == nil {
		t.Fatal("unknown zone loaded without error, want a failure")
	}
}

// TestIngestTimezoneEnvOverride verifies the FREEREPS_INGEST_TIMEZONE override,
// which is how a deployment that renders config.yaml from elsewhere sets the
// zone without touching the file.
func TestIngestTimezoneEnvOverride(t *testing.T) {
	t.Setenv("FREEREPS_INGEST_TIMEZONE", "America/New_York")
	cfg, err := Load(writeTemp(t, validYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Ingest.SessionTimezone != "America/New_York" {
		t.Errorf("ingest.session_timezone = %q, want America/New_York", cfg.Ingest.SessionTimezone)
	}
	if cfg.Ingest.Location.String() != "America/New_York" {
		t.Errorf("resolved location = %q, want America/New_York", cfg.Ingest.Location)
	}
}

// TestAuthBlock: the auth block is optional, and its defaults decide who gets
// an account. A typo in accounts.policy must fail startup instead of opening
// or closing registration silently.
func TestAuthBlock(t *testing.T) {
	const base = "database:\n  host: localhost\n  port: 5432\n  name: freereps\n  user: freereps\n  password: x\ningest:\n  session_timezone: Europe/Berlin\n"
	write := func(t *testing.T, body string) string {
		t.Helper()
		p := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(p, []byte(base+body), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}

	cfg, err := Load(write(t, ""))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Auth != nil {
		t.Fatal("auth must be off without an auth: block")
	}

	t.Setenv("FREEREPS_OIDC_CLIENT_SECRET", "s3cret")
	cfg, err = Load(write(t, "auth:\n  base_url: https://freereps.example.net\n  methods: [oidc]\n  oidc:\n    issuer: https://idp.example.net\n    client_id: abc\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Accounts.Policy != PolicyClosed || cfg.Auth.TokenPrefix != "freereps" || cfg.Auth.OIDC.ClientSecret != "s3cret" {
		t.Fatalf("defaults: policy %q prefix %q secret %q", cfg.Accounts.Policy, cfg.Auth.TokenPrefix, cfg.Auth.OIDC.ClientSecret)
	}

	if _, err := Load(write(t, "auth:\n  base_url: https://x.example.net\n  methods: [password]\naccounts:\n  policy: open\n")); err == nil {
		t.Fatal("unknown accounts.policy accepted")
	}
	if _, err := Load(write(t, "auth:\n  methods: [password]\n")); err == nil {
		t.Fatal("auth without base_url accepted")
	}

	a := AccountsConfig{AllowedEmailDomains: []string{"@Example.net"}}
	if !a.AllowsEmail("a@example.NET") || a.AllowsEmail("a@other.net") || a.AllowsEmail("nodomain") {
		t.Fatal("AllowsEmail")
	}
}
