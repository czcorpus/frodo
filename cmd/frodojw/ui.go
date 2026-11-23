// Copyright 2026 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2026 Institute of the Czech National Corpus,
//                Faculty of Arts, Charles University
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"frodo/jobs"
)

const timeLayout = "2006-01-02 15:04:05"

type viewMode int

const (
	viewList viewMode = iota
	viewDetail
)

// ---- messages

type tickMsg time.Time

type listMsg struct {
	jobs jobs.JobInfoListCompact
	util *utilization
	err  error
}

type detailMsg struct {
	jobID   string
	content string
	err     error
}

// ---- styles

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color("25")).Padding(0, 1)
	infoStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	errStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	helpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	keyStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Bold(true)
	borderStyle = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("25"))
)

// ---- model

type model struct {
	client   *apiClient
	interval time.Duration

	mode   viewMode
	width  int
	height int

	// list view
	table          table.Model
	jobs           jobs.JobInfoListCompact
	util           *utilization
	unfinishedOnly bool

	// detail view
	detailJobID string
	detail      viewport.Model
	detailReady bool

	inFlight   bool
	lastUpdate time.Time
	err        error
}

func newModel(client *apiClient, interval time.Duration) model {
	t := table.New(
		table.WithColumns(columns(100)),
		table.WithFocused(true),
	)
	st := table.DefaultStyles()
	st.Header = st.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("25")).
		BorderBottom(true).
		Bold(true)
	st.Selected = st.Selected.
		Foreground(lipgloss.Color("0")).
		Background(lipgloss.Color("14")).
		Bold(false)
	t.SetStyles(st)

	return model{
		client:   client,
		interval: interval,
		table:    t,
		detail:   viewport.New(0, 0),
	}
}

// noColors tells whether the output is monochrome. In such case, styles
// cannot highlight the selected row so we use an explicit marker column.
func noColors() bool {
	return lipgloss.ColorProfile() == termenv.Ascii
}

const cursorMarker = "▶"

func columns(width int) []table.Column {
	// fixed widths; each of the 7 columns gets also 2 chars of padding
	const (
		idW     = 8
		typeW   = 24
		statusW = 10
		startW  = 19
		durW    = 10
		padding = 7 * 2
	)
	corpusW := width - idW - typeW - statusW - 2*startW - durW - padding
	if corpusW < 10 {
		corpusW = 10
	}
	ans := []table.Column{
		{Title: "ID", Width: idW},
		{Title: "Type", Width: typeW},
		{Title: "Corpus", Width: corpusW},
		{Title: "Status", Width: statusW},
		{Title: "Started", Width: startW},
		{Title: "Updated", Width: startW},
		{Title: "Duration", Width: durW},
	}
	if noColors() {
		ans[2].Width = max(ans[2].Width-3, 10)
		ans = append([]table.Column{{Title: "", Width: 1}}, ans...)
	}
	return ans
}

// markCursor updates the marker column (if used) according
// to the current cursor position
func (m *model) markCursor() {
	if !noColors() {
		return
	}
	rows := m.table.Rows()
	for i, row := range rows {
		if i == m.table.Cursor() {
			row[0] = cursorMarker

		} else {
			row[0] = " "
		}
	}
	m.table.SetRows(rows)
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.fetchList(), tick(m.interval))
}

func tick(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m model) fetchList() tea.Cmd {
	client := m.client
	unfinishedOnly := m.unfinishedOnly
	return func() tea.Msg {
		lst, err := client.jobList(unfinishedOnly)
		if err != nil {
			return listMsg{err: err}
		}
		ans := listMsg{jobs: lst}
		// utilization is just an extra, do not fail the whole view on it
		if u, err := client.utilization(); err == nil {
			ans.util = &u
		}
		return ans
	}
}

func (m model) fetchDetail(jobID string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		content, err := client.jobDetail(jobID)
		return detailMsg{jobID: jobID, content: content, err: err}
	}
}

// refresh fetches data for the currently active view only
func (m *model) refresh() tea.Cmd {
	if m.inFlight {
		return nil
	}
	m.inFlight = true
	if m.mode == viewDetail {
		return m.fetchDetail(m.detailJobID)
	}
	return m.fetchList()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	logInput(msg)
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		return m, nil

	case tickMsg:
		return m, tea.Batch(m.refresh(), tick(m.interval))

	case listMsg:
		m.inFlight = false
		m.err = msg.err
		if msg.err == nil {
			m.lastUpdate = time.Now()
			m.util = msg.util
			m.setJobs(msg.jobs)
		}
		if m.mode != viewList { // view changed while loading; load the right data now
			return m, m.refresh()
		}
		return m, nil

	case detailMsg:
		m.inFlight = false
		if m.mode != viewDetail || msg.jobID != m.detailJobID {
			return m, m.refresh()
		}
		m.err = msg.err
		if msg.err == nil {
			m.lastUpdate = time.Now()
			m.detail.SetContent(msg.content)
			m.detailReady = true
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "f10":
			return m, tea.Quit
		case "r", "f5":
			m.inFlight = false // allow forced refresh
			return m, m.refresh()
		}
		if m.mode == viewList {
			return m.updateList(msg)
		}
		return m.updateDetail(msg)
	}
	return m, nil
}

func (m model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		idx := m.table.Cursor()
		if idx >= 0 && idx < len(m.jobs) {
			m.mode = viewDetail
			m.detailJobID = m.jobs[idx].ID
			m.detailReady = false
			m.detail.SetContent("")
			m.detail.GotoTop()
			m.err = nil
			m.inFlight = false
			return m, m.refresh()
		}
		return m, nil
	case "u":
		m.unfinishedOnly = !m.unfinishedOnly
		m.inFlight = false
		return m, m.refresh()
	}
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	m.markCursor()
	return m, cmd
}

func (m model) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "backspace", "left", "h":
		m.mode = viewList
		m.err = nil
		m.inFlight = false
		return m, m.refresh()
	}
	var cmd tea.Cmd
	m.detail, cmd = m.detail.Update(msg)
	return m, cmd
}

// layout recalculates sizes of the components according to the terminal size
func (m *model) layout() {
	if m.width == 0 || m.height == 0 {
		return
	}
	innerW := m.width - 2 // border
	headerH := lipgloss.Height(m.headerView())
	footerH := lipgloss.Height(m.footerView())
	innerH := m.height - headerH - footerH - 2 // border
	if innerH < 3 {
		innerH = 3
	}
	m.table.SetColumns(columns(innerW))
	m.table.SetWidth(innerW)
	m.table.SetHeight(innerH)
	m.detail.Width = innerW
	m.detail.Height = innerH
}

func jobStatus(j *jobs.JobInfoCompact) string {
	if !j.Finished {
		return "● running"
	}
	if j.OK {
		return "✓ ok"
	}
	return "✗ failed"
}

func fmtTime(t jobs.JSONTime) string {
	if t.IsZero() {
		return "-"
	}
	return time.Time(t).Local().Format(timeLayout)
}

func fmtDuration(d time.Duration) string {
	if d < 0 {
		return "-"
	}
	d = d.Round(time.Second)
	h := int(d.Hours())
	mi := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm%02ds", h, mi, s)
	}
	if mi > 0 {
		return fmt.Sprintf("%dm%02ds", mi, s)
	}
	return fmt.Sprintf("%ds", s)
}

func jobDuration(j *jobs.JobInfoCompact) string {
	if j.Start.IsZero() {
		return "-"
	}
	if j.Finished {
		if j.Update.IsZero() {
			return "-"
		}
		return fmtDuration(j.Update.Sub(j.Start))
	}
	return fmtDuration(time.Since(time.Time(j.Start)))
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// setJobs fills the table with new jobs, keeping the cursor
// on the same job (if it still exists).
func (m *model) setJobs(newJobs jobs.JobInfoListCompact) {
	var selectedID string
	if c := m.table.Cursor(); c >= 0 && c < len(m.jobs) {
		selectedID = m.jobs[c].ID
	}
	m.jobs = newJobs
	rows := make([]table.Row, len(m.jobs))
	newCursor := -1
	for i, j := range m.jobs {
		corpus := j.CorpusID
		if j.AliasedCorpusID != "" && j.AliasedCorpusID != j.CorpusID {
			corpus = fmt.Sprintf("%s (%s)", j.AliasedCorpusID, j.CorpusID)
		}
		rows[i] = table.Row{
			shortID(j.ID),
			j.Type,
			corpus,
			jobStatus(j),
			fmtTime(j.Start),
			fmtTime(j.Update),
			jobDuration(j),
		}
		if noColors() {
			rows[i] = append(table.Row{" "}, rows[i]...)
		}
		if j.ID == selectedID {
			newCursor = i
		}
	}
	m.table.SetRows(rows)
	if len(rows) > 0 {
		if newCursor < 0 { // job disappeared (or nothing selected yet)
			newCursor = min(max(m.table.Cursor(), 0), len(rows)-1)
		}
		m.table.SetCursor(newCursor)
	}
	m.markCursor()
}

func (m model) headerView() string {
	title := titleStyle.Render("FRODO Job Watch")
	var info string
	if m.mode == viewDetail {
		info = fmt.Sprintf(" %s  ›  job %s", m.client.baseURL, m.detailJobID)

	} else {
		var running int
		for _, j := range m.jobs {
			if !j.Finished {
				running++
			}
		}
		info = fmt.Sprintf(" %s  ›  %d jobs, %d running", m.client.baseURL, len(m.jobs), running)
		if m.util != nil {
			info += fmt.Sprintf("  │  slots %d/%d, queue %d",
				m.util.CurrentRunningJobs, m.util.MaxNumConcurrentJobs, m.util.JobQueueLength)
		}
		if m.unfinishedOnly {
			info += "  │  filter: unfinished only"
		}
	}
	ans := lipgloss.JoinHorizontal(lipgloss.Top, title, infoStyle.Render(info))
	if m.width > 0 {
		ans = lipgloss.NewStyle().MaxWidth(m.width).Render(ans)
	}
	return ans
}

func helpItem(key, desc string) string {
	return keyStyle.Render(key) + helpStyle.Render(" "+desc)
}

func (m model) footerView() string {
	var items []string
	if m.mode == viewDetail {
		items = []string{
			helpItem("↑/↓/PgUp/PgDn", "scroll"),
			helpItem("Esc/Bksp/←/h", "back"),
			helpItem("r", "refresh"),
			helpItem("q", "quit"),
		}

	} else {
		items = []string{
			helpItem("↑/↓/j/k", "move"),
			helpItem("Enter", "detail"),
			helpItem("u", "toggle unfinished"),
			helpItem("r", "refresh"),
			helpItem("q", "quit"),
		}
	}
	status := helpStyle.Render(fmt.Sprintf("auto-refresh %s", m.interval))
	if !m.lastUpdate.IsZero() {
		status += helpStyle.Render(", last update " + m.lastUpdate.Format("15:04:05"))
	}
	line := strings.Join(items, "  ") + "   " + status
	if m.width > 0 {
		line = lipgloss.NewStyle().MaxWidth(m.width).Render(line)
	}
	if m.err != nil {
		errLine := errStyle.Render("Error: " + m.err.Error())
		if m.width > 0 {
			errLine = lipgloss.NewStyle().MaxWidth(m.width).Render(errLine)
		}
		return errLine + "\n" + line
	}
	return "\n" + line
}

func (m model) View() string {
	if m.width == 0 {
		return "loading..."
	}
	var body string
	if m.mode == viewDetail {
		if m.detailReady {
			body = m.detail.View()

		} else {
			body = lipgloss.Place(m.detail.Width, m.detail.Height, lipgloss.Center, lipgloss.Center, "loading...")
		}

	} else if len(m.jobs) == 0 && m.lastUpdate.IsZero() {
		body = lipgloss.Place(m.table.Width(), m.table.Height(), lipgloss.Center, lipgloss.Center, "loading...")

	} else if len(m.jobs) == 0 {
		body = lipgloss.Place(m.table.Width(), m.table.Height(), lipgloss.Center, lipgloss.Center, "no jobs")

	} else {
		body = m.table.View()
	}
	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.headerView(),
		borderStyle.Render(body),
		m.footerView(),
	)
}
