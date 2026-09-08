package main

import (
	"fmt"
	"os"
	"os/exec"

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
