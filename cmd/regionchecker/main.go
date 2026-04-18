// Command regionchecker classifies IP addresses and hostnames to ISO country
// codes using offline RIR delegated-stats snapshots.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/urfave/cli/v2"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app := &cli.App{
		Name:    "regionchecker",
		Usage:   "Offline-first IP/domain to country classifier",
		Version: fmt.Sprintf("%s (%s, %s)", version, commit, buildDate),
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "config", Usage: "path to YAML config file"},
			&cli.StringFlag{Name: "cache-dir", Usage: "override cache directory"},
		},
		Commands: []*cli.Command{
			checkCmd(),
			updateDBCmd(),
			cacheCmd(),
			benchCmd(),
			serveCmd(),
			healthcheckCmd(),
			bootstrapCmd(),
			versionCmd(),
		},
	}

	if err := app.RunContext(ctx, os.Args); err != nil {
		slog.Error("exit", "err", err)
		os.Exit(1)
	}
}

func versionCmd() *cli.Command {
	return &cli.Command{
		Name:  "version",
		Usage: "Print build info",
		Action: func(c *cli.Context) error {
			fmt.Printf("regionchecker %s\ncommit: %s\nbuilt:  %s\n", version, commit, buildDate)
			return nil
		},
	}
}
