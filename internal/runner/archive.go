package runner

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// known top-level directories inside a bare (non-prefixed) venv-pack archive
var venvRootDirs = map[string]bool{
	"bin": true, "lib": true, "lib64": true,
	"include": true, "etc": true, "share": true, "local": true,
}

func findArchive(dir string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.tar.gz"))
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no .tar.gz archive found in %s", dir)
	}
	return matches[0], nil
}

// extractVenv extracts a venv-pack archive to venvDir, handling both:
//   - bare archives where entries start directly with bin/, lib/, etc.
//   - prefixed archives where entries are under a top-level subdirectory (e.g. venv/)
func extractVenv(archive, venvDir string) error {
	topDir, err := archiveTopDir(archive)
	if err != nil {
		return fmt.Errorf("inspect archive: %w", err)
	}

	if topDir == "" {
		// Bare archive — extract directly into venvDir
		if err := os.MkdirAll(venvDir, 0755); err != nil {
			return err
		}
		return extractTarGz(archive, venvDir)
	}

	// Prefixed archive — extract to parent, then rename if the top dir name differs
	parent := filepath.Dir(venvDir)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return err
	}
	if err := extractTarGz(archive, parent); err != nil {
		return err
	}
	extracted := filepath.Join(parent, topDir)
	if extracted == venvDir {
		return nil
	}
	return os.Rename(extracted, venvDir)
}

// archiveTopDir peeks at the first real entry of a .tar.gz to detect a
// top-level subdirectory prefix. Returns "" for bare archives (entries start
// directly with bin/, lib/, etc.). Uses the first path component of the first
// non-empty entry rather than relying on TypeDir, which is not set consistently
// by all archive tools (e.g. venv-pack).
func archiveTopDir(archive string) (string, error) {
	f, err := os.Open(archive)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err != nil {
			return "", nil
		}
		// Take the first path component, ignoring leading "./" and trailing "/".
		top := strings.SplitN(strings.TrimPrefix(strings.TrimSuffix(hdr.Name, "/"), "./"), "/", 2)[0]
		if top == "" || top == "." {
			continue // skip "." and empty root entries
		}
		if venvRootDirs[top] {
			return "", nil // first component is a known venv dir → bare archive
		}
		return top, nil // first component is a wrapper directory
	}
}

func extractTarGz(archive, dest string) error {
	cmd := exec.Command("tar", "-xzf", archive, "-C", dest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
