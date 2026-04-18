package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/urfave/cli/v2"
)

func healthcheckCmd() *cli.Command {
	return &cli.Command{
		Name:  "healthcheck",
		Usage: "Probe HTTP endpoint and exit 0/1 (for Docker HEALTHCHECK)",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "addr", Value: "http://127.0.0.1:8080/healthz"},
			&cli.DurationFlag{Name: "timeout", Value: 3 * time.Second},
		},
		Action: func(c *cli.Context) error {
			ctx, cancel := context.WithTimeout(c.Context, c.Duration("timeout"))
			defer cancel()
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.String("addr"), nil)
			if err != nil {
				return err
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return fmt.Errorf("healthcheck: %w", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				return fmt.Errorf("healthcheck: status %d", resp.StatusCode)
			}
			return nil
		},
	}
}
