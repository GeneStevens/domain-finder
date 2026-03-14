package termui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/genestevens/domain-finder/internal/match"
	"github.com/genestevens/domain-finder/internal/report"
)

type startMsg struct {
	total  int
	filter report.FilterMode
}

type activeMsg struct {
	index     int
	total     int
	candidate string
}

type statusMsg struct {
	line string
}

type rowMsg struct {
	result match.CandidateResult
}

type noteMsg struct {
	line string
}

type finishMsg struct {
	summary report.Summary
}

type interactiveModel struct {
	zones        []string
	color        bool
	showPartials bool
	candWidth    int
	zoneWidth    int
	statusWidth  int
	total        int
	filter       string
	liveProgress string
	liveChecking string
	results      []string
	notes        []string
	done         bool
	summaryLine  string
	interrupt    func()
}

func newInteractiveModel(zones []string, candidates []string, color, showPartials bool, interrupt func()) interactiveModel {
	candWidth, zoneWidth, statusWidth := consoleColumnWidths(zones, candidates)
	return interactiveModel{
		zones:        append([]string(nil), zones...),
		color:        color,
		showPartials: showPartials,
		candWidth:    candWidth,
		zoneWidth:    zoneWidth,
		statusWidth:  statusWidth,
		interrupt:    interrupt,
	}
}

func (m interactiveModel) Init() tea.Cmd {
	return nil
}

func (m interactiveModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			if m.interrupt != nil {
				m.interrupt()
			}
			m.done = true
			if m.summaryLine == "" {
				m.summaryLine = "Interrupted."
			}
			m.liveProgress = ""
			m.liveChecking = ""
			return m, tea.Quit
		}
	case startMsg:
		m.total = msg.total
		m.filter = string(msg.filter)
	case activeMsg:
		m.liveChecking = fmt.Sprintf("checking: %s... [%d/%d]", truncate(msg.candidate, m.candWidth+4), msg.index, msg.total)
	case statusMsg:
		m.liveProgress = strings.TrimSpace(msg.line)
	case rowMsg:
		if shouldEmitRow(m.showPartials, msg.result) {
			m.results = append(m.results, formatRow(m.candWidth, m.zoneWidth, m.statusWidth, m.color, msg.result))
		}
		m.liveChecking = ""
	case noteMsg:
		m.notes = append(m.notes, msg.line)
		m.liveProgress = ""
		m.liveChecking = ""
	case finishMsg:
		m.summaryLine = fmt.Sprintf("Done: checked %d | emitted %d | strong %d", msg.summary.TotalCandidates, msg.summary.EmittedResults, msg.summary.AbsentInAll)
		m.liveProgress = ""
		m.liveChecking = ""
		m.done = true
		return m, tea.Quit
	}
	return m, nil
}

func (m interactiveModel) View() string {
	lines := []string{
		fmt.Sprintf("Zone files loaded: %s", strings.Join(upperZones(m.zones), ", ")),
		fmt.Sprintf("Searching %d stems | filter: %s", m.total, m.filter),
		"",
		fmt.Sprintf("%-*s  %-*s  %-*s", m.candWidth, "stem", m.zoneWidth, "available_zones", m.statusWidth, "result"),
	}
	lines = append(lines, m.results...)
	lines = append(lines, m.notes...)
	if !m.done {
		if live := strings.TrimSpace(strings.Join(nonEmpty([]string{m.liveProgress, m.liveChecking}), " | ")); live != "" {
			lines = append(lines, live)
		}
	}
	if m.summaryLine != "" {
		lines = append(lines, m.summaryLine)
	}
	return strings.Join(lines, "\n")
}
