package config

import (
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Auth     AuthConfig     `yaml:"auth"`
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

type AuthConfig struct {
	APIKey string `yaml:"api_key"`
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
//	FREEREPS_AUTH_API_KEY
func Load(path string) (*Config, error) {
	cfg := &Config{}

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
	if v := os.Getenv("FREEREPS_AUTH_API_KEY"); v != "" {
		cfg.Auth.APIKey = v
	}
}

func (c *Config) validate() error {
	if c.Server.Port == 0 {
		return fmt.Errorf("server.port is required")
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
	if c.Auth.APIKey == "" {
		return fmt.Errorf("auth.api_key is required")
	}
	return nil
}
