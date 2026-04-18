//go:build e2e

package e2e_test

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var (
	binaryPath string
	cacheDir   string
)

// TestMain builds the regionchecker binary into a temp dir, seeds a parsed
// RIR snapshot from testdata/delegated-synthetic.txt via `update-db`-style
// inline build, and exports CACHE_DIR/binary path for each test.
func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "regionchecker-e2e-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmp)

	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Dir(filepath.Dir(thisFile))

	binaryPath = filepath.Join(tmp, "regionchecker")
	if runtime.GOOS == "windows" {
		binaryPath += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/regionchecker")
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		panic("build failed: " + string(out))
	}

	cacheDir = filepath.Join(tmp, "cache")
	if err := seedSnapshot(repoRoot, cacheDir); err != nil {
		panic("seed snapshot: " + err.Error())
	}

	code := m.Run()
	os.Exit(code)
}

// seedSnapshot builds a RIR snapshot from the synthetic sample and writes it
// where `openSnapshot` expects (cache/parsed/ipv4-ranges.bin).
func seedSnapshot(repoRoot, cacheDir string) error {
	if err := os.MkdirAll(filepath.Join(cacheDir, "parsed"), 0o755); err != nil {
		return err
	}
	tool := filepath.Join(repoRoot, "testdata", "delegated-synthetic.txt")
	// Run a helper go program that reads the sample and writes a snapshot.
	helper := filepath.Join(repoRoot, "e2e", "seed", "main.go")
	cmd := exec.Command("go", "run", helper,
		"--input", tool,
		"--output", filepath.Join(cacheDir, "parsed", "ipv4-ranges.bin"))
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return &seedErr{out: string(out), err: err}
	}
	return nil
}

type seedErr struct {
	out string
	err error
}

func (s *seedErr) Error() string { return s.err.Error() + ": " + s.out }

func runCheck(t *testing.T, host, format string, extraArgs ...string) (string, int) {
	t.Helper()
	args := []string{"--cache-dir", cacheDir, "check", "--offline", "-o", format}
	args = append(args, extraArgs...)
	args = append(args, host)
	cmd := exec.Command(binaryPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if exit, ok := err.(*exec.ExitError); ok {
		code = exit.ExitCode()
	}
	combined := stdout.String()
	if stderr.Len() > 0 {
		combined += "\nSTDERR:" + stderr.String()
	}
	return combined, code
}

func TestCheckGoldenCSV(t *testing.T) {
	f, err := os.Open(filepath.Join("..", "testdata", "hosts-golden.csv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rdr := csv.NewReader(f)
	rows, err := rdr.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	// Skip header.
	for _, row := range rows[1:] {
		input, wantCC, wantConf := row[0], row[1], row[2]
		t.Run(input, func(t *testing.T) {
			out, _ := runCheck(t, input, "json")
			if strings.Contains(wantConf, "ErrBogon") {
				if !strings.Contains(out, "reserved range") && !strings.Contains(out, "bogon") {
					t.Errorf("bogon output missing for %s: %s", input, out)
				}
				return
			}
			var got map[string]any
			// JSON on stdout is the first line
			first := strings.SplitN(out, "\n", 2)[0]
			if err := json.Unmarshal([]byte(first), &got); err != nil {
				t.Fatalf("json unmarshal %q: %v", first, err)
			}
			if got["final_country"] != wantCC {
				t.Errorf("input=%s final_country=%v want=%s", input, got["final_country"], wantCC)
			}
			if got["confidence"] != wantConf {
				t.Errorf("input=%s confidence=%v want=%s", input, got["confidence"], wantConf)
			}
		})
	}
}
