package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	// The zone database is compiled into the binary so that resolving
	// ingest.session_timezone does not depend on the host carrying
	// /usr/share/zoneinfo. Without it a runtime image built from scratch, or
	// an alpine image that drops the tzdata package, would fail startup on a
	// zone name that is perfectly valid.
	_ "time/tzdata"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server         ServerConfig    `yaml:"server"`
	Database       DatabaseConfig  `yaml:"database"`
	Tailscale      TailscaleConfig `yaml:"tailscale"`
	Ingest         IngestConfig    `yaml:"ingest"`
	Oura           OuraConfig      `yaml:"oura"`
	Hevy           HevyConfig      `yaml:"hevy"`
	OpenGym        OpenGymConfig   `yaml:"opengym"`
	Withings       WithingsConfig  `yaml:"withings"`
	Alerts         AlertsConfig    `yaml:"alerts"`
	SourcePriority []string        `yaml:"source_priority"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`

	// BaseURL overrides the origin the OAuth redirect URIs are built from, e.g.
	// https://freereps.example.ts.net. Leave it empty and the origin comes from
	// the request that starts the flow, which is the host the user reached the
	// UI on and therefore the host the provider has to redirect back to.
	//
	// Set it where the two differ: behind a reverse proxy that terminates TLS
	// under a different name, or where the UI is reachable under several names
	// while the provider accepts one registered URI.
	BaseURL string `yaml:"base_url"`
}

type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Name     string `yaml:"name"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	SSLMode  string `yaml:"sslmode"`
}

type TailscaleConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Hostname string `yaml:"hostname"`
	StateDir string `yaml:"state_dir"`
}

// IngestConfig holds settings for reading files whose timestamps carry no zone.
type IngestConfig struct {
	// SessionTimezone is the IANA zone the Alpha Progression CSV's wall-clock
	// session times are read in. It must be an explicit configuration value
	// rather than the process timezone: session_date is part of the unique key
	// workout_sets_source_natural_key (migration 000020), so reading the same
	// file in two zones inserts every session twice. That is the 2026-08-10
	// entry in INCIDENTS.md.
	SessionTimezone string `yaml:"session_timezone"`

	// Location is SessionTimezone resolved by Load. Nothing outside this
	// package sets it.
	Location *time.Location `yaml:"-"`
}

// OuraConfig holds server-wide Oura sync settings. Per-user credentials
// (client_id, client_secret) are stored in the database, not here.
type OuraConfig struct {
	SyncInterval time.Duration `yaml:"-"`
	BackfillDays int           `yaml:"backfill_days"`

	// RawSyncInterval is the YAML representation; parsed into SyncInterval by Load.
	RawSyncInterval string `yaml:"sync_interval"`
}

// HevyConfig holds server-wide Hevy sync settings. The per-user API key and the
// ingest cutoff are stored in the database, not here.
//
// There is no backfill window: the first sync fetches the full event history and
// the cutoff decides how much of it is kept.
type HevyConfig struct {
	SyncInterval time.Duration `yaml:"-"`

	// RawSyncInterval is the YAML representation; parsed into SyncInterval by Load.
	RawSyncInterval string `yaml:"sync_interval"`
}

// OpenGymConfig holds server-wide openGym sync settings. The per-user instance,
// token and ingest cutoff are stored in the database, not here.
type OpenGymConfig struct {
	SyncInterval time.Duration `yaml:"-"`

	// RawSyncInterval is the YAML representation; parsed into SyncInterval by Load.
	RawSyncInterval string `yaml:"sync_interval"`
}

// WithingsConfig holds server-wide Withings sync settings. Per-user credentials
// (client_id, client_secret) are stored in the database, not here.
type WithingsConfig struct {
	SyncInterval time.Duration `yaml:"-"`
	BackfillDays int           `yaml:"backfill_days"`

	// RawSyncInterval is the YAML representation; parsed into SyncInterval by Load.
	RawSyncInterval string `yaml:"sync_interval"`
}

// AlertsConfig seeds the alert channel on first start. The values then live in
// the database and are edited in the Settings UI (internal/alerts), so this block
// is read once per fresh database and ignored afterwards — a redeploy must not
// overwrite what the operator entered.
//
// Disabled by default: an instance whose operator has not named a topic would
// otherwise post into nothing on every check interval.
type AlertsConfig struct {
	Enabled bool `yaml:"enabled"`

	// NtfyURL is the full topic URL, e.g. https://ntfy.example.com/freereps-alerts
	NtfyURL string `yaml:"ntfy_url"`

	// Format is the wire format: "ntfy" for a text message a person reads in
	// the ntfy app, "kuma-json" for Uptime Kuma's webhook object, which a
	// program parses (internal/notify).
	Format string `yaml:"format"`

	// Hostname is the `hostname` field of every payload, so a consumer can name
	// the instance an alert came from — a test instance beside a production one.
	Hostname string `yaml:"hostname"`

	CheckInterval    time.Duration `yaml:"-"`
	AppleSilence     time.Duration `yaml:"-"`
	FailureThreshold int           `yaml:"failure_threshold"`

	// Raw forms are the YAML representation; parsed by Load.
	RawCheckInterval string `yaml:"check_interval"`
	RawAppleSilence  string `yaml:"apple_silence"`
}

// DSN returns a PostgreSQL connection string.
func (d DatabaseConfig) DSN() string {
	sslmode := d.SSLMode
	if sslmode == "" {
		sslmode = "disable"
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		d.User, d.Password, d.Host, d.Port, d.Name, sslmode)
}

// Load reads config from a YAML file, then applies environment variable overrides.
// Env vars use the prefix FREEREPS_ and underscore-separated paths:
//
//	FREEREPS_SERVER_HOST, FREEREPS_SERVER_PORT, FREEREPS_SERVER_BASE_URL,
//	FREEREPS_DB_HOST, FREEREPS_DB_PORT, FREEREPS_DB_NAME,
//	FREEREPS_DB_USER, FREEREPS_DB_PASSWORD, FREEREPS_DB_SSLMODE,
//	FREEREPS_TS_ENABLED, FREEREPS_TS_HOSTNAME, FREEREPS_TS_STATE_DIR,
//	FREEREPS_INGEST_TIMEZONE,
//	FREEREPS_ALERTS_ENABLED, FREEREPS_ALERTS_NTFY_URL, FREEREPS_ALERTS_FORMAT
func Load(path string) (*Config, error) {
	cfg := &Config{
		Tailscale: TailscaleConfig{
			Enabled:  true,
			Hostname: "freereps",
			StateDir: "tsnet-state",
		},
		// The stored history was written in Europe/Berlin, so that is the
		// default an unset key resolves to; a different value moves every
		// future Alpha session relative to the sessions already stored.
		Ingest: IngestConfig{
			SessionTimezone: "Europe/Berlin",
		},
		Oura: OuraConfig{
			RawSyncInterval: "30m",
			BackfillDays:    90,
		},
		Hevy: HevyConfig{
			RawSyncInterval: "30m",
		},
		OpenGym: OpenGymConfig{
			RawSyncInterval: "30m",
		},
		Withings: WithingsConfig{
			RawSyncInterval: "30m",
			BackfillDays:    90,
		},
		// Thresholds decided on 2026-09-20, see DECISIONS.md: three failed runs
		// at a 30-minute sync interval keep transient DNS failures out of the
		// channel, and 36 hours of silence on the Apple Health path means two
		// missed automation days rather than one quiet evening.
		Alerts: AlertsConfig{
			Format:           "ntfy",
			Hostname:         "freereps",
			RawCheckInterval: "5m",
			RawAppleSilence:  "36h",
			FailureThreshold: 3,
		},
		// Withings first: the same weight and blood pressure values also reach
		// FreeReps through Apple Health, where they arrive only when the Health
		// app has synced. The direct read is the more timely of the two.
		SourcePriority: []string{"Withings", "Oura", ""},
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	applyEnvOverrides(cfg)

	// Resolve the ingest timezone. An unknown zone name fails startup instead
	// of falling back to UTC: a silent fallback writes session timestamps that
	// differ from every session already stored, which is how the duplicate
	// import in INCIDENTS.md 2026-08-10 happened in the first place.
	//
	// Two spellings are rejected for the same reason. time.LoadLocation reads
	// "" as UTC and "Local" as the process timezone, so either one turns a
	// blank or copy-pasted config key into the deployment-dependent behaviour
	// this setting exists to remove.
	switch cfg.Ingest.SessionTimezone {
	case "":
		return nil, fmt.Errorf("ingest.session_timezone must name an IANA zone, e.g. Europe/Berlin")
	case "Local":
		return nil, fmt.Errorf(`ingest.session_timezone must name an IANA zone, not "Local": ` +
			"the zone has to be the same on every host that imports")
	}
	loc, err := time.LoadLocation(cfg.Ingest.SessionTimezone)
	if err != nil {
		return nil, fmt.Errorf("loading ingest.session_timezone %q: %w", cfg.Ingest.SessionTimezone, err)
	}
	cfg.Ingest.Location = loc

	// Parse Oura sync interval.
	if cfg.Oura.RawSyncInterval != "" {
		d, err := time.ParseDuration(cfg.Oura.RawSyncInterval)
		if err != nil {
			return nil, fmt.Errorf("parsing oura.sync_interval: %w", err)
		}
		cfg.Oura.SyncInterval = d
	}

	// Parse Hevy sync interval.
	if cfg.Hevy.RawSyncInterval != "" {
		d, err := time.ParseDuration(cfg.Hevy.RawSyncInterval)
		if err != nil {
			return nil, fmt.Errorf("parsing hevy.sync_interval: %w", err)
		}
		cfg.Hevy.SyncInterval = d
	}

	if cfg.OpenGym.RawSyncInterval != "" {
		d, err := time.ParseDuration(cfg.OpenGym.RawSyncInterval)
		if err != nil {
			return nil, fmt.Errorf("parsing opengym.sync_interval: %w", err)
		}
		cfg.OpenGym.SyncInterval = d
	}

	// Parse Withings sync interval.
	if cfg.Withings.RawSyncInterval != "" {
		d, err := time.ParseDuration(cfg.Withings.RawSyncInterval)
		if err != nil {
			return nil, fmt.Errorf("parsing withings.sync_interval: %w", err)
		}
		cfg.Withings.SyncInterval = d
	}

	// Parse the alert durations.
	if cfg.Alerts.RawCheckInterval != "" {
		d, err := time.ParseDuration(cfg.Alerts.RawCheckInterval)
		if err != nil {
			return nil, fmt.Errorf("parsing alerts.check_interval: %w", err)
		}
		cfg.Alerts.CheckInterval = d
	}
	if cfg.Alerts.RawAppleSilence != "" {
		d, err := time.ParseDuration(cfg.Alerts.RawAppleSilence)
		if err != nil {
			return nil, fmt.Errorf("parsing alerts.apple_silence: %w", err)
		}
		cfg.Alerts.AppleSilence = d
	}

	// A base URL with a path would produce redirect URIs like
	// `https://host/app/oura/callback` where the router serves `/oura/callback`,
	// and the provider would reject the mismatch at the end of the flow rather
	// than at startup.
	if cfg.Server.BaseURL != "" {
		u, err := url.Parse(cfg.Server.BaseURL)
		if err != nil {
			return nil, fmt.Errorf("parsing server.base_url: %w", err)
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return nil, fmt.Errorf("server.base_url must be an http or https URL, got %q", cfg.Server.BaseURL)
		}
		if u.Host == "" {
			return nil, fmt.Errorf("server.base_url names no host: %q", cfg.Server.BaseURL)
		}
		if p := strings.TrimSuffix(u.Path, "/"); p != "" {
			return nil, fmt.Errorf("server.base_url carries a path (%q); it must be scheme and host only", u.Path)
		}
		cfg.Server.BaseURL = strings.TrimSuffix(cfg.Server.BaseURL, "/")
	}

	// An enabled channel without a target is a configuration error rather than
	// a silent no-op: the whole point of the channel is that a failure is not
	// silent.
	if cfg.Alerts.Enabled && cfg.Alerts.NtfyURL == "" {
		return nil, fmt.Errorf("alerts.enabled is set but alerts.ntfy_url is empty")
	}
	if f := cfg.Alerts.Format; f != "ntfy" && f != "kuma-json" {
		return nil, fmt.Errorf("alerts.format must be \"ntfy\" or \"kuma-json\", got %q", f)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("FREEREPS_SERVER_HOST"); v != "" {
		cfg.Server.Host = v
	}
	if v := os.Getenv("FREEREPS_SERVER_BASE_URL"); v != "" {
		cfg.Server.BaseURL = v
	}
	if v := os.Getenv("FREEREPS_SERVER_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = port
		}
	}
	if v := os.Getenv("FREEREPS_ALERTS_ENABLED"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.Alerts.Enabled = b
		}
	}
	if v := os.Getenv("FREEREPS_ALERTS_NTFY_URL"); v != "" {
		cfg.Alerts.NtfyURL = v
	}
	if v := os.Getenv("FREEREPS_ALERTS_FORMAT"); v != "" {
		cfg.Alerts.Format = v
	}
	if v := os.Getenv("FREEREPS_DB_HOST"); v != "" {
		cfg.Database.Host = v
	}
	if v := os.Getenv("FREEREPS_DB_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Database.Port = port
		}
	}
	if v := os.Getenv("FREEREPS_DB_NAME"); v != "" {
		cfg.Database.Name = v
	}
	if v := os.Getenv("FREEREPS_DB_USER"); v != "" {
		cfg.Database.User = v
	}
	if v := os.Getenv("FREEREPS_DB_PASSWORD"); v != "" {
		cfg.Database.Password = v
	}
	if v := os.Getenv("FREEREPS_DB_SSLMODE"); v != "" {
		cfg.Database.SSLMode = v
	}
	if v := os.Getenv("FREEREPS_TS_ENABLED"); v != "" {
		cfg.Tailscale.Enabled = strings.EqualFold(v, "true") || v == "1"
	}
	if v := os.Getenv("FREEREPS_TS_HOSTNAME"); v != "" {
		cfg.Tailscale.Hostname = v
	}
	if v := os.Getenv("FREEREPS_TS_STATE_DIR"); v != "" {
		cfg.Tailscale.StateDir = v
	}
	if v := os.Getenv("FREEREPS_INGEST_TIMEZONE"); v != "" {
		cfg.Ingest.SessionTimezone = v
	}
}

func (c *Config) validate() error {
	if !c.Tailscale.Enabled && c.Server.Port == 0 {
		return fmt.Errorf("server.port is required when tailscale is disabled")
	}
	if c.Database.Host == "" {
		return fmt.Errorf("database.host is required")
	}
	if c.Database.Port == 0 {
		return fmt.Errorf("database.port is required")
	}
	if c.Database.Name == "" {
		return fmt.Errorf("database.name is required")
	}
	if c.Database.User == "" {
		return fmt.Errorf("database.user is required")
	}
	return nil
}
