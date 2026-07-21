package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
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

	fmt.Printf("fusion-runner: downloading %s:%s (resolved version %s, %d file(s)) from fusion-index...\n",
		r.cfg.Artifact, r.cfg.Tag, version, len(files))

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

	return "", nil, fmt.Errorf("no version or tag matching %q", ref)
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
		return fmt.Errorf("GET %s: status %d", downloadURL, resp.StatusCode)
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
		return fmt.Errorf("GET %s: 404 not found", u)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: status %d", u, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}
