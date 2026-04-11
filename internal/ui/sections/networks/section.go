package networks

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

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
	*base.Section
	ctx            context.Context
	networkService client.NetworkService
}

// New creates a new network section.
func New(ctx context.Context, networks []client.Network, svc client.NetworkService) *Section {
	items := make([]list.Item, len(networks))
	for i, n := range networks {
		items[i] = networkItem{network: n}
	}

	s := &Section{
		ctx:            ctx,
		networkService: svc,
		Section:        base.New("networks", items, []panel.Panel{newDetailsPanel()}),
	}

	s.LoadingText = "Loading..."
	s.ActivePanelInitFn = func(item list.Item) string {
		ni, ok := item.(networkItem)
		if !ok {
			return ""
		}
		return formatNetworkDetails(ni.network)
	}
	s.RefreshCmd = s.updateNetworksCmd
	s.PruneCmd = s.confirmNetworkPrune
	s.HandleMsg = s.handleMsg
	s.HandleKey = s.handleKey

	return s
}

func (s *Section) handleMsg(msg tea.Msg) (tea.Cmd, bool) {
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
			}, true
		}
		return s.List.SetItems(msg.items), true
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
			}, true
		}
		summary := fmt.Sprintf("Pruned %d networks", msg.report.ItemsDeleted)
		return tea.Batch(s.updateNetworksCmd(), func() tea.Msg {
			return message.ShowBannerMsg{Message: summary, IsError: false}
		}), true
	case networkRemovedMsg:
		log.Printf("[networks] networkRemovedMsg: id=%q err=%v", msg.ID, msg.Error)
		if msg.Error != nil {
			return func() tea.Msg {
				return message.ShowBannerMsg{
					Message: fmt.Sprintf("Error deleting network: %s", msg.Error.Error()),
					IsError: true,
				}
			}, true
		}
		return tea.Batch(s.RemoveItemAndUpdatePanel(msg.Idx), func() tea.Msg {
			return message.ShowBannerMsg{
				Message: fmt.Sprintf("Network %s deleted", msg.Name),
				IsError: false,
			}
		}), true
	}
	return nil, false
}

func (s *Section) handleKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	if key.Matches(msg, keys.Keys.NetworkDelete) {
		return s.confirmNetworkDelete(), true
	}
	return nil, false
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
