package output_test

import (
	"bytes"
	"encoding/json"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/binsarjr/regionchecker/internal/classifier"
	"github.com/binsarjr/regionchecker/internal/output"
)

func sample() *classifier.Result {
	return &classifier.Result{
		Input:          "google.co.id",
		Type:           "domain",
		Resolved:       []netip.Addr{netip.MustParseAddr("49.0.109.161")},
		DomainCountry:  "ID",
		DomainSuffix:   "cctld",
		IPCountry:      "ID",
		Registry:       "apnic",
		FinalCountry:   "ID",
		Confidence:     classifier.ConfHigh,
		Reason:         "domain cctld matches ip country",
		LookupDuration: 3 * time.Millisecond,
	}
}

func TestParse(t *testing.T) {
	cases := []struct {
		in   string
		want output.Format
		err  bool
	}{
		{"", output.FormatText, false},
		{"text", output.FormatText, false},
		{"JSON", output.FormatJSON, false},
		{"csv", output.FormatCSV, false},
		{"xml", "", true},
	}
	for _, tc := range cases {
		got, err := output.Parse(tc.in)
		if tc.err {
			if err == nil {
				t.Errorf("Parse(%q) expected error, got nil", tc.in)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("Parse(%q) = (%q, %v), want (%q, nil)", tc.in, got, err, tc.want)
		}
	}
}

func TestWriteText(t *testing.T) {
	var buf bytes.Buffer
	w := output.New(&buf, output.FormatText)
	if err := w.Write(sample()); err != nil {
		t.Fatal(err)
	}
	s := buf.String()
	if !strings.Contains(s, "google.co.id") || !strings.Contains(s, "ID") || !strings.Contains(s, "high") {
		t.Errorf("text output missing fields: %q", s)
	}
}

func TestWriteJSON(t *testing.T) {
	var buf bytes.Buffer
	w := output.New(&buf, output.FormatJSON)
	if err := w.Write(sample()); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if got["final_country"] != "ID" {
		t.Errorf("final_country = %v, want ID", got["final_country"])
	}
	if got["confidence"] != "high" {
		t.Errorf("confidence = %v, want high", got["confidence"])
	}
	if got["lookup_ms"] != float64(3) {
		t.Errorf("lookup_ms = %v, want 3", got["lookup_ms"])
	}
}

func TestWriteCSV(t *testing.T) {
	var buf bytes.Buffer
	w := output.New(&buf, output.FormatCSV)
	if err := w.Write(sample()); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("csv lines = %d, want 2", len(lines))
	}
	if !strings.HasPrefix(lines[0], "input,type,") {
		t.Errorf("csv header = %q", lines[0])
	}
	if !strings.Contains(lines[1], "google.co.id") || !strings.Contains(lines[1], "ID") {
		t.Errorf("csv row missing fields: %q", lines[1])
	}
}
