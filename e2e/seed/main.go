// seed builds a parsed RIR snapshot from a delegated-stats text file.
// Used by e2e tests to pre-populate the cache dir before invoking the CLI.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"

	"github.com/binsarjr/regionchecker/internal/rir"
)

func main() {
	input := flag.String("input", "", "delegated-stats input file")
	output := flag.String("output", "", "parsed snapshot output (ipv4-ranges.bin)")
	flag.Parse()
	if *input == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "usage: seed --input FILE --output FILE")
		os.Exit(2)
	}
	data, err := os.ReadFile(*input)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read input:", err)
		os.Exit(1)
	}
	db, err := rir.Build(bytes.NewReader(data))
	if err != nil {
		fmt.Fprintln(os.Stderr, "build:", err)
		os.Exit(1)
	}
	var buf bytes.Buffer
	if err := rir.Snapshot(db, [32]byte{}, &buf); err != nil {
		fmt.Fprintln(os.Stderr, "snapshot:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*output, buf.Bytes(), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "write output:", err)
		os.Exit(1)
	}
}
