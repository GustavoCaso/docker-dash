package networks

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/ui/components/panel"
	"github.com/GustavoCaso/docker-dash/internal/ui/keys"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
	"github.com/GustavoCaso/docker-dash/internal/ui/sections/base"
	"github.com/GustavoCaso/docker-dash/internal/ui/theme"
)

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
	base.Section
	ctx            context.Context
	networkService client.NetworkService
	panels         []panel.Panel
	activePanelIdx int
	panelWidth     int
	panelHeight    int
}

// New creates a new network section.
func New(ctx context.Context, networks []client.Network, svc client.NetworkService) *Section {
	items := make([]list.Item, len(networks))
	for i, n := range networks {
		items[i] = networkItem{network: n}
	}

	return &Section{
		ctx:            ctx,
		networkService: svc,
		panels:         []panel.Panel{newDetailsPanel()},
		activePanelIdx: 0,
		Section: base.Section{
			List:    base.NewList(items),
			Spinner: base.NewSpinner(),
		},
	}
}

func (s *Section) Init() tea.Cmd {
	selected := s.List.SelectedItem()
	if selected == nil {
		return nil
	}
	item, ok := selected.(networkItem)
	if !ok {
		return nil
	}
	return s.activePanel().Init(formatNetworkDetails(item.network))
}

// SetSize sets dimensions.
func (s *Section) SetSize(width, height int) {
	s.Width = width
	s.Height = height

	// Account for details menu height
	menuHeight := lipgloss.Height(s.detailsMenu())
	menuX, menuY := theme.Tab.GetFrameSize()

	// Account for padding and borders
	listX, listY := theme.ListStyle.GetFrameSize()

	// Panel Style
	panelX, panelY := theme.NoBorders.GetFrameSize()

	listWidth := int(float64(width) * theme.SplitRatio)
	detailWidth := width - listWidth

	s.List.SetSize(listWidth-listX, height-listY)
	s.panelWidth = detailWidth - panelX - menuX
	// TODO: Figure out the + 1
	s.panelHeight = height - menuHeight - menuY - panelY + 1
	s.activePanel().SetSize(s.panelWidth, s.panelHeight)
}

// Update handles messages.
func (s *Section) Update(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd

	if spinnerCmd := s.UpdateSpinner(msg); spinnerCmd != nil {
		cmds = append(cmds, spinnerCmd)
	}

	switch msg := msg.(type) {
	case networksLoadedMsg:
		log.Printf("[networks] networksLoadedMsg: count=%d", len(msg.items))
		s.Loading = false
		if msg.error != nil {
			return func() tea.Msg {
				return message.ShowBannerMsg{
					Message: fmt.Sprintf("Error loading networks: %s", msg.error.Error()),
					IsError: true,
				}
			}
		}
		cmd := s.List.SetItems(msg.items)
		cmds = append(cmds, cmd)
		return tea.Batch(cmds...)
	case networksPrunedMsg:
		log.Printf(
			"[networks] networksPrunedMsg: deleted=%d spaceReclaimed=%d",
			msg.report.ItemsDeleted,
			msg.report.SpaceReclaimed,
		)
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
		log.Printf("[networks] networkRemovedMsg: id=%q err=%v", msg.ID, msg.Error)
		if msg.Error != nil {
			return func() tea.Msg {
				return message.ShowBannerMsg{
					Message: fmt.Sprintf("Error deleting network: %s", msg.Error.Error()),
					IsError: true,
				}
			}
		}
		return tea.Batch(s.removeItem(msg.Idx), func() tea.Msg {
			return message.ShowBannerMsg{
				Message: fmt.Sprintf("Network %s deleted", msg.Name),
				IsError: false,
			}
		})
	case tea.KeyMsg:
		log.Printf("[networks] KeyMsg: key=%q", msg.String())
		if handled, filterCmds := s.HandleFilterKey(msg); handled {
			return tea.Batch(filterCmds...)
		}

		switch {
		case key.Matches(msg, keys.Keys.PanelNext):
			currentPanel := s.activePanel()
			s.activePanelIdx = (s.activePanelIdx + 1) % len(s.panels)
			return tea.Batch(currentPanel.Close(), s.updateActivePanel())
		case key.Matches(msg, keys.Keys.PanelPrev):
			currentPanel := s.activePanel()
			s.activePanelIdx = (s.activePanelIdx - 1 + len(s.panels)) % len(s.panels)
			return tea.Batch(currentPanel.Close(), s.updateActivePanel())
		case key.Matches(msg, keys.Keys.Refresh):
			s.Loading = true
			return tea.Batch(s.Spinner.Tick, s.updateNetworksCmd())
		case key.Matches(msg, keys.Keys.Prune):
			return s.confirmNetworkPrune()
		case key.Matches(msg, keys.Keys.NetworkDelete):
			return s.confirmNetworkDelete()
		case key.Matches(msg, keys.Keys.Up, keys.Keys.Down):
			currentPanel := s.activePanel()
			var listCmd tea.Cmd
			s.List, listCmd = s.List.Update(msg)
			return tea.Batch(listCmd, currentPanel.Close(), s.updateActivePanel())
		case key.Matches(msg, keys.Keys.Filter):
			return tea.Batch(s.ToggleFilter(msg)...)
		}
	}

	cmds = append(cmds, s.activePanel().Update(msg))

	return tea.Batch(cmds...)
}

// View renders the section.
func (s *Section) View() string {
	s.SetSize(s.Width, s.Height)

	listView := s.RenderList("Loading...")

	detailContent := s.activePanel().View()

	details := theme.NoBorders.
		Width(s.panelWidth).
		Height(s.panelHeight).
		Render(detailContent)

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		listView,
		lipgloss.JoinVertical(lipgloss.Top, s.detailsMenu(), details),
	)
}

func (s *Section) detailsMenu() string {
	sectionsMenu := make([]string, 0, len(s.panels))
	for idx, p := range s.panels {
		if idx == s.activePanelIdx {
			sectionsMenu = append(sectionsMenu, theme.ActiveTab.Render(p.Name()))
		} else {
			sectionsMenu = append(sectionsMenu, theme.Tab.Render(p.Name()))
		}
	}

	detailsMenu := lipgloss.JoinHorizontal(lipgloss.Top, sectionsMenu...)
	gap := theme.TabGap.Render(strings.Repeat(" ", max(0, s.panelWidth-lipgloss.Width(detailsMenu))))

	return lipgloss.JoinHorizontal(lipgloss.Bottom, detailsMenu, gap)
}

// Reset resets internal state to when a component is first initialized.
func (s *Section) Reset() tea.Cmd {
	s.IsFilter = false
	cmd := s.activePanel().Close()
	s.SetSize(s.Width, s.Height)
	return cmd
}

func (s *Section) activePanel() panel.Panel {
	return s.panels[s.activePanelIdx]
}

func (s *Section) removeItem(idx int) tea.Cmd {
	s.List.RemoveItem(idx)
	if len(s.List.Items()) == 0 {
		return s.activePanel().Close()
	}
	s.List.Select(min(idx, len(s.List.Items())-1))
	return s.updateActivePanel()
}

func (s *Section) updateActivePanel() tea.Cmd {
	selected := s.List.SelectedItem()
	if selected == nil {
		return nil
	}
	item, ok := selected.(networkItem)
	if !ok {
		return nil
	}
	return s.activePanel().Init(formatNetworkDetails(item.network))
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
	items := s.List.Items()
	idx := s.List.Index()
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
	items := s.List.Items()
	idx := s.List.Index()
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
