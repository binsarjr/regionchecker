// Package output writes classifier results as text, JSON, or CSV.
package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/binsarjr/regionchecker/internal/classifier"
)

// Format enumerates supported output formats.
type Format string

const (
	FormatText Format = "text"
	FormatJSON Format = "json"
	FormatCSV  Format = "csv"
)

// Parse returns a Format from a user-supplied string.
func Parse(s string) (Format, error) {
	switch strings.ToLower(s) {
	case "", "text":
		return FormatText, nil
	case "json":
		return FormatJSON, nil
	case "csv":
		return FormatCSV, nil
	}
	return "", fmt.Errorf("output: unknown format %q", s)
}

// Writer serialises classifier.Results to w in format f.
type Writer struct {
	w     io.Writer
	f     Format
	csv   *csv.Writer
	wrote bool
}

// New returns a writer. For CSV the header row is written on first Write.
func New(w io.Writer, f Format) *Writer {
	out := &Writer{w: w, f: f}
	if f == FormatCSV {
		out.csv = csv.NewWriter(w)
	}
	return out
}

// Write serialises a single result.
func (o *Writer) Write(r *classifier.Result) error {
	switch o.f {
	case FormatJSON:
		return o.writeJSON(r)
	case FormatCSV:
		return o.writeCSV(r)
	default:
		return o.writeText(r)
	}
}

// Flush flushes buffered CSV rows.
func (o *Writer) Flush() error {
	if o.csv != nil {
		o.csv.Flush()
		return o.csv.Error()
	}
	return nil
}

func (o *Writer) writeText(r *classifier.Result) error {
	_, err := fmt.Fprintf(o.w, "%s\t%s\t%s\t%s\n",
		r.Input, r.FinalCountry, r.Confidence, r.Reason)
	return err
}

func (o *Writer) writeJSON(r *classifier.Result) error {
	buf, err := json.Marshal(jsonView(r))
	if err != nil {
		return err
	}
	_, err = o.w.Write(append(buf, '\n'))
	return err
}

func (o *Writer) writeCSV(r *classifier.Result) error {
	if !o.wrote {
		if err := o.csv.Write([]string{
			"input", "type", "final_country", "confidence",
			"domain_country", "domain_suffix", "ip_country", "registry", "reason", "lookup_ms",
		}); err != nil {
			return err
		}
		o.wrote = true
	}
	return o.csv.Write([]string{
		r.Input,
		r.Type,
		r.FinalCountry,
		r.Confidence,
		r.DomainCountry,
		r.DomainSuffix,
		r.IPCountry,
		r.Registry,
		r.Reason,
		strconv.FormatInt(r.LookupDuration.Milliseconds(), 10),
	})
}

type jsonResult struct {
	Input          string   `json:"input"`
	Type           string   `json:"type"`
	Resolved       []string `json:"resolved,omitempty"`
	DomainCountry  string   `json:"domain_country,omitempty"`
	DomainSuffix   string   `json:"domain_suffix,omitempty"`
	IPCountry      string   `json:"ip_country,omitempty"`
	Registry       string   `json:"registry,omitempty"`
	FinalCountry   string   `json:"final_country"`
	Confidence     string   `json:"confidence"`
	Reason         string   `json:"reason"`
	LookupMillis   int64    `json:"lookup_ms"`
}

func jsonView(r *classifier.Result) jsonResult {
	resolved := make([]string, 0, len(r.Resolved))
	for _, a := range r.Resolved {
		resolved = append(resolved, a.String())
	}
	return jsonResult{
		Input:         r.Input,
		Type:          r.Type,
		Resolved:      resolved,
		DomainCountry: r.DomainCountry,
		DomainSuffix:  r.DomainSuffix,
		IPCountry:     r.IPCountry,
		Registry:      r.Registry,
		FinalCountry:  r.FinalCountry,
		Confidence:    r.Confidence,
		Reason:        r.Reason,
		LookupMillis:  r.LookupDuration.Milliseconds(),
	}
}
