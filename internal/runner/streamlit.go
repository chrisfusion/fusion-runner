package runner

import (
	"log/slog"
	"os"
)

type StreamlitRunner struct {
	PythonRunner
}

func (r *StreamlitRunner) Setup() error {
	if err := r.PythonRunner.Setup(); err != nil {
		return err
	}
	if r.cfg.Port != "" {
		slog.Info("setting STREAMLIT_SERVER_PORT", "port", r.cfg.Port)
		os.Setenv("STREAMLIT_SERVER_PORT", r.cfg.Port)
	}
	return nil
}
