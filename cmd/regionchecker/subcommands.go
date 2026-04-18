package main

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/binsarjr/regionchecker/internal/asn"
	"github.com/binsarjr/regionchecker/internal/cache"
	"github.com/binsarjr/regionchecker/internal/classifier"
	"github.com/binsarjr/regionchecker/internal/config"
	"github.com/binsarjr/regionchecker/internal/output"
	"github.com/binsarjr/regionchecker/internal/resolver"
	"github.com/binsarjr/regionchecker/internal/rir"
	"github.com/binsarjr/regionchecker/internal/server"
)

const parsedSnapshotName = "ipv4-ranges.bin"

func checkCmd() *cli.Command {
	return &cli.Command{
		Name:      "check",
		Usage:     "Classify one or more IP/hostname inputs",
		ArgsUsage: "<host|ip>...",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "output", Aliases: []string{"o"}, Value: "text", Usage: "text|json|csv"},
			&cli.StringFlag{Name: "file", Aliases: []string{"f"}, Usage: "read inputs from file"},
			&cli.DurationFlag{Name: "timeout", Value: 5 * time.Second, Usage: "per-lookup DNS timeout"},
			&cli.BoolFlag{Name: "offline", Usage: "skip DNS resolution"},
			&cli.StringFlag{Name: "mmdb", Usage: "path to ASN MMDB for enrichment (optional)"},
		},
		Action: func(c *cli.Context) error {
			cfg, err := loadConfig(c)
			if err != nil {
				return err
			}
			inputs, err := collectInputs(c)
			if err != nil {
				return err
			}
			if len(inputs) == 0 {
				return fmt.Errorf("check: no input")
			}

			snap, err := openSnapshot(cfg)
			if err != nil {
				return err
			}
			defer snap.Close()

			var res classifier.Resolver
			if !c.Bool("offline") {
				res = resolver.New(c.Duration("timeout"), cfg.DNSServers)
			}
			cls := classifier.New(snap.DB, res, nil)
			if mmdbPath := c.String("mmdb"); mmdbPath != "" {
				db, err := asn.OpenMMDB(mmdbPath)
				if err != nil {
					return err
				}
				defer db.Close()
				cls.ASN = db
			}

			format, err := output.Parse(c.String("output"))
			if err != nil {
				return err
			}
			w := output.New(os.Stdout, format)
			defer w.Flush()

			for _, in := range dedup(inputs) {
				r, err := cls.Classify(c.Context, in)
				if err != nil {
					fmt.Fprintf(os.Stderr, "%s: %v\n", in, err)
					continue
				}
				if err := w.Write(r); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func updateDBCmd() *cli.Command {
	return &cli.Command{
		Name:  "update-db",
		Usage: "Refresh delegated-stats cache and rebuild parsed snapshot",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "source", Usage: "source name (nro, apnic, arin, ripe, lacnic, afrinic)"},
			&cli.BoolFlag{Name: "force", Usage: "force re-fetch"},
		},
		Action: func(c *cli.Context) error {
			cfg, err := loadConfig(c)
			if err != nil {
				return err
			}
			srcName := c.String("source")
			if srcName == "" {
				srcName = cfg.DBSource
			}
			src, ok := cache.Get(srcName)
			if !ok {
				return fmt.Errorf("update-db: unknown source %q", srcName)
			}
			store, err := cache.New(cfg.CacheDir)
			if err != nil {
				return err
			}
			fetcher := cache.NewFetcher(store, &http.Client{Timeout: 2 * time.Minute})
			body, err := fetcher.Fetch(c.Context, src.URL, src.Key)
			if err != nil {
				return fmt.Errorf("update-db: fetch %s: %w", src.URL, err)
			}
			db, err := rir.Build(bytes.NewReader(body))
			if err != nil {
				return fmt.Errorf("update-db: build: %w", err)
			}
			var buf bytes.Buffer
			if err := rir.Snapshot(db, [32]byte{}, &buf); err != nil {
				return fmt.Errorf("update-db: snapshot: %w", err)
			}
			if err := os.MkdirAll(filepath.Join(cfg.CacheDir, "parsed"), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(store.ParsedPath(parsedSnapshotName), buf.Bytes(), 0o644); err != nil {
				return err
			}
			fmt.Printf("updated %s: %d v4 ranges, %d v6 ranges, %d ASN ranges\n",
				src.Name, len(db.V4), len(db.V6), len(db.ASN))
			return nil
		},
	}
}

func cacheCmd() *cli.Command {
	return &cli.Command{
		Name:  "cache",
		Usage: "Cache inspection and maintenance",
		Subcommands: []*cli.Command{
			{
				Name:  "info",
				Usage: "Print cache metadata for each known source",
				Action: func(c *cli.Context) error {
					cfg, err := loadConfig(c)
					if err != nil {
						return err
					}
					store, err := cache.New(cfg.CacheDir)
					if err != nil {
						return err
					}
					names := make([]string, 0)
					for n := range cache.All() {
						names = append(names, n)
					}
					sort.Strings(names)
					for _, n := range names {
						src, _ := cache.Get(n)
						m, err := store.ReadMeta(src.Key)
						if err != nil {
							fmt.Printf("%s\tnot-cached\n", n)
							continue
						}
						age := time.Since(m.FetchedAt).Round(time.Second)
						fmt.Printf("%s\t%s\t%d bytes\t%s old\n", n, m.SHA256[:12], m.Bytes, age)
					}
					return nil
				},
			},
			{
				Name:  "clear",
				Usage: "Delete raw and parsed cache",
				Action: func(c *cli.Context) error {
					cfg, err := loadConfig(c)
					if err != nil {
						return err
					}
					for _, sub := range []string{"raw", "parsed"} {
						p := filepath.Join(cfg.CacheDir, sub)
						if err := os.RemoveAll(p); err != nil {
							return err
						}
						if err := os.MkdirAll(p, 0o755); err != nil {
							return err
						}
					}
					fmt.Println("cache cleared")
					return nil
				},
			},
		},
	}
}

func benchCmd() *cli.Command {
	return &cli.Command{
		Name:  "bench",
		Usage: "Benchmark lookup latency against random v4 addresses",
		Flags: []cli.Flag{
			&cli.IntFlag{Name: "samples", Value: 10_000},
		},
		Action: func(c *cli.Context) error {
			cfg, err := loadConfig(c)
			if err != nil {
				return err
			}
			snap, err := openSnapshot(cfg)
			if err != nil {
				return err
			}
			defer snap.Close()

			n := c.Int("samples")
			start := time.Now()
			for i := 0; i < n; i++ {
				u := rand.Uint32()
				b := [4]byte{byte(u >> 24), byte(u >> 16), byte(u >> 8), byte(u)}
				ip := netip.AddrFrom4(b)
				_, _, _ = snap.DB.LookupIP(ip)
			}
			elapsed := time.Since(start)
			fmt.Printf("bench: %d samples in %s (%.1f ns/op)\n",
				n, elapsed, float64(elapsed.Nanoseconds())/float64(n))
			return nil
		},
	}
}

func serveCmd() *cli.Command {
	return &cli.Command{
		Name:  "serve",
		Usage: "Run the HTTP API server",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "listen", Value: ":8080"},
			&cli.IntFlag{Name: "rate-limit", Value: 100},
		},
		Action: func(c *cli.Context) error {
			cfg, err := loadConfig(c)
			if err != nil {
				return err
			}
			snap, err := openSnapshot(cfg)
			if err != nil {
				return err
			}
			defer snap.Close()

			store, err := cache.New(cfg.CacheDir)
			if err != nil {
				return err
			}
			src, _ := cache.Get(cfg.DBSource)
			dbAge := func() (time.Duration, error) {
				m, err := store.ReadMeta(src.Key)
				if err != nil {
					return 0, err
				}
				return time.Since(m.FetchedAt), nil
			}

			res := resolver.New(cfg.DNSTimeout, cfg.DNSServers)
			cls := classifier.New(snap.DB, res, nil)
			metrics := server.NewMetrics()
			srv := server.New(server.Config{
				Addr:         c.String("listen"),
				ReadTimeout:  cfg.Server.ReadTimeout,
				WriteTimeout: cfg.Server.WriteTimeout,
				RateLimit:    c.Int("rate-limit"),
				MaxBatch:     cfg.Server.MaxBatch,
			}, cls, dbAge, metrics, nil)
			return srv.Run(c.Context)
		},
	}
}

// loadConfig resolves config from flags + env + optional YAML.
func loadConfig(c *cli.Context) (config.Config, error) {
	cfg, err := config.Load(c.String("config"))
	if err != nil {
		return cfg, err
	}
	if v := c.String("cache-dir"); v != "" {
		cfg.CacheDir = v
	}
	return cfg, nil
}

func openSnapshot(cfg config.Config) (*cache.Snapshot, error) {
	store, err := cache.New(cfg.CacheDir)
	if err != nil {
		return nil, err
	}
	path := store.ParsedPath(parsedSnapshotName)
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("snapshot not found at %s — run `regionchecker update-db`", path)
	}
	return cache.LoadSnapshot(path)
}

func collectInputs(c *cli.Context) ([]string, error) {
	args := c.Args().Slice()
	if file := c.String("file"); file != "" {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		for _, line := range bytes.Split(data, []byte{'\n'}) {
			s := string(bytes.TrimSpace(line))
			if s != "" && s[0] != '#' {
				args = append(args, s)
			}
		}
	}
	return args, nil
}

func dedup(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
