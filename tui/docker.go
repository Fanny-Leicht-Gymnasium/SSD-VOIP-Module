package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
)

var dockerMu sync.Mutex
var dockerCmd *exec.Cmd
var dockerCancel context.CancelFunc

type LogService string

const (
	LogAll      LogService = ""
	LogAsterisk LogService = "asterisk"
	LogBot      LogService = "bot"
)

func (service LogService) String() string {
	if service == "" {
		return "All"
	}
	return string(service)
}

func runDockerCommand(status string, args ...string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("docker", args...)
		cmd.Dir = projectDir()

		output, err := cmd.CombinedOutput()
		if err != nil {
			return dockerCommandMsg{
				Status: fmt.Sprintf("%s failed: %v", status, err),
				Output: string(output),
				Err:    err,
			}
		}

		return dockerCommandMsg{
			Status: status,
			Output: string(output),
		}
	}
}

func startDockerLogs(program *tea.Program, service string, session uint64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithCancel(context.Background())

		args := []string{"compose", "logs", "-f", "--tail", "200"}

		if service != "" {
			args = append(args, service)
		}

		cmd := exec.CommandContext(ctx, "docker", args...)
		cmd.Dir = projectDir()

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			cancel()
			return logLineMsg{
				Session: session,
				Line:    fmt.Sprintf("Failed to create stdout pipe: %v", err),
			}
		}

		stderr, err := cmd.StderrPipe()
		if err != nil {
			cancel()
			return logLineMsg{
				Session: session,
				Line:    fmt.Sprintf("Failed to create stderr pipe: %v", err),
			}
		}

		if err := cmd.Start(); err != nil {
			cancel()
			return logLineMsg{
				Session: session,
				Line:    fmt.Sprintf("Failed to start Docker logs: %v", err),
			}
		}

		dockerMu.Lock()
		dockerCmd = cmd
		dockerCancel = cancel
		dockerMu.Unlock()

		go streamReader(stdout, program, session)
		go streamReader(stderr, program, session)

		go func() {
			err := cmd.Wait()

			dockerMu.Lock()
			if dockerCmd == cmd {
				dockerCmd = nil
				dockerCancel = nil
			}
			dockerMu.Unlock()

			if err != nil {
				program.Send(logLineMsg{
					Session: session,
					Line:    fmt.Sprintf("Docker logs stopped: %v", err),
				})
			}
		}()

		return dockerLogsStartedMsg{
			Session: session,
		}
	}
}

func stopDockerLogs() {
	dockerMu.Lock()
	defer dockerMu.Unlock()

	if dockerCancel != nil {
		dockerCancel()
		dockerCancel = nil
	}

	dockerCmd = nil
}

func (m *Model) refreshDockerStatus() tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("docker", "compose", "ps", "--format", "{{.Service}}={{.State}}")
		cmd.Dir = projectDir()
		output, err := cmd.Output()
		if err != nil {
			return dockerStatusMsg{Asterisk: "error", Bot: "error"}
		}
		status := map[string]string{}
		for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
			key, value, ok := strings.Cut(line, "=")
			if ok {
				status[key] = value
			}
		}
		return dockerStatusMsg{Asterisk: status["asterisk"], Bot: status["bot"]}
	}
}

func openShell() tea.Cmd {
	return func() tea.Msg {
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/sh"
		}
		cmd := exec.Command(shell)
		cmd.Dir = projectDir()
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return dockerCommandMsg{Status: fmt.Sprintf("Shell failed: %v", err), Err: err}
		}
		return dockerCommandMsg{Status: "Shell closed"}
	}
}

func streamReader(
	reader interface {
		Read([]byte) (int, error)
	},
	program *tea.Program,
	session uint64,
) {
	buffer := make([]byte, 4096)
	var pending string

	for {
		n, err := reader.Read(buffer)

		if n > 0 {
			pending += string(buffer[:n])

			lines := strings.Split(pending, "\n")
			pending = lines[len(lines)-1]

			for _, line := range lines[:len(lines)-1] {
				line = strings.TrimRight(line, "\r")

				if line != "" {
					program.Send(logLineMsg{
						Session: session,
						Line:    line,
					})
				}
			}
		}

		if err != nil {
			if pending != "" {
				program.Send(logLineMsg{
					Session: session,
					Line:    pending,
				})
			}
			return
		}
	}
}
func getRunningComposeContainers() ([]string, error) {
	cmd := exec.Command(
		"docker",
		"compose",
		"ps",
		"--services",
		"--filter",
		"status=running",
	)
	cmd.Dir = projectDir()

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get running compose services: %w: %s",
			err,
			strings.TrimSpace(string(output)),
		)
	}

	var services []string

	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		service := strings.TrimSpace(line)

		if service != "" {
			services = append(services, service)
		}
	}

	return services, nil
}

func imageUpdateAvailable(service string) (bool, error) {
	cmd := exec.Command(
		"docker",
		"compose",
		"pull",
		service,
	)
	cmd.Dir = projectDir()

	output, err := cmd.CombinedOutput()
	outputText := string(output)

	if err != nil {
		return false, fmt.Errorf(
			"failed to pull image for %s: %w: %s",
			service,
			err,
			strings.TrimSpace(outputText),
		)
	}

	lowerOutput := strings.ToLower(outputText)

	return strings.Contains(lowerOutput, "downloaded newer image") ||
		strings.Contains(lowerOutput, "pulled") ||
		strings.Contains(lowerOutput, "pull complete"), nil
}

func pullImage(service string) error {
	// The image has already been pulled by imageUpdateAvailable.
	return nil
}

func recreateContainer(service string) error {
	cmd := exec.Command(
		"docker",
		"compose",
		"up",
		"-d",
		"--no-deps",
		service,
	)
	cmd.Dir = projectDir()

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"failed to recreate %s: %w: %s",
			service,
			err,
			strings.TrimSpace(string(output)),
		)
	}

	return nil
}
