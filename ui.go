package main

// The interactive list: pick a mission, confirm, write.

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type uiState int

const (
	statePicking uiState = iota
	stateConfirm
	stateDone
	stateError
)

type model struct {
	pilot    *Pilot
	missions []MissionEntry
	listPath string

	cursor int
	offset int
	state  uiState
	height int

	message string
}

var (
	titleStyle    = lipgloss.NewStyle().Bold(true)
	dimStyle      = lipgloss.NewStyle().Faint(true)
	cursorStyle   = lipgloss.NewStyle().Reverse(true)
	nextStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	doneStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	lockedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
	battleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("13"))
)

func newModel(pilot *Pilot, missions []MissionEntry, listPath string) model {
	m := model{pilot: pilot, missions: missions, listPath: listPath, height: 24}

	// Start the cursor on whichever mission the game would offer next.
	for i, e := range missions {
		if pilot.Mission(e.Index).Completed == 0 {
			m.cursor = i
			break
		}
	}
	m.scrollToCursor()
	return m
}

// nextIndex returns the mission index the game will currently offer, or -1.
func (m model) nextIndex() int {
	for _, e := range m.missions {
		if m.pilot.Mission(e.Index).Completed == 0 {
			return e.Index
		}
	}
	return -1
}

func (m model) visibleRows() int {
	rows := m.height - 8
	if rows < 5 {
		rows = 5
	}
	return rows
}

func (m *model) scrollToCursor() {
	rows := m.visibleRows()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+rows {
		m.offset = m.cursor - rows + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if w, ok := msg.(tea.WindowSizeMsg); ok {
		m.height = w.Height
		m.scrollToCursor()
		return m, nil
	}

	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch m.state {
	case stateDone, stateError:
		switch key.String() {
		case "q", "esc", "enter":
			return m, tea.Quit
		}
		return m, nil

	case stateConfirm:
		switch strings.ToLower(key.String()) {
		case "y":
			target := m.missions[m.cursor].Index
			patched, err := m.pilot.Patch(m.missions, target)
			if err != nil {
				m.state = stateError
				m.message = err.Error()
				return m, nil
			}
			if err := Save(m.pilot.Path, patched); err != nil {
				m.state = stateError
				m.message = err.Error()
				return m, nil
			}
			// Reload so the list reflects what was written.
			if reloaded, err := LoadPilot(m.pilot.Path); err == nil {
				m.pilot = reloaded
			}
			m.state = stateDone
			m.message = fmt.Sprintf("set to mission %d - %s", target, m.missions[m.cursor].Desc)
		case "n", "esc", "q":
			m.state = statePicking
			m.message = ""
		}
		return m, nil

	default:
		switch key.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.scrollToCursor()
			}
		case "down", "j":
			if m.cursor < len(m.missions)-1 {
				m.cursor++
				m.scrollToCursor()
			}
		case "home", "g":
			m.cursor = 0
			m.scrollToCursor()
		case "end", "G":
			m.cursor = len(m.missions) - 1
			m.scrollToCursor()
		case "pgup":
			m.cursor -= m.visibleRows()
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.scrollToCursor()
		case "pgdown":
			m.cursor += m.visibleRows()
			if m.cursor > len(m.missions)-1 {
				m.cursor = len(m.missions) - 1
			}
			m.scrollToCursor()
		case "enter":
			m.state = stateConfirm
		}
		return m, nil
	}
}

func (m model) View() string {
	if m.state == stateDone {
		return "\n  " + lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render("✓ "+m.message) +
			"\n\n" + dimStyle.Render("  backup saved beside the pilot file\n\n  press enter to exit")
	}
	if m.state == stateError {
		return "\n  " + lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("✗ "+m.message) +
			"\n\n  press enter to exit"
	}

	var b strings.Builder

	b.WriteString(titleStyle.Render("  TFTC Reimagined — set next mission"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(fmt.Sprintf("  pilot: %s   (%s)", m.pilot.Name, m.pilot.Path)))
	b.WriteString("\n\n")

	header := fmt.Sprintf("  %-6s %-22s %-44s %s", "INDEX", "BATTLE", "MISSION", "STATUS")
	b.WriteString(dimStyle.Render(header))
	b.WriteString("\n")

	next := m.nextIndex()
	rows := m.visibleRows()
	end := m.offset + rows
	if end > len(m.missions) {
		end = len(m.missions)
	}

	prevBattle := ""
	if m.offset > 0 {
		prevBattle = m.missions[m.offset-1].Battle
	}

	for i := m.offset; i < end; i++ {
		e := m.missions[i]
		pm := m.pilot.Mission(e.Index)

		battle := ""
		if e.Battle != prevBattle {
			battle = e.Battle
		}
		prevBattle = e.Battle

		status := "locked"
		statusStyle := lockedStyle
		switch {
		case e.Index == next:
			status = "next"
			statusStyle = nextStyle
		case pm.Completed > 0:
			status = "done"
			statusStyle = doneStyle
		}

		line := fmt.Sprintf("%-6d %-22s %-44s %s",
			e.Index, trimTo(battle, 22), trimTo(e.Desc, 44), statusStyle.Render(status))

		if i == m.cursor {
			b.WriteString(cursorStyle.Render("> " + line))
		} else {
			b.WriteString("  " + line)
		}
		b.WriteString("\n")
	}

	if m.offset > 0 {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  ↑ %d more above", m.offset)))
		b.WriteString("\n")
	}
	if end < len(m.missions) {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  ↓ %d more below", len(m.missions)-end)))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	if m.state == stateConfirm {
		e := m.missions[m.cursor]
		b.WriteString(selectedStyle.Render(fmt.Sprintf("  Set pilot to mission %d — %s?", e.Index, trimTo(e.Desc, 44))))
		b.WriteString("\n")
		b.WriteString(dimStyle.Render(fmt.Sprintf("  All earlier missions are marked complete so the game offers this one next.")))
		b.WriteString("\n")
		b.WriteString("  " + nextStyle.Render("[y]") + " write    " + lockedStyle.Render("[n]") + " cancel")
		b.WriteString("\n")
	} else {
		b.WriteString(dimStyle.Render("  ↑/↓ move   enter select   g/G top/bottom   q quit"))
		b.WriteString("\n")
	}

	return b.String()
}

func trimTo(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 3 {
		return s[:n]
	}
	return s[:n-3] + "..."
}
