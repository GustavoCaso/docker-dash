package networks

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/ui/components/panel"
	"github.com/GustavoCaso/docker-dash/internal/ui/helper"
	"github.com/GustavoCaso/docker-dash/internal/ui/keys"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
	"github.com/GustavoCaso/docker-dash/internal/ui/theme"
)

const listSplitRatio = 0.4

// networksLoadedMsg is sent when networks have been loaded asynchronously.
type networksLoadedMsg struct {
	error error
	items []list.Item
}

// networksPrunedMsg is sent when a network prune completes.
type networksPrunedMsg struct {
	report client.PruneReport
	err    error
}

// networkRemovedMsg is sent when a network deletion completes.
type networkRemovedMsg struct {
	ID    string
	Name  string
	Idx   int
	Error error
}

// networkItem implements list.Item interface.
type networkItem struct {
	network client.Network
}

func (n networkItem) Title() string { return n.network.Name }
func (n networkItem) Description() string {
	var parts []string
	parts = append(parts, n.network.Driver)
	parts = append(parts, n.network.Scope)
	if len(n.network.ConnectedContainers) > 0 {
		inUse := theme.StatusRunningStyle.Render(fmt.Sprintf("● %d connected", len(n.network.ConnectedContainers)))
		parts = append(parts, inUse)
	} else {
		parts = append(parts, "unused")
	}
	return strings.Join(parts, " • ")
}
func (n networkItem) FilterValue() string { return n.network.Name }

// Section wraps bubbles/list for displaying networks.
type Section struct {
	ctx            context.Context
	list           list.Model
	isFilter       bool
	viewport       viewport.Model
	networkService client.NetworkService
	detailsPanel   panel.Panel
	activePanel    panel.Panel
	width, height  int
	loading        bool
	spinner        spinner.Model
}

// New creates a new network section.
func New(ctx context.Context, networks []client.Network, svc client.NetworkService) *Section {
	items := make([]list.Item, len(networks))
	for i, n := range networks {
		items[i] = networkItem{network: n}
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(true)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return &Section{
		ctx:            ctx,
		list:           l,
		viewport:       viewport.New(0, 0),
		networkService: svc,
		detailsPanel:   newDetailsPanel(),
		spinner:        sp,
	}
}

// SetSize sets dimensions.
func (s *Section) SetSize(width, height int) {
	s.width = width
	s.height = height

	listX, listY := theme.ListStyle.GetFrameSize()

	if s.activePanel != nil {
		listWidth := int(float64(width) * listSplitRatio)
		detailWidth := width - listWidth

		s.list.SetSize(listWidth-listX, height-listY)
		s.viewport.Width = detailWidth - listX
		s.viewport.Height = height - listY
		s.activePanel.SetSize(s.viewport.Width, s.viewport.Height)
	} else {
		s.list.SetSize(width-listX, height-listY)
	}
}

// Update handles messages.
func (s *Section) Update(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd

	if s.loading {
		var spinnerCmd tea.Cmd
		s.spinner, spinnerCmd = s.spinner.Update(msg)
		cmds = append(cmds, spinnerCmd)
	}

	switch msg := msg.(type) {
	case networksLoadedMsg:
		s.loading = false
		if msg.error != nil {
			return func() tea.Msg {
				return message.ShowBannerMsg{
					Message: fmt.Sprintf("Error loading networks: %s", msg.error.Error()),
					IsError: true,
				}
			}
		}
		cmd := s.list.SetItems(msg.items)
		cmds = append(cmds, cmd)
		return tea.Batch(cmds...)
	case networksPrunedMsg:
		if msg.err != nil {
			return func() tea.Msg {
				return message.ShowBannerMsg{
					Message: "Error pruning networks: " + msg.err.Error(),
					IsError: true,
				}
			}
		}
		summary := fmt.Sprintf("Pruned %d networks", msg.report.ItemsDeleted)
		return tea.Batch(s.updateNetworksCmd(), func() tea.Msg {
			return message.ShowBannerMsg{Message: summary, IsError: false}
		})
	case networkRemovedMsg:
		if msg.Error != nil {
			return func() tea.Msg {
				return message.ShowBannerMsg{
					Message: fmt.Sprintf("Error deleting network: %s", msg.Error.Error()),
					IsError: true,
				}
			}
		}
		s.list.RemoveItem(msg.Idx)
		return func() tea.Msg {
			return message.ShowBannerMsg{
				Message: fmt.Sprintf("Network %s deleted", msg.Name),
				IsError: false,
			}
		}
	case tea.KeyMsg:
		if s.isFilter {
			var filterCmds []tea.Cmd
			var listCmd tea.Cmd
			s.list, listCmd = s.list.Update(msg)
			filterCmds = append(filterCmds, listCmd)

			if key.Matches(msg, keys.Keys.Esc) {
				s.isFilter = !s.isFilter
				filterCmds = append(filterCmds, func() tea.Msg { return message.ClearContextualKeyBindingsMsg{} })
			}
			return tea.Batch(filterCmds...)
		}

		switch {
		case key.Matches(msg, keys.Keys.NetworkInfo):
			if s.activePanel == s.detailsPanel {
				cmd := s.activePanel.Close()
				s.activePanel = nil
				return cmd
			}
			selected := s.list.SelectedItem()
			if selected == nil {
				return nil
			}
			item, ok := selected.(networkItem)
			if !ok {
				return nil
			}
			s.activePanel = s.detailsPanel
			return s.detailsPanel.Init(formatNetworkDetails(item.network))
		case key.Matches(msg, keys.Keys.Refresh):
			s.loading = true
			return tea.Batch(s.spinner.Tick, s.updateNetworksCmd())
		case key.Matches(msg, keys.Keys.Prune):
			return s.confirmNetworkPrune()
		case key.Matches(msg, keys.Keys.NetworkDelete):
			return s.confirmNetworkDelete()
		case key.Matches(msg, keys.Keys.Up, keys.Keys.Down):
			var listCmd tea.Cmd
			s.list, listCmd = s.list.Update(msg)
			return tea.Batch(listCmd, s.clearDetails())
		case key.Matches(msg, keys.Keys.Filter):
			s.isFilter = !s.isFilter
			var listCmd tea.Cmd
			s.list, listCmd = s.list.Update(msg)
			return tea.Batch(listCmd, s.extendFilterHelpCommand())
		}
	}

	if s.activePanel != nil {
		cmds = append(cmds, s.activePanel.Update(msg))
	}

	return tea.Batch(cmds...)
}

// View renders the section.
func (s *Section) View() string {
	s.SetSize(s.width, s.height)
	listContent := s.list.View()

	if s.loading {
		spinnerText := s.spinner.View() + " Loading..."
		listContent = helper.OverlayBottomRight(1, listContent, spinnerText, s.list.Width())
	}

	listView := theme.ListStyle.
		Width(s.list.Width()).
		Render(listContent)

	if s.activePanel == nil {
		return listView
	}

	detailContent := s.activePanel.View()
	detailView := theme.ListStyle.
		Width(s.viewport.Width).
		Height(s.viewport.Height).
		Render(detailContent)

	return lipgloss.JoinHorizontal(lipgloss.Top, listView, detailView)
}

// Reset reset internal state to when a component is first initialized.
func (s *Section) Reset() tea.Cmd {
	s.isFilter = false
	s.viewport.SetContent("")
	var cmd tea.Cmd
	if s.activePanel != nil {
		cmd = s.activePanel.Close()
		s.activePanel = nil
	}
	s.SetSize(s.width, s.height)
	return cmd
}

func (s *Section) clearDetails() tea.Cmd {
	var cmd tea.Cmd
	if s.activePanel != nil {
		cmd = s.activePanel.Close()
		s.activePanel = nil
	}
	s.viewport.SetContent("")
	return cmd
}

func (s *Section) updateNetworksCmd() tea.Cmd {
	ctx := s.ctx
	svc := s.networkService
	return func() tea.Msg {
		networks, err := svc.List(ctx)
		if err != nil {
			return networksLoadedMsg{error: err}
		}
		items := make([]list.Item, len(networks))
		for idx, n := range networks {
			items[idx] = networkItem{network: n}
		}
		return networksLoadedMsg{items: items}
	}
}

func (s *Section) deleteNetworkCmd() tea.Cmd {
	ctx := s.ctx
	svc := s.networkService
	items := s.list.Items()
	idx := s.list.Index()
	if idx < 0 || idx >= len(items) {
		return nil
	}

	item, ok := items[idx].(networkItem)
	if !ok {
		return nil
	}

	return func() tea.Msg {
		err := svc.Remove(ctx, item.network.ID)
		return networkRemovedMsg{ID: item.network.ID, Name: item.network.Name, Idx: idx, Error: err}
	}
}

func (s *Section) pruneNetworksCmd() tea.Cmd {
	ctx, svc := s.ctx, s.networkService
	return func() tea.Msg {
		report, err := svc.Prune(ctx, client.PruneOptions{})
		return networksPrunedMsg{report: report, err: err}
	}
}

func (s *Section) confirmNetworkPrune() tea.Cmd {
	pruneCmd := s.pruneNetworksCmd()
	return func() tea.Msg {
		return message.ShowConfirmationMsg{
			Title:     "Prune Networks",
			Body:      "Remove all unused networks?",
			OnConfirm: pruneCmd,
		}
	}
}

func (s *Section) confirmNetworkDelete() tea.Cmd {
	items := s.list.Items()
	idx := s.list.Index()
	if idx < 0 || idx >= len(items) {
		return nil
	}
	item, ok := items[idx].(networkItem)
	if !ok {
		return nil
	}
	deleteCmd := s.deleteNetworkCmd()
	return func() tea.Msg {
		return message.ShowConfirmationMsg{
			Title:     "Delete Network",
			Body:      fmt.Sprintf("Delete network %s?", item.network.Name),
			OnConfirm: deleteCmd,
		}
	}
}

func (s *Section) extendFilterHelpCommand() tea.Cmd {
	return func() tea.Msg {
		return message.AddContextualKeyBindingsMsg{Bindings: []key.Binding{
			key.NewBinding(
				key.WithKeys("esc"),
				key.WithHelp("esc", "exit"),
			),
		}}
	}
}
