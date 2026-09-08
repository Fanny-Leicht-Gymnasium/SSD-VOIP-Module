package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func piperVoicesDir() string {
	return filepath.Join(projectDir(), "piper-voices")
}

// piperVoiceURLs returns the .onnx and .onnx.json download URLs for a
// Piper voice name, following the layout of the official
// rhasspy/piper-voices Hugging Face repo:
//
//	<lang>/<lang_REGION>/<speaker>/<quality>/<name>.onnx[.json]
//
// e.g. "de_DE-thorsten-medium" ->
//

func piperVoiceURLs(name string) (onnxURL, jsonURL string, err error) {
	parts := strings.SplitN(name, "-", 3)
	if len(parts) != 3 {
		return "", "", fmt.Errorf(
			"unrecognized Piper voice name %q (expected <lang_REGION>-<speaker>-<quality>, e.g. de_DE-thorsten-medium)",
			name,
		)
	}
	langRegion, speaker, quality := parts[0], parts[1], parts[2]

	lang := langRegion
	if i := strings.Index(langRegion, "_"); i >= 0 {
		lang = langRegion[:i]
	}

	base := fmt.Sprintf(
		"https://huggingface.co/rhasspy/piper-voices/resolve/main/%s/%s/%s/%s/%s",
		lang, langRegion, speaker, quality, name,
	)

	return base + ".onnx", base + ".onnx.json", nil
}

// downloadAndApplyPiperVoice downloads the .onnx + .onnx.json files for
// the given Piper voice into ./piper-voices on the host (bind-mounted
// into the bot container at /app/piper-voices), then recreates the bot
// container so it picks up the new PIPER_MODELS value from .env. No image
// rebuild is needed — the voice files live outside the image.
func downloadAndApplyPiperVoice(name string) tea.Cmd {
	return func() tea.Msg {
		name = strings.TrimSpace(name)
		if name == "" {
			return dockerCommandMsg{Status: "No Piper voice selected"}
		}

		onnxURL, jsonURL, err := piperVoiceURLs(name)
		if err != nil {
			return dockerCommandMsg{Status: "Voice download failed: " + err.Error(), Err: err}
		}

		dir := piperVoicesDir()
		if err := os.MkdirAll(dir, 0755); err != nil {
			return dockerCommandMsg{Status: "Voice download failed: " + err.Error(), Err: err}
		}

		if err := downloadFile(onnxURL, filepath.Join(dir, name+".onnx")); err != nil {
			return dockerCommandMsg{
				Status: fmt.Sprintf("Failed to download %s.onnx: %v", name, err),
				Err:    err,
			}
		}
		if err := downloadFile(jsonURL, filepath.Join(dir, name+".onnx.json")); err != nil {
			return dockerCommandMsg{
				Status: fmt.Sprintf("Failed to download %s.onnx.json: %v", name, err),
				Err:    err,
			}
		}

		cmd := exec.Command("docker", "compose", "up", "-d", "bot")
		cmd.Dir = projectDir()
		output, err := cmd.CombinedOutput()
		if err != nil {
			return dockerCommandMsg{
				Status: fmt.Sprintf("Voice %s downloaded, but restarting bot failed: %v", name, err),
				Output: string(output),
				Err:    err,
			}
		}

		return dockerCommandMsg{
			Status: fmt.Sprintf("Voice %s downloaded to piper-voices/ and bot restarted", name),
			Output: string(output),
		}
	}
}

func downloadFile(url, dest string) error {
	if _, err := os.Stat(dest); err == nil {
		// Already present locally — skip re-downloading.
		return nil
	}

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}

	tmp := dest + ".part"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	out.Close()

	return os.Rename(tmp, dest)
}