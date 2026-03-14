package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Database  DatabaseConfig  `yaml:"database"`
	Tailscale TailscaleConfig `yaml:"tailscale"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
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
//	FREEREPS_SERVER_HOST, FREEREPS_SERVER_PORT,
//	FREEREPS_DB_HOST, FREEREPS_DB_PORT, FREEREPS_DB_NAME,
//	FREEREPS_DB_USER, FREEREPS_DB_PASSWORD, FREEREPS_DB_SSLMODE,
//	FREEREPS_TS_ENABLED, FREEREPS_TS_HOSTNAME, FREEREPS_TS_STATE_DIR
func Load(path string) (*Config, error) {
	cfg := &Config{
		Tailscale: TailscaleConfig{
			Enabled:  true,
			Hostname: "freereps",
			StateDir: "tsnet-state",
		},
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	applyEnvOverrides(cfg)

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("FREEREPS_SERVER_HOST"); v != "" {
		cfg.Server.Host = v
	}
	if v := os.Getenv("FREEREPS_SERVER_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = port
		}
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
