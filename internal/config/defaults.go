package config

import "time"

// Default returns the default configuration.
func Default() Config {
	return Config{
		CacheDir:          "", // empty → XDG_CACHE_HOME/regionchecker at Load-time
		DBSource:          "nro",
		DBMaxAge:          48 * time.Hour,
		DBRefreshInterval: 24 * time.Hour,
		OnlineEnabled:     false,
		IPInfoToken:       "",
		DNSTimeout:        3 * time.Second,
		DNSServers:        nil,
		Server: ServerConfig{
			Port:         8080,
			RateLimit:    100,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
			MaxBatch:     1000,
			Bind:         "0.0.0.0",
		},
		LogLevel:       "info",
		LogFormat:      "json",
		MMDBPath:       "",
		ASNOrgBoosters: []string{"TELKOM", "BIZNET", "INDIHOME", "LINKNET", "CBN"},
	}
}
