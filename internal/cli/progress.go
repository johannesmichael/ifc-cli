package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// ProgressReporter displays import progress to stderr.
type ProgressReporter struct {
	w          io.Writer
	quiet      bool
	jsonFormat bool
	totalBytes int64
	startTime  time.Time
	lastUpdate time.Time
}

// NewProgressReporter creates a progress reporter.
// totalBytes is the input file size (used for percentage calculation).
func NewProgressReporter(w io.Writer, totalBytes int64, quiet bool, jsonFormat bool) *ProgressReporter {
	now := time.Now()
	return &ProgressReporter{
		w:          w,
		quiet:      quiet,
		jsonFormat: jsonFormat,
		totalBytes: totalBytes,
		startTime:  now,
		lastUpdate: now,
	}
}

// Update reports progress. Throttled to max 10 updates per second.
func (p *ProgressReporter) Update(bytesProcessed int64, entitiesParsed int64) {
	if p.quiet {
		return
	}
	now := time.Now()
	if now.Sub(p.lastUpdate) < 100*time.Millisecond {
		return
	}
	p.lastUpdate = now
	p.report(bytesProcessed, entitiesParsed)
}

// Finish prints the final summary line.
func (p *ProgressReporter) Finish(entitiesParsed int64, errors int64) {
	if p.quiet {
		return
	}
	elapsed := time.Since(p.startTime).Seconds()
	if elapsed < 0.001 {
		elapsed = 0.001
	}
	mbPerSec := float64(p.totalBytes) / (1024 * 1024) / elapsed

	if p.jsonFormat {
		msg := map[string]any{
			"bytes_processed": p.totalBytes,
			"entities":        entitiesParsed,
			"errors":          errors,
			"percent":         100,
			"mb_per_sec":      round2(mbPerSec),
			"done":            true,
		}
		b, _ := json.Marshal(msg)
		fmt.Fprintf(p.w, "%s\n", b)
		return
	}

	fmt.Fprintf(p.w, "\r[100%%] %d entities | %.1f MB/s | %d errors\n", entitiesParsed, mbPerSec, errors)
}

func (p *ProgressReporter) report(bytesProcessed int64, entitiesParsed int64) {
	elapsed := time.Since(p.startTime).Seconds()
	if elapsed < 0.001 {
		elapsed = 0.001
	}

	percent := 0
	if p.totalBytes > 0 {
		percent = int(bytesProcessed * 100 / p.totalBytes)
		if percent > 99 {
			percent = 99
		}
	}

	mbPerSec := float64(bytesProcessed) / (1024 * 1024) / elapsed

	if p.jsonFormat {
		msg := map[string]any{
			"bytes_processed": bytesProcessed,
			"entities":        entitiesParsed,
			"percent":         percent,
			"mb_per_sec":      round2(mbPerSec),
		}
		b, _ := json.Marshal(msg)
		fmt.Fprintf(p.w, "%s\n", b)
		return
	}

	fmt.Fprintf(p.w, "\r[%2d%%] %d entities | %.1f MB/s", percent, entitiesParsed, mbPerSec)
}

func round2(f float64) float64 {
	return float64(int(f*100)) / 100
}
