package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	model := NewModel()

	program := tea.NewProgram(
		model,
		tea.WithAltScreen(),
	)

	model.program = program

	if _, err := program.Run(); err != nil {
		fmt.Println("TUI failed:", err)
		os.Exit(1)
	}
}
