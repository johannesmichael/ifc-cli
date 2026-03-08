package cli

import (
	"io"
	"log/slog"
	"os"
)

// SetupLogging configures a structured logger based on CLI flags.
// Logs are written to stderr so stdout remains available for structured output.
func SetupLogging(verbose bool, quiet bool, logFile string, jsonFormat bool) *slog.Logger {
	level := slog.LevelWarn
	if verbose {
		level = slog.LevelInfo
	}
	if quiet {
		level = slog.LevelError
	}

	var w io.Writer = os.Stderr

	if logFile != "" {
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			// Fall back to stderr if we can't open the log file.
			slog.New(newHandler(os.Stderr, level, jsonFormat)).Error("failed to open log file", "path", logFile, "error", err)
		} else {
			w = io.MultiWriter(os.Stderr, f)
		}
	}

	return slog.New(newHandler(w, level, jsonFormat))
}

func newHandler(w io.Writer, level slog.Level, jsonFormat bool) slog.Handler {
	opts := &slog.HandlerOptions{Level: level}
	if jsonFormat {
		return slog.NewJSONHandler(w, opts)
	}
	return slog.NewTextHandler(w, opts)
}
