package networks

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/GustavoCaso/docker-dash/internal/client"
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
	width, height  int
	showDetails    bool
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

	vp := viewport.New(0, 0)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	s := &Section{
		ctx:            ctx,
		list:           l,
		viewport:       vp,
		networkService: svc,
		spinner:        sp,
	}
	s.updateDetails()
	return s
}

// SetSize sets dimensions.
func (s *Section) SetSize(width, height int) {
	s.width = width
	s.height = height

	listX, listY := theme.ListStyle.GetFrameSize()

	if s.showDetails {
		listWidth := int(float64(width) * listSplitRatio)
		detailWidth := width - listWidth

		s.list.SetSize(listWidth-listX, height-listY)
		s.viewport.Width = detailWidth - listX
		s.viewport.Height = height - listY
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
			s.showDetails = !s.showDetails
			s.SetSize(s.width, s.height)
			return nil
		case key.Matches(msg, keys.Keys.Refresh):
			s.loading = true
			return tea.Batch(s.spinner.Tick, s.updateNetworksCmd())
		case key.Matches(msg, keys.Keys.NetworkDelete):
			return s.deleteNetworkCmd()
		case key.Matches(msg, keys.Keys.Up, keys.Keys.Down):
			var listCmd tea.Cmd
			s.list, listCmd = s.list.Update(msg)
			return listCmd
		case key.Matches(msg, keys.Keys.ScrollUp, keys.Keys.ScrollDown):
			var vpCmd tea.Cmd
			s.viewport, vpCmd = s.viewport.Update(msg)
			return vpCmd
		case key.Matches(msg, keys.Keys.Filter):
			s.isFilter = !s.isFilter
			var listCmd tea.Cmd
			s.list, listCmd = s.list.Update(msg)
			return tea.Batch(listCmd, s.extendFilterHelpCommand())
		}
	}

	var listCmd tea.Cmd
	s.list, listCmd = s.list.Update(msg)
	cmds = append(cmds, listCmd)

	var vpCmd tea.Cmd
	s.viewport, vpCmd = s.viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	return tea.Batch(cmds...)
}

// View renders the section.
func (s *Section) View() string {
	listContent := s.list.View()

	if s.loading {
		spinnerText := s.spinner.View() + " Loading..."
		listContent = helper.OverlayBottomRight(1, listContent, spinnerText, s.list.Width())
	}

	listView := theme.ListStyle.
		Width(s.list.Width()).
		Render(listContent)

	if !s.showDetails {
		return listView
	}

	s.updateDetails()

	detailView := theme.ListStyle.
		Width(s.viewport.Width).
		Render(s.viewport.View())

	return lipgloss.JoinHorizontal(lipgloss.Top, listView, detailView)
}

// Reset reset internal state to when a component isfirst initialized.
func (s *Section) Reset() tea.Cmd {
	s.isFilter = false
	s.showDetails = false
	s.viewport.SetContent("")
	s.SetSize(s.width, s.height)
	return nil
}

func (s *Section) updateDetails() {
	selected := s.list.SelectedItem()
	if selected == nil {
		s.viewport.SetContent("No network selected")
		return
	}

	item, ok := selected.(networkItem)
	if !ok {
		return
	}
	n := item.network

	const shortIDLen = 12
	id := n.ID
	if len(id) > shortIDLen {
		id = id[:shortIDLen]
	}

	var content strings.Builder

	fmt.Fprintf(&content, "Network: %s\n", n.Name)
	content.WriteString("═══════════════════════\n\n")

	label := theme.DetailLabelStyle
	value := theme.DetailValueStyle

	fmt.Fprintf(&content, "%s%s\n", label.Render("ID"), value.Render(id))
	fmt.Fprintf(&content, "%s%s\n", label.Render("Driver"), value.Render(n.Driver))
	fmt.Fprintf(&content, "%s%s\n", label.Render("Scope"), value.Render(n.Scope))

	internalStr := "false"
	if n.Internal {
		internalStr = "true"
	}
	fmt.Fprintf(&content, "%s%s\n", label.Render("Internal"), value.Render(internalStr))

	if n.IPAM.Subnet != "" {
		fmt.Fprintf(&content, "%s%s\n", label.Render("Subnet"), value.Render(n.IPAM.Subnet))
	}
	if n.IPAM.Gateway != "" {
		fmt.Fprintf(&content, "%s%s\n", label.Render("Gateway"), value.Render(n.IPAM.Gateway))
	}

	fmt.Fprintf(&content, "%s%s\n", label.Render("Containers"), value.Render(strconv.Itoa(len(n.ConnectedContainers))))

	if len(n.ConnectedContainers) > 0 {
		content.WriteString("\n")
		content.WriteString(label.Render("Connected Containers"))
		content.WriteString("\n")
		for _, c := range n.ConnectedContainers {
			ip := c.IPv4Address
			if ip == "" {
				ip = c.IPv6Address
			}
			if ip == "" {
				fmt.Fprintf(&content, "  %s\n", value.Render(c.Name))
			} else {
				fmt.Fprintf(&content, "  %s  %s\n", value.Render(c.Name), label.Render(ip))
			}
		}
	}

	fmt.Fprintf(&content, "\n%s%s\n", label.Render("Created"), value.Render(n.Created.Format(time.RFC3339)))

	s.viewport.SetContent(content.String())
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
