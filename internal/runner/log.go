package runner

import (
	"log/slog"
	"os"
)

// NewLogger builds the process-wide logger with the identifying fields of cfg
// (artifact, tag, version, namespace, runner type) attached to every record,
// so any log line — including ones emitted deep inside Setup()/Exec() — can be
// traced back to the metadata that produced it without relying on the startup
// banner having been printed first.
func NewLogger(cfg Config) *slog.Logger {
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})
	return slog.New(handler).With(
		"artifact", cfg.Artifact,
		"tag", cfg.Tag,
		"version", cfg.Version,
		"namespace", cfg.Namespace,
		"runnerType", cfg.RunnerType,
	)
}
