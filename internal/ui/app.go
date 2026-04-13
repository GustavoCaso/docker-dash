package ui

import (
	"context"
	"log"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/config"
	"github.com/GustavoCaso/docker-dash/internal/ui/components/confirmation"
	"github.com/GustavoCaso/docker-dash/internal/ui/components/form"
	"github.com/GustavoCaso/docker-dash/internal/ui/components/header"
	"github.com/GustavoCaso/docker-dash/internal/ui/components/statusbar"
	"github.com/GustavoCaso/docker-dash/internal/ui/helper"
	"github.com/GustavoCaso/docker-dash/internal/ui/keys"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
	"github.com/GustavoCaso/docker-dash/internal/ui/sections"
	"github.com/GustavoCaso/docker-dash/internal/ui/sections/compose"
	"github.com/GustavoCaso/docker-dash/internal/ui/sections/containers"
	"github.com/GustavoCaso/docker-dash/internal/ui/sections/images"
	"github.com/GustavoCaso/docker-dash/internal/ui/sections/networks"
	"github.com/GustavoCaso/docker-dash/internal/ui/sections/volumes"
)

type bannerType int

const (
	bannerNone bannerType = iota
	bannerSuccess
	bannerError
)

const (
	bannerTimeoutSecs  = 3 // seconds before banner auto-clears
	bannerOverlayLines = 2 // lines from bottom for banner overlay position
	spinnerOverlayLine = 2 // lines from bottom for spinner overlay position
)

// clearBannerMsg is sent to clear the banner after a timeout.
type clearBannerMsg struct{}

// autoRefreshMsg is sent periodically to refresh all views when a refresh interval is configured.
type autoRefreshMsg struct{}

var (
	bannerSuccessStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("229")).
				Background(lipgloss.Color("34")).
				Padding(0, 1)
	bannerErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("229")).
				Background(lipgloss.Color("196")).
				Padding(0, 1)
)

type model struct {
	cfg              *config.Config
	client           client.Client
	header           *header.Header
	containerSection sections.Section
	imageSection     sections.Section
	volumeSection    sections.Section
	networkSection   sections.Section
	composeSection   sections.Section
	statusBar        *statusbar.StatusBar
	keys             *keys.KeyMap
	imagesKeys       *keys.ViewKeyMap
	containerKeys    *keys.ViewKeyMap
	volumeKeys       *keys.ViewKeyMap
	networkKeys      *keys.ViewKeyMap
	composeKeys      *keys.ViewKeyMap
	activeKeys       *keys.ViewKeyMap
	width            int
	height           int
	bannerMsg        string
	bannerKind       bannerType
	spinner          spinner.Model
	spinnerRequests  map[string]spinnerRequest
	spinnerSequence  uint64
	initErr          string
	confirmation     confirmation.Model
	pendingCmd       tea.Cmd
	showConfirmation bool
	showForm         bool
	formModel        *form.Model
	refreshInterval  time.Duration
}

type spinnerRequest struct {
	Text  string
	Scope message.SpinnerScope
	Seq   uint64
}

func InitialModel(ctx context.Context, version string, cfg *config.Config, client client.Client) tea.Model {
	containersList, containersErr := client.Containers().List(ctx)
	imagesList, imagesErr := client.Images().List(ctx)
	volumesList, volumesErr := client.Volumes().List(ctx)
	networksList, networksErr := client.Networks().List(ctx)
	composeList, composeErr := client.Compose().List(ctx)

	var initErr string
	for _, err := range []error{containersErr, imagesErr, volumesErr, networksErr, composeErr} {
		if err != nil {
			initErr = "Failed to load data: " + err.Error()
			break
		}
	}

	var refreshInterval time.Duration
	if cfg.Refresh.Interval != "" {
		d, err := time.ParseDuration(cfg.Refresh.Interval)
		if err != nil {
			initErr = "Invalid refresh interval: " + err.Error()
		} else {
			refreshInterval = d
		}
	}

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return &model{
		cfg:              cfg,
		client:           client,
		keys:             keys.Keys,
		imagesKeys:       keys.Keys.ImageKeyMap(),
		containerKeys:    keys.Keys.ContainerKeyMap(),
		volumeKeys:       keys.Keys.VolumeKeyMap(),
		networkKeys:      keys.Keys.NetworkKeyMap(),
		composeKeys:      keys.Keys.ComposeKeyMap(),
		header:           header.New(version),
		containerSection: containers.New(ctx, containersList, client.Containers()),
		imageSection:     images.New(ctx, imagesList, client, cfg.UpdateCheck),
		volumeSection:    volumes.New(ctx, volumesList, client.Volumes()),
		networkSection:   networks.New(ctx, networksList, client.Networks()),
		composeSection:   compose.New(ctx, composeList, client.Compose()),
		statusBar:        statusbar.New(),
		spinner:          sp,
		spinnerRequests:  make(map[string]spinnerRequest),
		initErr:          initErr,
		confirmation:     confirmation.New(),
		refreshInterval:  refreshInterval,
	}
}

func (m *model) Init() tea.Cmd {
	if m.initErr != "" {
		return func() tea.Msg {
			return message.ShowBannerMsg{Message: m.initErr, IsError: true}
		}
	}

	cmds := []tea.Cmd{
		m.imageSection.Init(),
		m.containerSection.Init(),
		m.volumeSection.Init(),
		m.networkSection.Init(),
		m.composeSection.Init(),
	}

	if m.refreshInterval > 0 {
		cmds = append(cmds, tea.Tick(m.refreshInterval, func(_ time.Time) tea.Msg {
			return autoRefreshMsg{}
		}))
	}

	return tea.Batch(cmds...)
}

func (m *model) handleFormUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok && km.String() == "esc" {
		m.showForm = false
		m.formModel = nil
		return m, nil
	}
	updatedForm, cmd := m.formModel.Update(msg)
	if f, ok := updatedForm.(*form.Model); ok {
		m.formModel = f
	}
	if m.formModel.State() == huh.StateCompleted {
		m.showForm = false
		m.formModel = nil
	}
	return m, cmd
}

func (m *model) handleConfirmationUpdate(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil, false
	}
	switch km.String() {
	case "y":
		cmd := m.pendingCmd
		m.showConfirmation = false
		m.pendingCmd = nil
		return m, cmd, true
	case "n", "esc":
		m.showConfirmation = false
		m.pendingCmd = nil
		return m, nil, true
	}
	// Swallow all other keys while modal is visible
	return m, nil, true
}

//nolint:gocyclo // this is the main the complexity is acceptable
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	if spinnerCmd := m.updateSpinner(msg); spinnerCmd != nil {
		cmds = append(cmds, spinnerCmd)
	}

	if m.showForm {
		if _, ok := msg.(tea.WindowSizeMsg); !ok {
			return m.handleFormUpdate(msg)
		}
	}

	if m.showConfirmation {
		model, cmd, handled := m.handleConfirmationUpdate(msg)
		if handled {
			return model, cmd
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		log.Printf("[app] WindowSizeMsg: width=%d height=%d", msg.Width, msg.Height)
		m.width = msg.Width
		m.height = msg.Height

		// Reserve space for status bar
		statusBarHeight := 1
		if m.statusBar.IsFullView() {
			statusBarHeight = lipgloss.Height(m.statusBar.View())
		}

		// Reserve space for header
		m.header.SetWidth(msg.Width)
		headerHeight := lipgloss.Height(m.header.View())

		contentHeight := msg.Height - statusBarHeight - headerHeight

		m.containerSection.SetSize(msg.Width, contentHeight)
		m.imageSection.SetSize(msg.Width, contentHeight)
		m.volumeSection.SetSize(msg.Width, contentHeight)
		m.networkSection.SetSize(msg.Width, contentHeight)
		m.composeSection.SetSize(msg.Width, contentHeight)
		m.statusBar.SetSize(msg.Width, statusBarHeight)

	case message.ShowConfirmationMsg:
		log.Printf("[app] ShowConfirmationMsg: title=%q", msg.Title)
		m.confirmation.Init(msg.Title, msg.Body)
		m.pendingCmd = msg.OnConfirm
		m.showConfirmation = true
		return m, tea.Batch(cmds...)

	case message.ShowFormMsg:
		log.Print("[app] ShowFormMsg")
		m.formModel = msg.Form
		m.showForm = true
		cmds = append(cmds, m.formModel.Init())
		return m, tea.Batch(cmds...)

	case message.ShowBannerMsg:
		log.Printf("[app] ShowBannerMsg: message=%q", msg.Message)
		m.bannerMsg = msg.Message
		if msg.IsError {
			m.bannerKind = bannerError
		} else {
			m.bannerKind = bannerSuccess
		}

		clearTimeout := msg.ClearTimeout
		if clearTimeout <= 0 {
			clearTimeout = bannerTimeoutSecs * time.Second
		}

		cmds = append(cmds, tea.Tick(clearTimeout, func(_ time.Time) tea.Msg {
			return clearBannerMsg{}
		}))
		return m, tea.Batch(cmds...)

	case message.ShowSpinnerMsg:
		log.Printf("[app] ShowSpinnerMsg: id=%q text=%q", msg.ID, msg.Text)
		if cmd := m.showSpinner(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)

	case message.CancelSpinnerMsg:
		log.Printf("[app] CancelSpinnerMsg: id=%q", msg.ID)
		m.cancelSpinner(msg.ID)
		return m, tea.Batch(cmds...)

	case message.BubbleUpMsg:
		log.Printf("[app] BubbleUpMsg: key=%q", msg.KeyMsg.String())
		if msg.OnlyActive {
			model, cmd := m.forwardMessageToActive(msg.KeyMsg)
			cmds = append(cmds, cmd)
			return model, tea.Batch(cmds...)
		}
		model, cmd := m.forwardMessageToAll(msg.KeyMsg)
		cmds = append(cmds, cmd)
		return model, tea.Batch(cmds...)

	case clearBannerMsg:
		log.Printf("[app] clearBannerMsg")
		m.bannerMsg = ""
		m.bannerKind = bannerNone
		return m, tea.Batch(cmds...)

	case autoRefreshMsg:
		log.Printf("[app] autoRefreshMsg")
		_, cmd := m.forwardMessageToAll(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
		cmds = append(cmds, cmd, tea.Tick(m.refreshInterval, func(_ time.Time) tea.Msg {
			return autoRefreshMsg{}
		}))
		return m, tea.Batch(cmds...)

	case message.AddContextualKeyBindingsMsg:
		log.Printf("[app] AddContextualKeyBindingsMsg")
		if m.activeKeys != nil {
			m.activeKeys.ToggleContextual(msg.Bindings)
		}

		return m, tea.Batch(cmds...)

	case message.ClearContextualKeyBindingsMsg:
		log.Printf("[app] ClearContextualKeyBindingsMsg")
		if m.activeKeys != nil {
			m.activeKeys.DisableContextual()
		}
		return m, tea.Batch(cmds...)

	case tea.KeyMsg:
		log.Printf("[app] KeyMsg: key=%q", msg.String())
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Left):
			// section := m.activeSection()
			m.header.MoveLeft()
			return m, tea.Batch(cmds...)
		case key.Matches(msg, m.keys.Right):
			// section := m.activeSection()
			m.header.MoveRight()
			return m, tea.Batch(cmds...)
		case key.Matches(msg, m.keys.Refresh):
			model, cmd := m.forwardMessageToActive(msg)
			cmds = append(cmds, cmd)
			return model, tea.Batch(cmds...)
		case key.Matches(msg, m.keys.RefreshAll):
			model, cmd := m.forwardMessageToAll(tea.KeyMsg{
				Type:  tea.KeyRunes,
				Runes: []rune{'r'},
			})
			cmds = append(cmds, cmd)
			return model, tea.Batch(cmds...)
		case key.Matches(msg, m.keys.Help):
			m.statusBar.ToggleFullView()
			cmds = append(cmds, func() tea.Msg {
				return tea.WindowSizeMsg{
					Width:  m.width,
					Height: m.height,
				}
			})
			return m, tea.Batch(cmds...)
		}
	}

	// Forward key messages to focused component only
	if _, ok := msg.(tea.KeyMsg); ok {
		model, cmd := m.forwardMessageToActive(msg)
		cmds = append(cmds, cmd)
		return model, tea.Batch(cmds...)
	}

	// Forward other messages to all components
	model, cmd := m.forwardMessageToAll(msg)
	cmds = append(cmds, cmd)
	return model, tea.Batch(cmds...)
}

func (m *model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	if m.showForm {
		return lipgloss.Place(
			m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			m.formModel.View(),
		)
	}

	if m.showConfirmation {
		return lipgloss.Place(
			m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			m.confirmation.View(),
		)
	}

	var listView string
	var listKeyMap *keys.ViewKeyMap

	switch m.header.ActiveView() {
	case header.ViewContainers:
		listView = m.containerSection.View()
		listKeyMap = m.containerKeys
	case header.ViewImages:
		listView = m.imageSection.View()
		listKeyMap = m.imagesKeys
	case header.ViewVolumes:
		listView = m.volumeSection.View()
		listKeyMap = m.volumeKeys
	case header.ViewNetworks:
		listView = m.networkSection.View()
		listKeyMap = m.networkKeys
	case header.ViewCompose:
		listView = m.composeSection.View()
		listKeyMap = m.composeKeys
	}

	m.activeKeys = listKeyMap
	m.statusBar.SetKeyMap(listKeyMap)

	content := lipgloss.JoinVertical(lipgloss.Left, m.header.View(), listView)

	if m.bannerMsg != "" {
		var style lipgloss.Style
		if m.bannerKind == bannerError {
			style = bannerErrorStyle
		} else {
			style = bannerSuccessStyle
		}
		bannerText := style.Render(m.bannerMsg)
		content = helper.OverlayBottomRight(bannerOverlayLines, content, bannerText, m.width)
	}

	if spinnerText := m.activeSpinnerText(); spinnerText != "" {
		offset := spinnerOverlayLine
		if m.bannerMsg != "" {
			offset++
		}
		content = helper.OverlayBottomRight(offset, content, m.spinner.View()+" "+spinnerText, m.width)
	}

	return lipgloss.JoinVertical(lipgloss.Left, content, m.statusBar.View())
}

func (m *model) updateSpinner(msg tea.Msg) tea.Cmd {
	if len(m.spinnerRequests) == 0 {
		return nil
	}
	if _, ok := msg.(spinner.TickMsg); !ok {
		return nil
	}

	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return cmd
}

func (m *model) showSpinner(msg message.ShowSpinnerMsg) tea.Cmd {
	before := len(m.spinnerRequests)
	text := msg.Text
	if text == "" {
		text = "Loading..."
	}
	m.spinnerSequence++
	m.spinnerRequests[msg.ID] = spinnerRequest{
		Text:  text,
		Scope: msg.Scope,
		Seq:   m.spinnerSequence,
	}
	if before == 0 {
		return m.spinner.Tick
	}
	return nil
}

func (m *model) cancelSpinner(id string) {
	delete(m.spinnerRequests, id)
}

func (m *model) activeSpinnerText() string {
	scope := m.activeSpinnerScope()
	if text, ok := m.spinnerTextForScope(scope); ok {
		return text
	}
	scope.Panel = ""
	if text, ok := m.spinnerTextForScope(scope); ok {
		return text
	}
	return ""
}

func (m *model) activeSpinnerScope() message.SpinnerScope {
	return message.SpinnerScope{
		Section: m.activeSectionName(),
		Panel:   m.activeSection().ActivePanelName(),
	}
}

func (m *model) spinnerTextForScope(scope message.SpinnerScope) (string, bool) {
	var (
		text    string
		bestSeq uint64
		found   bool
	)
	for _, request := range m.spinnerRequests {
		if request.Scope != scope {
			continue
		}
		if !found || request.Seq > bestSeq {
			text = request.Text
			bestSeq = request.Seq
			found = true
		}
	}
	return text, found
}

func (m *model) activeSectionName() string {
	switch m.header.ActiveView() {
	case header.ViewContainers:
		return string(sections.ContainersSection)
	case header.ViewImages:
		return string(sections.ImagesSection)
	case header.ViewVolumes:
		return string(sections.VolumesSection)
	case header.ViewNetworks:
		return string(sections.NetworksSection)
	case header.ViewCompose:
		return string(sections.ComposeSection)
	default:
		return ""
	}
}

func (m *model) forwardMessageToActive(msg tea.Msg) (tea.Model, tea.Cmd) {
	section := m.activeSection()
	cmd := section.Update(msg)

	return m, cmd
}

func (m *model) forwardMessageToAll(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmds := []tea.Cmd{
		m.containerSection.Update(msg),
		m.imageSection.Update(msg),
		m.volumeSection.Update(msg),
		m.networkSection.Update(msg),
		m.composeSection.Update(msg),
	}
	return m, tea.Batch(cmds...)
}

func (m *model) activeSection() sections.Section {
	var section sections.Section
	switch m.header.ActiveView() {
	case header.ViewContainers:
		section = m.containerSection
	case header.ViewImages:
		section = m.imageSection
	case header.ViewVolumes:
		section = m.volumeSection
	case header.ViewNetworks:
		section = m.networkSection
	case header.ViewCompose:
		section = m.composeSection
	}
	return section
}
