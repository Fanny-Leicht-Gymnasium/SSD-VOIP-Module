package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

var (
	version    = "dev"
	repository = "unknown"
	branch     = "unknown"
)

var disableCensoring = false

func GetVersionInfo() (string, string, string) {
	return version, branch, repository
}

func main() {
	skipDockerCheck := false
	skipCompose := false
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--no-censor-tokens":
			disableCensoring = true
		case "--no-compose":
			skipCompose = true
		case "--no-docker-check":
			skipDockerCheck = true
		}
	}
	if !skipDockerCheck {
		if err := checkDockerDependencies(); err != nil {
			fmt.Println("Dependency check failed:", err)
			os.Exit(1)
		}
	}
	if !skipCompose {
		if err := installDockerCompose("."); err != nil {
			fmt.Println("Failed to install docker-compose.yml:", err)
			os.Exit(1)
		}
		if err := updateRunningContainers(); err != nil {
			fmt.Println("Failed to update running containers:", err)
			os.Exit(1)
		}

	}

	model := NewModel()

	program := tea.NewProgram(
		model,
		tea.WithAltScreen(),
	)

	model.program = program

	setupRestartSignal(program)

	if _, err := program.Run(); err != nil {
		fmt.Println("TUI failed:", err)
		os.Exit(1)
	}
}

// checkDockerDependencies verifies that Docker and Docker Compose are available.
func checkDockerDependencies() error {
	fmt.Println("Checking Docker dependencies...")

	// Check whether Docker is installed.
	if _, err := exec.LookPath("docker"); err != nil {
		fmt.Println()
		fmt.Println("Docker is not installed or is not available in PATH.")
		fmt.Println()
		fmt.Println("Please install Docker first:")
		fmt.Println("https://docs.docker.com/engine/install/")
		return fmt.Errorf("docker not found")
	}

	// Check whether the Docker daemon is reachable.
	if err := exec.Command("docker", "info").Run(); err != nil {
		fmt.Println()
		fmt.Println("Docker is installed, but the Docker daemon is not running")
		fmt.Println("or the current user cannot access it.")
		fmt.Println()
		fmt.Println("Please start Docker and try again.")
		return fmt.Errorf("docker daemon is unavailable")
	}

	// Check Docker Compose.
	if err := exec.Command("docker", "compose", "version").Run(); err == nil {
		fmt.Println("Docker: OK")
		fmt.Println("Docker Compose: OK")
		fmt.Println()
		return nil
	}

	fmt.Println()
	fmt.Println("Docker Compose is not installed.")
	fmt.Println()
	return fmt.Errorf("docker compose is required")

}
func updateRunningContainers() error {
	lockFile, err := acquireUpdateLock()
	if err != nil {
		// Another TUI instance is already handling the update.
		fmt.Println("Docker update skipped:", err)
		return nil
	}
	defer releaseUpdateLock(lockFile)
	services, err := getRunningComposeContainers()
	if err != nil {
		return err
	}

	for _, service := range services {
		fmt.Printf("Checking running container: %s\n", service)

		cmd := exec.Command(
			"docker",
			"compose",
			"pull",
			service,
		)
		cmd.Dir = projectDir()

		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf(
				"failed to pull %s: %w: %s",
				service,
				err,
				strings.TrimSpace(string(output)),
			)
		}

		fmt.Print(string(output))

		cmd = exec.Command(
			"docker",
			"compose",
			"up",
			"-d",
			"--no-deps",
			service,
		)
		cmd.Dir = projectDir()

		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf(
				"failed to update %s: %w: %s",
				service,
				err,
				strings.TrimSpace(string(output)),
			)
		}

		fmt.Print(string(output))
	}

	return nil
}
