package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type GitHubRelease struct {
	TagName    string `json:"tag_name"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	Assets     []struct {
		Name       string `json:"name"`
		BrowserURL string `json:"browser_download_url"`
	} `json:"assets"`
}

type versionCheckMsg struct {
	LatestVersion string
	DownloadURL   string
	Err           error
}

type updateResultMsg struct {
	Status string
	Err    error
}

func checkForUpdate() tea.Cmd {
	return func() tea.Msg {
		release, err := fetchLatestRelease()
		if err != nil {
			return versionCheckMsg{Err: err}
		}

		latest := strings.TrimSpace(release.TagName)

		if latest == "" {
			return versionCheckMsg{
				Err: fmt.Errorf("GitHub release has no version tag"),
			}
		}

		if !isSemver(version) {
			return versionCheckMsg{
				LatestVersion: latest,
			}
		}

		if !isNewerVersion(latest, version) {
			return versionCheckMsg{
				LatestVersion: latest,
			}
		}

		assetName := getTUIAssetName()

		for _, asset := range release.Assets {
			if asset.Name == assetName {
				return versionCheckMsg{
					LatestVersion: latest,
					DownloadURL:   asset.BrowserURL,
				}
			}
		}

		return versionCheckMsg{
			LatestVersion: latest,
			Err: fmt.Errorf(
				"release %s does not contain %s",
				latest,
				assetName,
			),
		}
	}
}

func getTUIAssetName() string {
	return fmt.Sprintf(
		"ssd-voip-tui-%s-%s",
		runtime.GOOS,
		runtime.GOARCH,
	)
}

func fetchLatestRelease() (*GitHubRelease, error) {
	repo := strings.TrimPrefix(repository, "https://github.com/")
	repo = strings.TrimSuffix(repo, "/")

	parts := strings.Split(repo, "/")

	if len(parts) != 2 {
		return nil, fmt.Errorf(
			"invalid GitHub repository: %s",
			repository,
		)
	}

	url := "https://api.github.com/repos/" + repo + "/releases/latest"

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "ssd-voip-tui")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"GitHub API returned %s",
			resp.Status,
		)
	}

	var release GitHubRelease

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	if release.Draft || release.Prerelease {
		return nil, fmt.Errorf("latest release is not stable")
	}

	return &release, nil
}

func isSemver(value string) bool {
	value = strings.TrimPrefix(strings.TrimSpace(value), "v")
	parts := strings.Split(value, ".")

	if len(parts) != 3 {
		return false
	}

	for _, part := range parts {
		if _, err := strconv.Atoi(part); err != nil {
			return false
		}
	}

	return true
}

func parseVersion(value string) ([3]int, bool) {
	var result [3]int

	value = strings.TrimPrefix(strings.TrimSpace(value), "v")
	parts := strings.Split(value, ".")

	if len(parts) != 3 {
		return result, false
	}

	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil {
			return result, false
		}

		result[i] = n
	}

	return result, true
}

func isNewerVersion(latest, current string) bool {
	a, okA := parseVersion(latest)
	b, okB := parseVersion(current)

	if !okA || !okB {
		return false
	}

	for i := 0; i < 3; i++ {
		if a[i] > b[i] {
			return true
		}

		if a[i] < b[i] {
			return false
		}
	}

	return false
}

func updateTUI(downloadURL string) tea.Cmd {
	return func() tea.Msg {
		executable, err := os.Executable()
		if err != nil {
			return updateResultMsg{Err: err}
		}

		executable, err = filepath.EvalSymlinks(executable)
		if err != nil {
			return updateResultMsg{Err: err}
		}

		tmpFile, err := os.CreateTemp(
			filepath.Dir(executable),
			".ssd-voip-tui-update-*",
		)
		if err != nil {
			return updateResultMsg{Err: err}
		}

		tmpPath := tmpFile.Name()

		defer func() {
			tmpFile.Close()
			os.Remove(tmpPath)
		}()

		client := &http.Client{
			Timeout: 5 * time.Minute,
		}

		req, err := http.NewRequest(
			http.MethodGet,
			downloadURL,
			nil,
		)
		if err != nil {
			return updateResultMsg{Err: err}
		}

		req.Header.Set("User-Agent", "ssd-voip-tui")

		resp, err := client.Do(req)
		if err != nil {
			return updateResultMsg{Err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return updateResultMsg{
				Err: fmt.Errorf(
					"download failed: %s",
					resp.Status,
				),
			}
		}

		if _, err := io.Copy(tmpFile, resp.Body); err != nil {
			return updateResultMsg{Err: err}
		}

		if err := tmpFile.Close(); err != nil {
			return updateResultMsg{Err: err}
		}

		if err := os.Chmod(tmpPath, 0755); err != nil {
			return updateResultMsg{Err: err}
		}

		backupPath := executable + ".old"

		_ = os.Remove(backupPath)

		if err := os.Rename(executable, backupPath); err != nil {
			return updateResultMsg{
				Err: fmt.Errorf(
					"cannot replace executable: %w",
					err,
				),
			}
		}

		if err := os.Rename(tmpPath, executable); err != nil {
			_ = os.Rename(backupPath, executable)

			return updateResultMsg{
				Err: fmt.Errorf(
					"cannot install update: %w",
					err,
				),
			}
		}

		_ = os.Remove(backupPath)

		return updateResultMsg{
			Status: "TUI updated successfully. Restart the application.",
		}
	}
}
