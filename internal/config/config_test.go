package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/binsarjr/regionchecker/internal/config"
)

func TestLoadDefaults(t *testing.T) {
	// Isolate env to avoid host pollution.
	clearEnv(t)
	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DBSource != "nro" {
		t.Errorf("DBSource = %q, want nro", cfg.DBSource)
	}
	if cfg.DBMaxAge != 48*time.Hour {
		t.Errorf("DBMaxAge = %v, want 48h", cfg.DBMaxAge)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("Server.Port = %d, want 8080", cfg.Server.Port)
	}
	if cfg.CacheDir == "" {
		t.Error("CacheDir should be resolved to XDG or home path")
	}
}

func TestLoadYAML(t *testing.T) {
	clearEnv(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	yaml := `
db_source: apnic
db_max_age: 72h
online_enabled: true
server:
  port: 9090
  rate_limit: 50
asn_org_boosters: [FOO, BAR]
`
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DBSource != "apnic" {
		t.Errorf("DBSource = %q, want apnic", cfg.DBSource)
	}
	if cfg.DBMaxAge != 72*time.Hour {
		t.Errorf("DBMaxAge = %v, want 72h", cfg.DBMaxAge)
	}
	if !cfg.OnlineEnabled {
		t.Error("OnlineEnabled should be true")
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("Server.Port = %d, want 9090", cfg.Server.Port)
	}
	if len(cfg.ASNOrgBoosters) != 2 || cfg.ASNOrgBoosters[0] != "FOO" {
		t.Errorf("ASNOrgBoosters = %v, want [FOO BAR]", cfg.ASNOrgBoosters)
	}
}

func TestLoadEnvOverridesYAML(t *testing.T) {
	clearEnv(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("db_source: apnic\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("REGIONCHECKER_DB_SOURCE", "arin")
	t.Setenv("REGIONCHECKER_PORT", "7070")
	t.Setenv("REGIONCHECKER_ONLINE", "true")
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DBSource != "arin" {
		t.Errorf("DBSource = %q, want arin (env override)", cfg.DBSource)
	}
	if cfg.Server.Port != 7070 {
		t.Errorf("Server.Port = %d, want 7070", cfg.Server.Port)
	}
	if !cfg.OnlineEnabled {
		t.Error("OnlineEnabled should be true via env")
	}
}

func TestLoadYAMLMissingOK(t *testing.T) {
	clearEnv(t)
	cfg, err := config.Load("/non/existent/path.yaml")
	if err != nil {
		t.Fatalf("Load should not error on missing yaml: %v", err)
	}
	if cfg.DBSource != "nro" {
		t.Errorf("expected defaults, got DBSource = %q", cfg.DBSource)
	}
}

func TestLoadCacheDirFromXDG(t *testing.T) {
	clearEnv(t)
	t.Setenv("XDG_CACHE_HOME", "/tmp/xdg-test")
	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.CacheDir != "/tmp/xdg-test/regionchecker" {
		t.Errorf("CacheDir = %q, want /tmp/xdg-test/regionchecker", cfg.CacheDir)
	}
}

func clearEnv(t *testing.T) {
	for _, k := range []string{
		"REGIONCHECKER_CACHE_DIR", "REGIONCHECKER_DB_SOURCE", "REGIONCHECKER_DB_MAX_AGE",
		"REGIONCHECKER_ONLINE", "REGIONCHECKER_IPINFO_TOKEN", "REGIONCHECKER_DNS_TIMEOUT",
		"REGIONCHECKER_DNS_SERVERS", "REGIONCHECKER_LOG_LEVEL", "REGIONCHECKER_LOG_FORMAT",
		"REGIONCHECKER_MMDB_PATH", "REGIONCHECKER_PORT", "REGIONCHECKER_BIND",
		"REGIONCHECKER_RATE_LIMIT", "XDG_CACHE_HOME",
	} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
}
