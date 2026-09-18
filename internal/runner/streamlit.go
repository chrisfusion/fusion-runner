package runner

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
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

	// WEAVE_INGRESS_PATH (from metadata.yaml's ingress.path) is a leading-slash
	// path like "/showcase" or root "/". Streamlit's own baseUrlPath config takes
	// a bare segment with no slashes, and must be unset (not "") for root — so
	// only set it when there's an actual non-root segment.
	if basePath := strings.Trim(r.cfg.IngressPath, "/"); basePath != "" {
		slog.Info("setting STREAMLIT_SERVER_BASE_URL_PATH", "basePath", basePath)
		os.Setenv("STREAMLIT_SERVER_BASE_URL_PATH", basePath)
	}
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
