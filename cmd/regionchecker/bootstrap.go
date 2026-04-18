package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/binsarjr/regionchecker/internal/cache"
)

func bootstrapCmd() *cli.Command {
	return &cli.Command{
		Name:      "bootstrap",
		Usage:     "Pre-warm cache, sweep orphan tmp files, then exec a follow-up command",
		ArgsUsage: "[-- follow-up-cmd args...]",
		Flags: []cli.Flag{
			&cli.DurationFlag{Name: "orphan-age", Value: 5 * time.Minute, Usage: "tmp file age considered orphan"},
			&cli.BoolFlag{Name: "skip-update", Usage: "skip update-db when snapshot is missing"},
		},
		Action: func(c *cli.Context) error {
			cfg, err := loadConfig(c)
			if err != nil {
				return err
			}
			store, err := cache.New(cfg.CacheDir)
			if err != nil {
				return err
			}

			if err := sweepOrphans(store.TmpDir(), c.Duration("orphan-age")); err != nil {
				return fmt.Errorf("bootstrap: sweep tmp: %w", err)
			}

			// Update DB if snapshot missing (unless skipped).
			if !c.Bool("skip-update") {
				snapPath := store.ParsedPath(parsedSnapshotName)
				if _, statErr := os.Stat(snapPath); os.IsNotExist(statErr) {
					fmt.Fprintln(os.Stderr, "bootstrap: no parsed snapshot; running update-db")
					if err := updateDBCmd().Run(c); err != nil {
						return fmt.Errorf("bootstrap: update-db: %w", err)
					}
				}
			}

			// Exec follow-up command if provided.
			args := c.Args().Slice()
			if len(args) == 0 {
				return nil
			}
			return execFollowUp(args)
		},
	}
}

// sweepOrphans removes files under dir whose mtime is older than maxAge.
func sweepOrphans(dir string, maxAge time.Duration) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	cutoff := time.Now().Add(-maxAge)
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
	return nil
}

// execFollowUp replaces the current process with the given command.
// On POSIX systems this uses execve; the current binary is replaced.
func execFollowUp(args []string) error {
	exe, err := exeLookPath(args[0])
	if err != nil {
		return err
	}
	return syscall.Exec(exe, args, os.Environ())
}

// exeLookPath resolves name; absolute paths are passed through.
func exeLookPath(name string) (string, error) {
	if filepath.IsAbs(name) {
		return name, nil
	}
	// Only allow our own binary names to avoid picking up PATH surprises.
	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	if name == filepath.Base(self) || name == "regionchecker" {
		return self, nil
	}
	return name, nil
}
