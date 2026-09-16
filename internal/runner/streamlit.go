package runner

import (
	"log/slog"
	"os"
	"path/filepath"
	"syscall"
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
	os.Setenv("STREAMLIT_SERVER_HEADLESS", "true")
	os.Setenv("STREAMLIT_SERVER_ADDRESS", "0.0.0.0")
	return nil
}

// Exec runs `streamlit run <entrypoint>` via the venv's python -m streamlit,
// rather than exec'ing venv/bin/streamlit directly: that console script is a
// pip-generated shim with a hardcoded shebang pointing at the build host's
// venv path, which fixPythonSymlinks does not rewrite (it only fixes bin/python*
// symlinks), so exec'ing it directly would fail in the container.
func (r *StreamlitRunner) Exec() error {
	python := filepath.Join(venvDir, "bin", "python")
	entrypoint := filepath.Join(r.cfg.MountPath, r.cfg.Entrypoint)
	slog.Info("exec", "python", python, "entrypoint", entrypoint)
	return syscall.Exec(python, []string{python, "-m", "streamlit", "run", entrypoint}, os.Environ())
}
