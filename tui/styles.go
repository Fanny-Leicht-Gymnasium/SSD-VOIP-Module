package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	sectionTitleStyle = lipgloss.NewStyle().Bold(true)
	configPanelStyle  = panelStyle
	logsPanelStyle    = panelStyle
	footerStyle       = lipgloss.NewStyle()
	titleStyle        = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("205"))

	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1, 2)

	panelFocusedStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("62")).
				Padding(1, 2)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("62")).
			Bold(true)

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	valueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("117"))

	secretStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("203"))

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	tabActiveStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("62")).
			Padding(0, 1)

	tabStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Padding(0, 1)
)

func renderHeader(
	width int,
	asterisk string,
	bot string,
	service LogService,
) string {
	title := titleStyle.Render(
		"SSD VOIP MODULE 2",
	)

	status := fmt.Sprintf(
		"Asterisk: %s   Bot: %s   Logs: %s",
		formatServiceStatus(asterisk),
		formatServiceStatus(bot),
		service.String(),
	)

	right := statusStyle.Render(status)

	gap := width -
		lipgloss.Width(title) -
		lipgloss.Width(right)

	if gap < 1 {
		gap = 1
	}

	return title +
		strings.Repeat(" ", gap) +
		right
}

func formatServiceStatus(status string) string {
	switch strings.ToLower(status) {
	case "running":
		return "● RUNNING"

	case "exited":
		return "● EXITED"

	case "created":
		return "● CREATED"

	case "paused":
		return "● PAUSED"

	case "error":
		return "● ERROR"

	default:
		return "● " + strings.ToUpper(status)
	}
}

func renderConfig(
	m *Model,
) string {
	width := m.width/2 - 3

	if width < 40 {
		width = 40
	}

	var builder strings.Builder

	builder.WriteString(
		lipgloss.NewStyle().
			Bold(true).
			Render("Configuration"),
	)

	builder.WriteString("\n\n")

	for index, item := range fields {
		selected :=
			m.focus == FocusConfig &&
				m.cursor == index

		prefix := "  "

		if selected {
			prefix = "› "
		}

		value := item.Value

		if item.Secret {
			value = displayValue(
				value,
				true,
			)
		}

		line := fmt.Sprintf(
			"%s%-22s %s",
			prefix,
			item.Label,
			value,
		)

		if selected {
			line = selectedStyle.Render(line)
		}

		builder.WriteString(line)
		builder.WriteString("\n")
	}

	if m.editing {
		builder.WriteString("\n")

		builder.WriteString(
			lipgloss.NewStyle().
				Bold(true).
				Render("Edit"),
		)

		builder.WriteString("\n\n")
		builder.WriteString(m.input.View())
	}

	style := panelStyle

	if m.focus == FocusConfig {
		style = panelFocusedStyle
	}

	return style.
		Width(width).
		Render(builder.String())
}

func renderLogs(
	width int,
	height int,
	content string,
	service LogService,
	focused bool,
) string {
	panelWidth := width/2 - 3

	if panelWidth < 40 {
		panelWidth = 40
	}

	tabs := renderLogTabs(service)

	body := tabs +
		"\n\n" +
		content

	style := panelStyle

	if focused {
		style = panelFocusedStyle
	}

	return style.
		Width(panelWidth).
		Height(max(10, height-12)).
		Render(body)
}

func renderLogTabs(
	selected LogService,
) string {
	all := tabStyle.Render("3 All")
	asterisk := tabStyle.Render("1 Asterisk")
	bot := tabStyle.Render("2 Bot")

	switch selected {
	case LogAsterisk:
		asterisk = tabActiveStyle.Render("1 Asterisk")

	case LogBot:
		bot = tabActiveStyle.Render("2 Bot")

	default:
		all = tabActiveStyle.Render("3 All")
	}

	return all + " " + asterisk + " " + bot
}

func joinPanels(
	left string,
	right string,
) string {
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		left,
		" ",
		right,
	)
}

func renderStatus(
	status string,
	isError bool,
) string {
	if isError {
		return errorStyle.Render(
			"Status: " + status,
		)
	}

	if strings.Contains(
		strings.ToLower(status),
		"saved",
	) ||
		strings.Contains(
			strings.ToLower(status),
			"online",
		) ||
		strings.Contains(
			strings.ToLower(status),
			"started",
		) ||
		strings.Contains(
			strings.ToLower(status),
			"restarted",
		) {
		return successStyle.Render(
			"Status: " + status,
		)
	}

	return statusStyle.Render(
		"Status: " + status,
	)
}

func renderHelp(
	editMode bool,
) string {
	if editMode {
		return helpStyle.Render(
			"Enter save • Esc cancel",
		)
	}

	return helpStyle.Render(
		"↑/↓ Navigate • Enter Edit • Tab Panel • S Save • T ARI • R Restart • O Start • X Stop • C Clear • Q Quit",
	)
}
