package tui

import (
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/devports/devpt/pkg/health"
	"github.com/devports/devpt/pkg/models"
)

type viewMode int
type viewFocus int
type confirmKind int
type modalKind int

const (
	viewModeTable viewMode = iota
	viewModeLogs
	viewModeLogsDebug
	viewModeCommand
	viewModeSearch
)

const (
	focusRunning viewFocus = iota
	focusManaged
)

const (
	confirmStopPID confirmKind = iota
	confirmRemoveService
	confirmSudoKill
	confirmGroupStop
	confirmGroupRestart
	confirmGroupStart
	confirmGroupRemove
)

const (
	modalHelp modalKind = iota + 1
	modalConfirm
)

type confirmState struct {
	kind         confirmKind
	prompt       string
	pid          int
	name         string
	serviceName  string
	namespace    string
	serviceNames []string
	pids         []int
}

type modalState struct {
	kind modalKind
}

type topModel struct {
	app        AppDeps
	servers    []*models.ServerInfo
	width      int
	height     int
	lastUpdate time.Time
	lastInput  time.Time
	err        error

	selected   int
	managedSel int
	focus      viewFocus
	mode       viewMode

	logLines   []string
	logErr     error
	logSvc     *models.ManagedService
	logPID     int
	followLogs bool

	cmdInput    string
	searchQuery string
	cmdStatus   string
	searchInput textinput.Model

	health           map[int]string
	healthDetails    map[int]*health.HealthCheck
	showHealthDetail bool
	healthBusy       bool
	healthLast       time.Time
	healthChk        *health.Checker

	sortBy      sortMode
	sortReverse bool
	lastSortBy  sortMode // track last sorted column for 3-state cycle

	starting map[string]time.Time
	removed  map[string]*models.ManagedService

	modal   *modalState
	confirm *confirmState
	table   processTable

	keys             keyMap
	help             help.Model
	viewport         viewport.Model
	viewportNeedsTop bool
	highlightIndex   int
	highlightMatches []int

	lastClickTime        time.Time
	lastClickY           int
	tableFollowSelection bool

	// Toggle-based visual group selection (g key)
	groupHighlightNamespace *string
}

type tickMsg time.Time

type logMsg struct {
	lines []string
	err   error
}

type healthMsg struct {
	icons   map[int]string
	details map[int]*health.HealthCheck
	err     error
}

func Run(app AppDeps) error {
	model := newTopModel(app)
	p := tea.NewProgram(model)
	_, err := p.Run()
	return err
}

func newTopModel(app AppDeps) *topModel {
	searchInput := textinput.New()
	searchInput.Prompt = ">"
	searchInput.Placeholder = ""
	searchInput.CharLimit = 256
	searchInput.SetVirtualCursor(true)
	searchStyles := textinput.DefaultStyles(false)
	searchStyles.Focused.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	searchStyles.Focused.Text = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	searchStyles.Blurred.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	searchStyles.Blurred.Text = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	searchInput.SetStyles(searchStyles)

	m := &topModel{
		app:                  app,
		lastUpdate:           time.Now(),
		lastInput:            time.Now(),
		mode:                 viewModeTable,
		focus:                focusRunning,
		followLogs:           false,
		health:               make(map[int]string),
		healthDetails:        make(map[int]*health.HealthCheck),
		healthChk:            health.NewChecker(800 * time.Millisecond),
		sortBy:               sortRecent,
		starting:             make(map[string]time.Time),
		removed:              make(map[string]*models.ManagedService),
		keys:                 defaultKeyMap(),
		help:                 help.New(),
		searchInput:          searchInput,
		tableFollowSelection: true,
	}
	if servers, err := app.DiscoverServers(); err == nil {
		m.servers = servers
	}

	m.viewport = viewport.New()
	m.table = newProcessTable()
	m.highlightIndex = 0

	return m
}

func (m topModel) Init() tea.Cmd {
	return tickCmd()
}

func (m *topModel) refresh() {
	if servers, err := m.app.DiscoverServers(); err == nil {
		m.servers = servers
		m.lastUpdate = time.Now()
		if m.selected >= len(m.visibleServers()) && len(m.visibleServers()) > 0 {
			m.selected = len(m.visibleServers()) - 1
		}
		if m.managedSel >= len(m.managedServices()) && len(m.managedServices()) > 0 {
			m.managedSel = len(m.managedServices()) - 1
		}
		for name, at := range m.starting {
			if m.isServiceRunning(name) || time.Since(at) > 45*time.Second {
				delete(m.starting, name)
			}
		}
	} else {
		m.err = err
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}
