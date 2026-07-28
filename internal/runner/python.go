package runner

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)


const venvDir = "/tmp/venv"

type PythonRunner struct {
	cfg Config
}

func (r *PythonRunner) Setup() error {
	if r.cfg.Entrypoint == "" {
		return fmt.Errorf("ENTRYPOINT env var is not set")
	}

	archive, err := findArchive(r.cfg.MountPath)
	if err != nil {
		return err
	}

	slog.Info("extracting venv", "archive", filepath.Base(archive))

	if err := extractVenv(archive, venvDir); err != nil {
		return fmt.Errorf("extract venv: %w", err)
	}
	if err := fixPythonSymlinks(venvDir); err != nil {
		return fmt.Errorf("fix python symlinks: %w", err)
	}

	return nil
}

func (r *PythonRunner) Exec() error {
	python := filepath.Join(venvDir, "bin", "python")
	entrypoint := filepath.Join(r.cfg.MountPath, r.cfg.Entrypoint)
	slog.Info("exec", "python", python, "entrypoint", entrypoint)
	return syscall.Exec(python, []string{python, entrypoint}, os.Environ())
}

// fixPythonSymlinks repoints any absolute python* symlinks in the venv (which
// point to the build-host interpreter path) to the container's interpreter.
func fixPythonSymlinks(venvDir string) error {
	binDir := filepath.Join(venvDir, "bin")
	entries, err := os.ReadDir(binDir)
	if err != nil {
		return err
	}

	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "python") {
			continue
		}

		linkPath := filepath.Join(binDir, e.Name())
		info, err := os.Lstat(linkPath)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			continue
		}

		target, err := os.Readlink(linkPath)
		if err != nil || !filepath.IsAbs(target) {
			continue // relative symlinks are already relocatable
		}

		// Absolute target from the build host — repoint to the container's binary.
		systemBin, err := exec.LookPath(e.Name())
		if err != nil {
			slog.Warn("no matching interpreter for venv symlink in this container, leaving it pointed at the build host path",
				"symlink", e.Name(), "buildHostTarget", target)
			continue
		}

		if err := os.Remove(linkPath); err != nil {
			return err
		}
		if err := os.Symlink(systemBin, linkPath); err != nil {
			return err
		}
		slog.Info("repointed venv symlink", "symlink", e.Name(), "from", target, "to", systemBin)
	}
	return nil
}
