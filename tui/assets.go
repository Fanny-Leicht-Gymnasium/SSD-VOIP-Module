package main

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed assets/docker-compose.yml
var dockerComposeYAML []byte

func installDockerCompose(targetDir string) error {
	targetPath := filepath.Join(targetDir, "docker-compose.yml")

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("create target directory: %w", err)
	}

	// Write to a temporary file first so a failed update cannot corrupt the compose file.
	tmp, err := os.CreateTemp(targetDir, ".docker-compose-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary compose file: %w", err)
	}

	tmpPath := tmp.Name()

	defer func() {
		tmp.Close()
		os.Remove(tmpPath)
	}()

	if _, err := tmp.Write(dockerComposeYAML); err != nil {
		return fmt.Errorf("write compose file: %w", err)
	}

	if err := tmp.Chmod(0644); err != nil {
		return fmt.Errorf("set compose file permissions: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close compose file: %w", err)
	}

	if err := os.Rename(tmpPath, targetPath); err != nil {
		return fmt.Errorf("install compose file: %w", err)
	}

	return nil
}
