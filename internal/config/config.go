// Package config loads the regionchecker configuration with the precedence
// flag > env REGIONCHECKER_* > yaml > default.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the runtime configuration.
type Config struct {
	CacheDir          string        `yaml:"cache_dir"`
	DBSource          string        `yaml:"db_source"`
	DBMaxAge          time.Duration `yaml:"db_max_age"`
	DBRefreshInterval time.Duration `yaml:"db_refresh_interval"`
	OnlineEnabled     bool          `yaml:"online_enabled"`
	IPInfoToken       string        `yaml:"ipinfo_token"`
	DNSTimeout        time.Duration `yaml:"dns_timeout"`
	DNSServers        []string      `yaml:"dns_servers"`
	Server            ServerConfig  `yaml:"server"`
	LogLevel          string        `yaml:"log_level"`
	LogFormat         string        `yaml:"log_format"`
	MMDBPath          string        `yaml:"mmdb_path"`
	ASNOrgBoosters    []string      `yaml:"asn_org_boosters"`
}

// ServerConfig controls the HTTP server subcommand.
type ServerConfig struct {
	Port         int           `yaml:"port"`
	RateLimit    int           `yaml:"rate_limit"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	MaxBatch     int           `yaml:"max_batch"`
	Bind         string        `yaml:"bind"`
}

// Load applies default → yaml (from path, may be empty) → env REGIONCHECKER_*.
// Flags are applied separately by the CLI layer.
func Load(yamlPath string) (Config, error) {
	cfg := Default()
	if yamlPath != "" {
		if err := loadYAML(&cfg, yamlPath); err != nil {
			return cfg, err
		}
	}
	applyEnv(&cfg)
	if cfg.CacheDir == "" {
		cfg.CacheDir = defaultCacheDir()
	}
	return cfg, nil
}

func loadYAML(cfg *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("config: read %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("config: parse %s: %w", path, err)
	}
	return nil
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("REGIONCHECKER_CACHE_DIR"); v != "" {
		cfg.CacheDir = v
	}
	if v := os.Getenv("REGIONCHECKER_DB_SOURCE"); v != "" {
		cfg.DBSource = v
	}
	if v := os.Getenv("REGIONCHECKER_DB_MAX_AGE"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.DBMaxAge = d
		}
	}
	if v := os.Getenv("REGIONCHECKER_ONLINE"); v != "" {
		cfg.OnlineEnabled = parseBool(v)
	}
	if v := os.Getenv("REGIONCHECKER_IPINFO_TOKEN"); v != "" {
		cfg.IPInfoToken = v
	}
	if v := os.Getenv("REGIONCHECKER_DNS_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.DNSTimeout = d
		}
	}
	if v := os.Getenv("REGIONCHECKER_DNS_SERVERS"); v != "" {
		cfg.DNSServers = splitCSV(v)
	}
	if v := os.Getenv("REGIONCHECKER_LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	if v := os.Getenv("REGIONCHECKER_LOG_FORMAT"); v != "" {
		cfg.LogFormat = v
	}
	if v := os.Getenv("REGIONCHECKER_MMDB_PATH"); v != "" {
		cfg.MMDBPath = v
	}
	if v := os.Getenv("REGIONCHECKER_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = n
		}
	}
	if v := os.Getenv("REGIONCHECKER_BIND"); v != "" {
		cfg.Server.Bind = v
	}
	if v := os.Getenv("REGIONCHECKER_RATE_LIMIT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Server.RateLimit = n
		}
	}
}

func parseBool(s string) bool {
	switch strings.ToLower(s) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// defaultCacheDir returns $XDG_CACHE_HOME/regionchecker or ~/.cache/regionchecker.
func defaultCacheDir() string {
	if v := os.Getenv("XDG_CACHE_HOME"); v != "" {
		return filepath.Join(v, "regionchecker")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".cache", "regionchecker")
	}
	return filepath.Join(os.TempDir(), "regionchecker")
}
