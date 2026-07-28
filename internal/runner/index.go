package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// IndexPythonRunner resolves an artifact name + tag (or literal semver) against
// fusion-index over REST, downloads every file attached to the resolved version
// into cfg.MountPath, then delegates to PythonRunner for venv extraction and exec.
type IndexPythonRunner struct {
	PythonRunner
}

func (r *IndexPythonRunner) Setup() error {
	if r.cfg.IndexURL == "" {
		return fmt.Errorf("INDEX_URL env var is not set")
	}
	if r.cfg.Artifact == "" {
		return fmt.Errorf("WEAVE_ARTIFACT env var is not set")
	}
	if r.cfg.Tag == "" {
		return fmt.Errorf("WEAVE_TAG env var is not set")
	}

	ctx := context.Background()

	artifactID, err := findArtifactID(ctx, r.cfg.IndexURL, r.cfg.Artifact)
	if err != nil {
		return fmt.Errorf("resolve artifact %q: %w", r.cfg.Artifact, err)
	}

	version, files, err := resolveVersionFiles(ctx, r.cfg.IndexURL, artifactID, r.cfg.Tag)
	if err != nil {
		return fmt.Errorf("resolve %s:%s: %w", r.cfg.Artifact, r.cfg.Tag, err)
	}

	slog.Info("downloading artifact files from fusion-index",
		"resolvedVersion", version, "fileCount", len(files))

	if err := os.MkdirAll(r.cfg.MountPath, 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", r.cfg.MountPath, err)
	}

	for _, f := range files {
		dest := filepath.Join(r.cfg.MountPath, filepath.Base(f.Name))
		if err := downloadIndexFile(ctx, r.cfg.IndexURL, f.DownloadURL, dest); err != nil {
			return fmt.Errorf("download %s: %w", f.Name, err)
		}
	}

	return r.PythonRunner.Setup()
}

// --- fusion-index REST client (minimal, stdlib-only) ---

type indexArtifactPage struct {
	Items []struct {
		ID int64 `json:"id"`
	} `json:"items"`
}

type indexVersion struct {
	Version string `json:"version"`
	Tags    []struct {
		Tag string `json:"tag"`
	} `json:"tags"`
}

type indexFile struct {
	Name        string `json:"name"`
	DownloadURL string `json:"downloadUrl"`
}

func findArtifactID(ctx context.Context, indexURL, artifactName string) (int64, error) {
	u := fmt.Sprintf("%s/api/v1/artifacts?name=%s", indexURL, url.QueryEscape(artifactName))
	var page indexArtifactPage
	if err := getJSON(ctx, u, &page); err != nil {
		return 0, err
	}
	for _, item := range page.Items {
		if item.ID != 0 {
			return item.ID, nil
		}
	}
	return 0, fmt.Errorf("artifact %q not found", artifactName)
}

// resolveVersionFiles matches ref against either a version's literal semver
// string or any tag assigned to it, then returns that version's files.
func resolveVersionFiles(ctx context.Context, indexURL string, artifactID int64, ref string) (string, []indexFile, error) {
	u := fmt.Sprintf("%s/api/v1/artifacts/%d/versions", indexURL, artifactID)
	var versions []indexVersion
	if err := getJSON(ctx, u, &versions); err != nil {
		return "", nil, err
	}

	for _, v := range versions {
		matched := v.Version == ref
		for _, t := range v.Tags {
			if t.Tag == ref {
				matched = true
			}
		}
		if !matched {
			continue
		}

		fu := fmt.Sprintf("%s/api/v1/artifacts/%d/versions/%s/files", indexURL, artifactID, v.Version)
		var files []indexFile
		if err := getJSON(ctx, fu, &files); err != nil {
			return "", nil, err
		}
		if len(files) == 0 {
			return "", nil, fmt.Errorf("version %s has no files", v.Version)
		}
		return v.Version, files, nil
	}

	return "", nil, fmt.Errorf("no version or tag matching %q (available: %s)", ref, describeVersions(versions))
}

// describeVersions renders the versions/tags fusion-index actually returned,
// so a resolution failure shows what was available instead of just the ref
// that didn't match anything.
func describeVersions(versions []indexVersion) string {
	if len(versions) == 0 {
		return "<no versions registered for this artifact>"
	}
	parts := make([]string, len(versions))
	for i, v := range versions {
		tags := make([]string, len(v.Tags))
		for j, t := range v.Tags {
			tags[j] = t.Tag
		}
		if len(tags) == 0 {
			parts[i] = v.Version
		} else {
			parts[i] = fmt.Sprintf("%s [%s]", v.Version, strings.Join(tags, ", "))
		}
	}
	return strings.Join(parts, ", ")
}

func downloadIndexFile(ctx context.Context, indexURL, downloadURL, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, indexURL+downloadURL, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: status %d: %s", downloadURL, resp.StatusCode, readBodySnippet(resp))
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = out.ReadFrom(resp.Body)
	return err
}

func getJSON(ctx context.Context, u string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("GET %s: 404 not found: %s", u, readBodySnippet(resp))
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: status %d: %s", u, resp.StatusCode, readBodySnippet(resp))
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}

// readBodySnippet returns a short, single-line excerpt of a non-2xx response
// body for error messages — fusion-index's error responses (validation
// messages, stack traces) are otherwise discarded once resp.Body is closed.
func readBodySnippet(resp *http.Response) string {
	const maxLen = 500
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxLen))
	if err != nil || len(body) == 0 {
		return "<no response body>"
	}
	snippet := strings.TrimSpace(strings.ReplaceAll(string(body), "\n", " "))
	if len(snippet) == maxLen {
		snippet += "..."
	}
	return snippet
}
