package compose

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

// composeLoadedMsg is sent when compose projects have been loaded asynchronously.
type composeLoadedMsg struct {
	error error
	items []list.Item
}

type composeActionMsg struct {
	project client.ComposeProject
	action  string
	err     error
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
	s.HandleKey = s.handleKey

	return s
}

func (s *Section) handleMsg(msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case composeLoadedMsg:
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
		return tea.Batch(s.List.SetItems(msg.items), s.Section.Init()), true
	case composeActionMsg:
		if msg.err != nil {
			return func() tea.Msg {
				return message.ShowBannerMsg{
					Message: fmt.Sprintf("Error %s compose project: %s", msg.action, msg.err.Error()),
					IsError: true,
				}
			}, true
		}

		return tea.Batch(
			s.updateComposeCmd(),
			func() tea.Msg {
				return message.ShowBannerMsg{
					Message: fmt.Sprintf("Compose project %s %s", msg.project.Name, msg.action),
					IsError: false,
				}
			},
		), true
	}

	return nil, false
}

func (s *Section) handleKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	switch {
	case key.Matches(msg, keys.Keys.ComposeUp):
		return s.confirmProjectUp(), true
	case key.Matches(msg, keys.Keys.ComposeDown):
		return s.confirmProjectDown(), true
	case key.Matches(msg, keys.Keys.ComposeStartStop):
		return s.confirmProjectToggle(), true
	case key.Matches(msg, keys.Keys.ComposeRestart):
		return s.confirmProjectRestart(), true
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
		return composeLoadedMsg{items: items}
	}
}

func (s *Section) selectedProject() (client.ComposeProject, bool) {
	items := s.List.Items()
	idx := s.List.Index()
	if idx < 0 || idx >= len(items) {
		return client.ComposeProject{}, false
	}

	item, ok := items[idx].(composeItem)
	if !ok {
		return client.ComposeProject{}, false
	}

	return item.project, true
}

func (s *Section) projectUpCmd() tea.Cmd {
	project, ok := s.selectedProject()
	if !ok {
		return nil
	}

	return func() tea.Msg {
		err := s.composeService.Up(s.ctx, project, client.ComposeUpOptions{})
		return composeActionMsg{project: project, action: "up", err: err}
	}
}

func (s *Section) projectDownCmd() tea.Cmd {
	project, ok := s.selectedProject()
	if !ok {
		return nil
	}

	return func() tea.Msg {
		err := s.composeService.Down(s.ctx, project, client.ComposeDownOptions{})
		return composeActionMsg{project: project, action: "down", err: err}
	}
}

func (s *Section) projectToggleCmd() tea.Cmd {
	project, ok := s.selectedProject()
	if !ok {
		return nil
	}

	return func() tea.Msg {
		var (
			action string
			err    error
		)

		if projectHasRunningServices(project) {
			action = "stopped"
			err = s.composeService.Stop(s.ctx, project, client.ComposeStopOptions{})
		} else {
			action = "started"
			err = s.composeService.Start(s.ctx, project, client.ComposeStartOptions{})
		}

		return composeActionMsg{project: project, action: action, err: err}
	}
}

func (s *Section) projectRestartCmd() tea.Cmd {
	project, ok := s.selectedProject()
	if !ok {
		return nil
	}

	return func() tea.Msg {
		err := s.composeService.Restart(s.ctx, project, client.ComposeRestartOptions{})
		return composeActionMsg{project: project, action: "restarted", err: err}
	}
}

func (s *Section) confirmProjectUp() tea.Cmd {
	project, ok := s.selectedProject()
	if !ok {
		return nil
	}

	upCmd := s.projectUpCmd()
	return func() tea.Msg {
		return message.ShowConfirmationMsg{
			Title:     "Compose Up",
			Body:      fmt.Sprintf("Run compose up for project %s?", project.Name),
			OnConfirm: upCmd,
		}
	}
}

func (s *Section) confirmProjectDown() tea.Cmd {
	project, ok := s.selectedProject()
	if !ok {
		return nil
	}

	downCmd := s.projectDownCmd()
	return func() tea.Msg {
		return message.ShowConfirmationMsg{
			Title:     "Compose Down",
			Body:      fmt.Sprintf("Run compose down for project %s?", project.Name),
			OnConfirm: downCmd,
		}
	}
}

func (s *Section) confirmProjectToggle() tea.Cmd {
	project, ok := s.selectedProject()
	if !ok {
		return nil
	}

	action := "Start"
	if projectHasRunningServices(project) {
		action = "Stop"
	}

	toggleCmd := s.projectToggleCmd()
	return func() tea.Msg {
		return message.ShowConfirmationMsg{
			Title:     fmt.Sprintf("%s Compose Project", action),
			Body:      fmt.Sprintf("%s compose project %s?", action, project.Name),
			OnConfirm: toggleCmd,
		}
	}
}

func (s *Section) confirmProjectRestart() tea.Cmd {
	project, ok := s.selectedProject()
	if !ok {
		return nil
	}

	restartCmd := s.projectRestartCmd()
	return func() tea.Msg {
		return message.ShowConfirmationMsg{
			Title:     "Restart Compose Project",
			Body:      fmt.Sprintf("Restart compose project %s?", project.Name),
			OnConfirm: restartCmd,
		}
	}
}

func projectHasRunningServices(project client.ComposeProject) bool {
	for _, service := range project.Services {
		if service.State == "running" {
			return true
		}
	}

	return false
}
