package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
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
	Status     string
	Err        error
	Executable string
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

// fileID identifies a file by device and inode, which stays stable across
// renames. Comparing /proc/PID/exe targets by path breaks once the running
// binary is renamed out from under a process (as happens during an update),
// because the /proc magic symlink then resolves to the new path of that
// same file. Comparing by inode instead is rename-proof.
type fileID struct {
	Dev uint64
	Ino uint64
}

func getFileID(path string) (fileID, error) {
	info, err := os.Stat(path)
	if err != nil {
		return fileID{}, err
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fileID{}, fmt.Errorf("cannot determine inode for %s", path)
	}

	return fileID{Dev: uint64(stat.Dev), Ino: stat.Ino}, nil
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

		// Capture the identity of the currently running binary before we
		// touch it. The inode survives the rename below, unlike the path.
		oldID, err := getFileID(executable)
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

		//_ = os.Remove(backupPath)
		restartOtherInstances(oldID)
		return updateResultMsg{
			Status:     "TUI updated successfully.",
			Executable: executable,
		}
	}
}
func restartTUI(executable string) error {
	if executable == "" {
		return fmt.Errorf("executable path is empty")
	}

	return syscall.Exec(
		executable,
		os.Args,
		os.Environ(),
	)
}

func findTUIInstances(target fileID) []int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}

	currentPID := os.Getpid()
	var pids []int

	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}

		if pid == currentPID {
			continue
		}

		// os.Stat follows the /proc/PID/exe symlink and reports the
		// identity of the file the process is actually running, even if
		// that file has since been renamed.
		id, err := getFileID(filepath.Join("/proc", entry.Name(), "exe"))
		if err != nil {
			continue
		}

		if id == target {
			pids = append(pids, pid)
		}
	}

	return pids
}
func restartOtherInstances(target fileID) {
	pids := findTUIInstances(target)

	for _, pid := range pids {
		process, err := os.FindProcess(pid)
		if err != nil {
			continue
		}

		_ = process.Signal(syscall.SIGUSR1)
	}
}
func setupRestartSignal(program *tea.Program) {
	restartSignal := make(chan os.Signal, 1)

	signal.Notify(restartSignal, syscall.SIGUSR1)

	go func() {
		for range restartSignal {
			program.Send(restartMsg{})
		}
	}()
}
