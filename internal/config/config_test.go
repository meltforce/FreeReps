package config

import (
	"os"
	"path/filepath"
	"testing"
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
auth:
  api_key: "test-key-123"
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
	if cfg.Auth.APIKey != "test-key-123" {
		t.Errorf("auth.api_key = %q, want %q", cfg.Auth.APIKey, "test-key-123")
	}
}

// TestEnvOverride verifies that FREEREPS_ env vars take precedence over YAML values.
// This ensures production deployments can override config via environment.
func TestEnvOverride(t *testing.T) {
	t.Setenv("FREEREPS_DB_HOST", "override-host")
	t.Setenv("FREEREPS_DB_PORT", "9999")
	t.Setenv("FREEREPS_AUTH_API_KEY", "env-key")

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
	if cfg.Auth.APIKey != "env-key" {
		t.Errorf("auth.api_key = %q, want %q", cfg.Auth.APIKey, "env-key")
	}
	// Unchanged fields should keep YAML values
	if cfg.Database.Name != "freereps" {
		t.Errorf("database.name = %q, want %q", cfg.Database.Name, "freereps")
	}
}

// TestValidationMissingPort verifies that missing required fields produce a clear error.
// Prevents starting the server with incomplete configuration.
func TestValidationMissingPort(t *testing.T) {
	yaml := `
server:
  host: "0.0.0.0"
database:
  host: "localhost"
  port: 5432
  name: "freereps"
  user: "freereps"
auth:
  api_key: "key"
`
	_, err := Load(writeTemp(t, yaml))
	if err == nil {
		t.Fatal("expected validation error for missing port")
	}
}

// TestValidationMissingAPIKey verifies that a missing API key is rejected.
// Without an API key, the ingest endpoint would be unprotected.
func TestValidationMissingAPIKey(t *testing.T) {
	yaml := `
server:
  port: 8080
database:
  host: "localhost"
  port: 5432
  name: "freereps"
  user: "freereps"
auth: {}
`
	_, err := Load(writeTemp(t, yaml))
	if err == nil {
		t.Fatal("expected validation error for missing api_key")
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
