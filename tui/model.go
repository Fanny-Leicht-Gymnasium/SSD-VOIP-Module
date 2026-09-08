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

// Page selects which group of config fields the config panel shows.
type Page int

const (
	PageGeneral Page = iota
	PageAsterisk
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
	page                                     Page
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

	// Log display state.
	logHorizontalOffset int
	logsFullscreen      bool

	// Piper voice select-with-autocomplete state, active while editing
	// the PIPER_MODELS field.
	selecting      bool
	selectFiltered []PiperVoice
	selectCursor   int
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
		if m.selecting {
			switch msg.String() {
			case "esc":
				m.editing = false
				m.selecting = false
				m.input.Blur()
				return nil
			case "enter":
				if len(m.selectFiltered) > 0 {
					m.input.SetValue(m.selectFiltered[m.selectCursor].Name)
				}
				m.saveCurrentInput()
				m.editing = false
				m.selecting = false
				m.input.Blur()
				return nil
			case "up":
				if m.selectCursor > 0 {
					m.selectCursor--
				}
				return nil
			case "down":
				if m.selectCursor < len(m.selectFiltered)-1 {
					m.selectCursor++
				}
				return nil
			}
			var c tea.Cmd
			m.input, c = m.input.Update(msg)
			m.updatePiperFilter()
			return c
		}

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
	case "left":
		if m.focus == FocusLogs {
			m.scrollLogsHorizontal(-1)
			return nil
		}

		if m.focus == FocusConfig && m.page != PageGeneral {
			m.page = PageGeneral
			m.cursor = 0
		}

	case "right":
		if m.focus == FocusLogs {
			m.scrollLogsHorizontal(1)
			return nil
		}

		if m.focus == FocusConfig && m.page != PageAsterisk {
			m.page = PageAsterisk
			m.cursor = 0
		}
	case "ctrl+left":
		if m.focus == FocusLogs {
			m.scrollLogsHorizontal(-20)
		}

	case "ctrl+right":
		if m.focus == FocusLogs {
			m.scrollLogsHorizontal(20)
		}

	case "f":
		if m.focus == FocusLogs {
			m.logsFullscreen = !m.logsFullscreen
			m.resetLogHorizontalScroll()
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
		values := fieldsMap(m.fields)
		if err := saveEnv(envPath(), values); err != nil {
			m.statusMessage = "Save failed: " + err.Error()
		} else if err := writeAsteriskConfig(values); err != nil {
			m.statusMessage = "Saved .env, but Asterisk config failed: " + err.Error()
		} else {
			m.statusMessage = "Environment and Asterisk config saved (press B to fetch the Piper voice if it changed)"
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
	case "b":
		return downloadAndApplyPiperVoice(m.fieldValue("PIPER_MODELS"))
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

func (m *Model) fieldValue(key string) string {
	for _, f := range m.fields {
		if f.Key == key {
			return f.Value
		}
	}
	return ""
}

// visibleFieldIndices returns the indices into m.fields belonging to the
// currently selected page, in display order.
func (m *Model) visibleFieldIndices() []int {
	var indices []int
	for i, f := range m.fields {
		if f.Page == m.page {
			indices = append(indices, i)
		}
	}
	return indices
}

func (m *Model) moveCursor(d int) {
	m.cursor += d
	if m.cursor < 0 {
		m.cursor = 0
	}
	if maxCursor := len(m.visibleFieldIndices()) - 1; m.cursor > maxCursor {
		m.cursor = maxCursor
	}
}
func (m *Model) beginEdit() {
	visible := m.visibleFieldIndices()
	if m.cursor < 0 || m.cursor >= len(visible) {
		return
	}
	f := m.fields[visible[m.cursor]]
	m.input.SetValue(f.Value)
	m.input.EchoMode = textinput.EchoNormal
	if f.Secret {
		m.input.EchoMode = textinput.EchoPassword
	}
	m.input.Focus()
	m.editing = true

	m.selecting = f.Key == "PIPER_MODELS"
	if m.selecting {
		m.selectCursor = 0
		m.updatePiperFilter()
	}
}
func (m *Model) updatePiperFilter() {
	m.selectFiltered = filterPiperVoices(m.input.Value())
	if m.selectCursor >= len(m.selectFiltered) {
		m.selectCursor = max(0, len(m.selectFiltered)-1)
	}
}
func (m *Model) saveCurrentInput() {
	visible := m.visibleFieldIndices()
	if m.cursor >= 0 && m.cursor < len(visible) {
		m.fields[visible[m.cursor]].Value = m.input.Value()
	}
}
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

	m.viewport.SetContent(m.visibleLogContent())
	m.viewport.GotoBottom()
}
func (m *Model) scrollLogsHorizontal(delta int) {
	m.logHorizontalOffset += delta

	if m.logHorizontalOffset < 0 {
		m.logHorizontalOffset = 0
	}

	// Do not allow the offset to move beyond the longest log line.
	maxWidth := 0
	for _, line := range m.logs {
		if w := lipgloss.Width(line); w > maxWidth {
			maxWidth = w
		}
	}

	visibleWidth := max(1, m.viewport.Width)

	maxOffset := max(0, maxWidth-visibleWidth)
	if m.logHorizontalOffset > maxOffset {
		m.logHorizontalOffset = maxOffset
	}
}

func (m *Model) resetLogHorizontalScroll() {
	m.logHorizontalOffset = 0
}

func (m *Model) visibleLogContent() string {
	if len(m.logs) == 0 {
		return "Docker logs will appear here..."
	}

	visibleWidth := max(1, m.viewport.Width)

	lines := make([]string, 0, len(m.logs))

	for _, line := range m.logs {
		lines = append(lines, clipLogLine(line, m.logHorizontalOffset, visibleWidth))
	}

	return strings.Join(lines, "\n")
}

// clipLogLine returns the visible horizontal section of a log line.
func clipLogLine(line string, offset, width int) string {
	if offset <= 0 {
		return truncateLogLine(line, width)
	}

	runes := []rune(line)

	if offset >= len(runes) {
		return ""
	}

	end := minInt(len(runes), offset+width)
	return string(runes[offset:end])
}

func truncateLogLine(line string, width int) string {
	if width <= 0 {
		return ""
	}

	runes := []rune(line)

	if len(runes) <= width {
		return line
	}

	return string(runes[:width])
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func (m *Model) refreshLogViewport() {
	m.viewport.SetContent(m.visibleLogContent())
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

func (m *Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Fullscreen log mode hides the configuration panel and footer.
	if m.logsFullscreen {
		return m.renderLogsFullscreen()
	}

	header := m.renderHeader()
	footer := m.renderFooter()
	config := m.renderConfig()

	overhead := lipgloss.Height(header) + lipgloss.Height(footer)

	var body string

	if m.width >= 110 {
		logsHeight := max(5, m.height-overhead)
		logs := m.renderLogsSized(logPanelWidth(m.width), logsHeight)
		body = lipgloss.JoinHorizontal(lipgloss.Top, config, logs)
	} else {
		logsHeight := max(5, m.height-overhead-lipgloss.Height(config))
		logs := m.renderLogsSized(panelWidth(m.width), logsHeight)
		body = lipgloss.JoinVertical(lipgloss.Left, config, logs)
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}
func (m *Model) renderLogsFullscreen() string {
	width := max(10, m.width)
	height := max(1, m.height-2)

	m.viewport.Width = max(1, width-6)
	m.viewport.Height = max(1, height-2)

	m.refreshLogViewport()

	title := "Logs [" + displayLogService(m.logService) + "]"

	if m.logHorizontalOffset > 0 {
		title += fmt.Sprintf("  ← offset %d", m.logHorizontalOffset)
	}

	content := title + "\n\n" + m.viewport.View()

	return panelFocusedStyle.
		Width(width).
		Height(height).
		Render(content)
}
func (m *Model) renderHeader() string {
	version, branch, repo := GetVersionInfo()
	return titleStyle.Render("SSD VOIP TUI") + "  " + statusStyle.Render(fmt.Sprintf("Asterisk: %s", formatServiceStatus(m.asteriskStatus))) + statusStyle.Render(fmt.Sprintf("   Bot: %s", formatServiceStatus(m.botStatus))) + "  " + versionStyle.Render(fmt.Sprintf("Version: %s Branch: %s Repo: %s", version, branch, repo))
}
func (m *Model) renderPageTabs() string {
	general := tabStyle.Render("General")
	asterisk := tabStyle.Render("Asterisk")
	if m.page == PageGeneral {
		general = tabActiveStyle.Render("General")
	} else {
		asterisk = tabActiveStyle.Render("Asterisk")
	}
	return general + " " + asterisk
}
func (m *Model) renderConfig() string {
	var b strings.Builder
	b.WriteString(m.renderPageTabs())
	b.WriteString("\n\n")

	title := "General Configuration"
	if m.page == PageAsterisk {
		title = "Asterisk Configuration"
	}
	b.WriteString(title)
	b.WriteString("\n\n")

	visible := m.visibleFieldIndices()
	for pos, idx := range visible {
		f := m.fields[idx]
		p := "  "
		if pos == m.cursor && m.focus == FocusConfig {
			p = "> "
		}
		v := displayValue(f.Value, f.Secret)
		if m.editing && pos == m.cursor {
			v = m.input.View()
		}
		b.WriteString(fmt.Sprintf("%s%-22s %s\n", p, f.Label, v))
	}

	if m.editing && m.selecting {
		b.WriteString("\n")
		b.WriteString("Matching Piper voices (type to filter, ↑/↓ choose, Enter select):\n")

		if len(m.selectFiltered) == 0 {
			b.WriteString("  (no match — Enter keeps typed value)\n")
		}

		limit := len(m.selectFiltered)
		if limit > 8 {
			limit = 8
		}
		for i := 0; i < limit; i++ {
			v := m.selectFiltered[i]
			line := fmt.Sprintf("  %-32s %s", v.Name, v.Description)
			if i == m.selectCursor {
				line = selectedStyle.Render(fmt.Sprintf("› %-32s %s", v.Name, v.Description))
			}
			b.WriteString(line)
			b.WriteString("\n")
		}
		if len(m.selectFiltered) > limit {
			b.WriteString(fmt.Sprintf("  ...and %d more, keep typing to narrow\n", len(m.selectFiltered)-limit))
		}
	}

	style := panelStyle
	if m.focus == FocusConfig {
		style = panelFocusedStyle
	}
	return style.Width(panelWidth(m.width)).Render(b.String())
}

func (m *Model) renderLogsSized(width, height int) string {
	innerHeight := height - 4
	viewportHeight := innerHeight - 2

	if viewportHeight < 1 {
		viewportHeight = 1
	}

	m.viewport.Width = max(10, width-6)
	m.viewport.Height = viewportHeight

	m.refreshLogViewport()

	style := panelStyle

	if m.focus == FocusLogs {
		style = panelFocusedStyle
	}

	title := "Logs [" + displayLogService(m.logService) + "]"

	if m.logHorizontalOffset > 0 {
		title += fmt.Sprintf("  ← offset %d", m.logHorizontalOffset)
	}

	return style.Width(width).Render(
		title + "\n\n" + m.viewport.View(),
	)
}
func (m *Model) renderFooter() string {
	return helpStyle.Render(
		"Up/Down navigate  Left/Right page/scroll  Enter edit  Tab focus  F fullscreen  " +
			"1/2/3 logs  S save  T ARI  R restart  O start  B fetch voice  X stop  H shell  Q quit\n" +
			m.statusMessage,
	)
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
