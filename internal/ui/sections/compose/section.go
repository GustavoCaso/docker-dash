package compose

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/ui/components/panel"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
	"github.com/GustavoCaso/docker-dash/internal/ui/sections/base"
	"github.com/GustavoCaso/docker-dash/internal/ui/theme"
)

// composeLoadedMsg is sent when compose projects have been loaded asynchronously.
type composeLoadedMsg struct {
	error    error
	items    []list.Item
	projects []client.ComposeProject
}

// composeItem implements list.Item interface for a Compose project.
type composeItem struct {
	project client.ComposeProject
}

func (c composeItem) Title() string { return c.project.Name }
func (c composeItem) Description() string {
	total := len(c.project.Services)
	running := 0
	for _, svc := range c.project.Services {
		if svc.State == "running" {
			running++
		}
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("%d services", total))
	if total > 0 {
		runningStr := theme.StatusRunningStyle.Render(fmt.Sprintf("● %d running", running))
		parts = append(parts, runningStr)
	}
	return strings.Join(parts, " • ")
}
func (c composeItem) FilterValue() string { return c.project.Name }

// Section wraps bubbles/list for displaying Compose projects.
type Section struct {
	*base.Section
	ctx            context.Context
	composeService client.ComposeProjectService
}

// New creates a new Compose section.
func New(ctx context.Context, projects []client.ComposeProject, svc client.ComposeProjectService) *Section {
	items := make([]list.Item, len(projects))
	for i, p := range projects {
		items[i] = composeItem{project: p}
	}

	s := &Section{
		ctx:            ctx,
		composeService: svc,
		Section:        base.New("compose", items, []panel.Panel{newDetailsPanel()}),
	}

	s.LoadingText = "Loading..."
	s.ActivePanelInitFn = func(item list.Item) string {
		ci, ok := item.(composeItem)
		if !ok {
			return ""
		}
		return formatProjectDetails(ci.project)
	}
	s.RefreshCmd = s.updateComposeCmd
	s.HandleMsg = s.handleMsg

	return s
}

func (s *Section) handleMsg(msg tea.Msg) (tea.Cmd, bool) {
	if msg, ok := msg.(composeLoadedMsg); ok {
		log.Printf("[compose] composeLoadedMsg: count=%d", len(msg.items))
		s.Loading = false
		if msg.error != nil {
			return func() tea.Msg {
				return message.ShowBannerMsg{
					Message: fmt.Sprintf("Error loading compose projects: %s", msg.error.Error()),
					IsError: true,
				}
			}, true
		}
		// Rebuild the ActivePanelInitFn closure with the updated project data.
		projects := msg.projects
		s.ActivePanelInitFn = func(item list.Item) string {
			ci, itemOk := item.(composeItem)
			if !itemOk {
				return ""
			}
			for _, p := range projects {
				if p.Name == ci.project.Name {
					return formatProjectDetails(p)
				}
			}
			return formatProjectDetails(ci.project)
		}
		return s.List.SetItems(msg.items), true
	}

	return nil, false
}

func (s *Section) updateComposeCmd() tea.Cmd {
	ctx := s.ctx
	svc := s.composeService
	return func() tea.Msg {
		projects, err := svc.List(ctx)
		if err != nil {
			return composeLoadedMsg{error: err}
		}
		items := make([]list.Item, len(projects))
		for i, p := range projects {
			items[i] = composeItem{project: p}
		}
		return composeLoadedMsg{items: items, projects: projects}
	}
}
