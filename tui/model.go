package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Focus int

const (
	FocusConfig Focus = iota
	FocusLogs
)

type logLineMsg struct {
	Session uint64
	Line    string
}
type dockerLogsStartedMsg struct{ Session uint64 }
type dockerCommandMsg struct {
	Status, Output string
	Err            error
}
type dockerStatusMsg struct{ Asterisk, Bot string }
type statusTickMsg struct{}

type Model struct {
	width, height                            int
	focus                                    Focus
	fields                                   []field
	cursor                                   int
	editing                                  bool
	input                                    textinput.Model
	logs                                     []string
	maxLogs                                  int
	logSession                               uint64
	logService                               LogService
	viewport                                 viewport.Model
	statusMessage, asteriskStatus, botStatus string
	program                                  *tea.Program
}

func NewModel() *Model {
	input := textinput.New()
	input.CharLimit = 500
	input.Width = 50
	m := &Model{focus: FocusConfig, fields: defaultFields(), maxLogs: 5000, input: input}
	m.viewport = viewport.New(80, 20)
	m.viewport.SetContent("Docker logs will appear here...")
	return m
}
func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.refreshDockerStatus(), tea.Tick(3*time.Second, func(time.Time) tea.Msg { return statusTickMsg{} }))
}
func (m *Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.updateLayout()
		if m.program != nil && m.logSession == 0 {
			return m, m.startLogs()
		}
	case statusTickMsg:
		return m, tea.Batch(m.refreshDockerStatus(), tea.Tick(3*time.Second, func(time.Time) tea.Msg { return statusTickMsg{} }))
	case logLineMsg:
		if msg.Session == m.logSession {
			m.addLog(msg.Line)
		}
	case dockerLogsStartedMsg:
		if msg.Session == m.logSession {
			m.statusMessage = "Docker logs: " + displayLogService(m.logService)
		}
	case dockerCommandMsg:
		m.statusMessage = msg.Status
		for _, line := range strings.Split(strings.TrimRight(msg.Output, "\n"), "\n") {
			m.addLog(line)
		}
	case dockerStatusMsg:
		m.asteriskStatus, m.botStatus = msg.Asterisk, msg.Bot
	case tea.KeyMsg:
		return m, m.handleKey(msg)
	}
	return m, nil
}
func (m *Model) handleKey(msg tea.KeyMsg) tea.Cmd {
	if m.editing {
		switch msg.String() {
		case "esc":
			m.editing = false
			m.input.Blur()
			return nil
		case "enter":
			m.saveCurrentInput()
			m.editing = false
			m.input.Blur()
			return nil
		}
		var c tea.Cmd
		m.input, c = m.input.Update(msg)
		return c
	}
	switch msg.String() {
	case "q", "ctrl+c":
		stopDockerLogs()
		return tea.Quit
	case "tab":
		if m.focus == FocusConfig {
			m.focus = FocusLogs
		} else {
			m.focus = FocusConfig
		}
	case "up", "k":
		if m.focus == FocusConfig {
			m.moveCursor(-1)
		} else {
			m.viewport.ScrollUp(1)
		}
	case "down", "j":
		if m.focus == FocusConfig {
			m.moveCursor(1)
		} else {
			m.viewport.ScrollDown(1)
		}
	case "pgup":
		m.viewport.PageUp()
	case "pgdown":
		m.viewport.PageDown()
	case "home":
		m.viewport.GotoTop()
	case "end":
		m.viewport.GotoBottom()
	case "enter":
		if m.focus == FocusConfig {
			m.beginEdit()
		}
	case "1":
		return m.changeLogService(LogAsterisk)
	case "2":
		return m.changeLogService(LogBot)
	case "3":
		return m.changeLogService(LogAll)
	case "s":
		if err := saveEnv(envPath(), fieldsMap(m.fields)); err != nil {
			m.statusMessage = "Save failed: " + err.Error()
		} else {
			m.statusMessage = "Environment saved"
		}
	case "t":
		return func() tea.Msg {
			status, ok := testARI(fieldsMap(m.fields))
			return dockerCommandMsg{Status: status, Err: statusError(ok)}
		}
	case "r":
		return runDockerCommand("Restarting Docker services...", "compose", "restart")
	case "o":
		return runDockerCommand("Starting Docker services...", "compose", "up", "-d")
	case "x":
		return runDockerCommand("Stopping Docker services...", "compose", "stop")
	case "c":
		m.logs = nil
		m.viewport.SetContent("Docker logs cleared")
	case "l":
		m.focus = FocusLogs
	case "h":
		return openShell()
	case "esc":
		m.focus = FocusConfig
	}
	return nil
}
func (m *Model) moveCursor(d int) {
	m.cursor += d
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.fields) {
		m.cursor = len(m.fields) - 1
	}
}
func (m *Model) beginEdit() {
	f := m.fields[m.cursor]
	m.input.SetValue(f.Value)
	m.input.EchoMode = textinput.EchoNormal
	if f.Secret {
		m.input.EchoMode = textinput.EchoPassword
	}
	m.input.Focus()
	m.editing = true
}
func (m *Model) saveCurrentInput() { m.fields[m.cursor].Value = m.input.Value() }
func (m *Model) changeLogService(s LogService) tea.Cmd {
	m.logService = s
	m.logSession++
	stopDockerLogs()
	m.logs = nil
	return startDockerLogs(m.program, string(s), m.logSession)
}
func (m *Model) startLogs() tea.Cmd {
	m.logSession++
	stopDockerLogs()
	m.logs = nil
	return startDockerLogs(m.program, string(m.logService), m.logSession)
}
func (m *Model) addLog(line string) {
	if line == "" {
		return
	}
	m.logs = append(m.logs, line)
	if len(m.logs) > m.maxLogs {
		m.logs = m.logs[len(m.logs)-m.maxLogs:]
	}
	m.viewport.SetContent(strings.Join(m.logs, "\n"))
	m.viewport.GotoBottom()
}
func displayLogService(s LogService) string {
	switch s {
	case LogAsterisk:
		return "Asterisk"
	case LogBot:
		return "Bot"
	default:
		return "All services"
	}
}
func (m *Model) updateLayout() {
	m.viewport.Width = max(20, m.width/2-6)
	m.viewport.Height = max(5, m.height-8)
}
func (m *Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}
	config := m.renderConfig()
	logs := m.renderLogs()
	header := m.renderHeader()
	footer := m.renderFooter()
	if m.height < 32 {
		if m.focus == FocusLogs {
			return lipgloss.JoinVertical(lipgloss.Left, header, logs, footer)
		}
		return lipgloss.JoinVertical(lipgloss.Left, header, config, footer)
	}
	body := lipgloss.JoinVertical(lipgloss.Left, config, logs)
	if m.width >= 110 {
		body = lipgloss.JoinHorizontal(lipgloss.Top, config, logs)
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}
func (m *Model) renderHeader() string {
	return titleStyle.Render("SSD VOIP TUI") + "  " + statusStyle.Render(fmt.Sprintf("Asterisk: %s   Bot: %s", formatServiceStatus(m.asteriskStatus), formatServiceStatus(m.botStatus)))
}
func (m *Model) renderConfig() string {
	var b strings.Builder
	b.WriteString("Configuration\n\n")
	for i, f := range m.fields {
		p := "  "
		if i == m.cursor && m.focus == FocusConfig {
			p = "> "
		}
		v := displayValue(f.Value, f.Secret)
		if m.editing && i == m.cursor {
			v = m.input.View()
		}
		b.WriteString(fmt.Sprintf("%s%-22s %s\n", p, f.Label, v))
	}
	return panelFocusedStyle.Width(panelWidth(m.width)).Render(b.String())
}
func (m *Model) renderLogs() string {
	height := max(5, m.height-8)
	return panelStyle.Width(logPanelWidth(m.width)).Height(height).Render("Logs [" + displayLogService(m.logService) + "]\n\n" + m.viewport.View())
}
func (m *Model) renderFooter() string {
	return helpStyle.Render("Up/Down navigate  Enter edit  Tab focus  1/2/3 logs  S save  T ARI  R restart  O start  X stop  H shell  Q quit\n" + m.statusMessage)
}
func panelWidth(n int) int {
	if n < 110 {
		return max(20, n-6)
	}
	return n/2 - 4
}
func logPanelWidth(n int) int { return panelWidth(n) }
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
