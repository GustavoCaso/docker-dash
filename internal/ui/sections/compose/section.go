package compose

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/ui/components/form"
	"github.com/GustavoCaso/docker-dash/internal/ui/components/panel"
	"github.com/GustavoCaso/docker-dash/internal/ui/keys"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
	"github.com/GustavoCaso/docker-dash/internal/ui/sections"
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
func New(ctx context.Context, svc client.ComposeProjectService) *Section {
	s := &Section{
		ctx:            ctx,
		composeService: svc,
		Section:        base.New(sections.ComposeSection, []panel.Panel{newDetailsPanel()}),
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

func (s *Section) handleMsg(msg tea.Msg) base.UpdateResult {
	switch msg := msg.(type) {
	case composeLoadedMsg:
		log.Printf("[compose] composeLoadedMsg: count=%d", len(msg.items))
		if msg.error != nil {
			return base.UpdateResult{
				Cmd: func() tea.Msg {
					return message.ShowBannerMsg{
						Message: fmt.Sprintf("Error loading compose projects: %s", msg.error.Error()),
						IsError: true,
					}
				},
				Handled:     true,
				StopSpinner: true,
			}
		}
		return base.UpdateResult{
			Cmd:         tea.Batch(s.UpdateItems(msg.items)...),
			Handled:     true,
			StopSpinner: true,
		}
	case composeActionMsg:
		if msg.err != nil {
			return base.UpdateResult{
				Cmd: func() tea.Msg {
					return message.ShowBannerMsg{
						Message: fmt.Sprintf("Error %s compose project: %s", msg.action, msg.err.Error()),
						IsError: true,
					}
				},
				Handled:     true,
				StopSpinner: true,
			}
		}

		return base.UpdateResult{
			Cmd: tea.Batch(
				s.updateComposeCmd(),
				func() tea.Msg {
					return message.ShowBannerMsg{
						Message: fmt.Sprintf("Compose project %s %s", msg.project.Name, msg.action),
						IsError: false,
					}
				},
			),
			Handled: true,
		}
	}

	return base.UpdateResult{}
}

func (s *Section) handleKey(msg tea.KeyMsg) base.UpdateResult {
	switch {
	case key.Matches(msg, keys.Keys.ComposeUp):
		return base.UpdateResult{Cmd: s.showUpForm(), Handled: true}
	case key.Matches(msg, keys.Keys.ComposeDown):
		return base.UpdateResult{Cmd: s.showDownForm(), Handled: true}
	case key.Matches(msg, keys.Keys.ComposeStartStop):
		return base.UpdateResult{Cmd: s.confirmProjectToggle(), Handled: true}
	case key.Matches(msg, keys.Keys.ComposeRestart):
		return base.UpdateResult{Cmd: s.showRestartForm(), Handled: true}
	}

	return base.UpdateResult{}
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

func (s *Section) projectUpCmd(opts client.ComposeUpOptions) tea.Cmd {
	project, ok := s.selectedProject()
	if !ok {
		return nil
	}

	return func() tea.Msg {
		err := s.composeService.Up(s.ctx, project, opts)
		return composeActionMsg{project: project, action: "up", err: err}
	}
}

func (s *Section) projectDownCmd(opts client.ComposeDownOptions) tea.Cmd {
	project, ok := s.selectedProject()
	if !ok {
		return nil
	}

	return func() tea.Msg {
		err := s.composeService.Down(s.ctx, project, opts)
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

func (s *Section) projectRestartCmd(opts client.ComposeRestartOptions) tea.Cmd {
	project, ok := s.selectedProject()
	if !ok {
		return nil
	}

	return func() tea.Msg {
		err := s.composeService.Restart(s.ctx, project, opts)
		return composeActionMsg{project: project, action: "restarted", err: err}
	}
}

func (s *Section) showUpForm() tea.Cmd {
	project, ok := s.selectedProject()
	if !ok {
		return nil
	}

	var build, removeOrphans, wait bool
	f := composeUpForm(&build, &removeOrphans, &wait)
	upForm := form.New(
		fmt.Sprintf("Up — %s", project.Name),
		f,
		func(_ *huh.Form) tea.Cmd {
			opts := client.ComposeUpOptions{
				Build:         build,
				RemoveOrphans: removeOrphans,
				Wait:          wait,
			}
			return s.WithSpinner(s.projectUpCmd(opts))
		},
	)
	return func() tea.Msg {
		return message.ShowFormMsg{Form: upForm}
	}
}

func (s *Section) showDownForm() tea.Cmd {
	project, ok := s.selectedProject()
	if !ok {
		return nil
	}

	var removeOrphans, volumes bool
	var images string
	f := composeDownForm(&removeOrphans, &volumes, &images)
	downForm := form.New(
		fmt.Sprintf("Down — %s", project.Name),
		f,
		func(_ *huh.Form) tea.Cmd {
			opts := client.ComposeDownOptions{
				RemoveOrphans: removeOrphans,
				Volumes:       volumes,
				Images:        images,
			}
			return s.WithSpinner(s.projectDownCmd(opts))
		},
	)
	return func() tea.Msg {
		return message.ShowFormMsg{Form: downForm}
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
			OnConfirm: s.WithSpinner(toggleCmd),
		}
	}
}

func (s *Section) showRestartForm() tea.Cmd {
	project, ok := s.selectedProject()
	if !ok {
		return nil
	}

	var noDeps bool
	var timeoutStr string
	f := composeRestartForm(&noDeps, &timeoutStr)
	restartForm := form.New(
		fmt.Sprintf("Restart — %s", project.Name),
		f,
		func(_ *huh.Form) tea.Cmd {
			return s.WithSpinner(s.projectRestartCmd(buildRestartOptions(noDeps, timeoutStr)))
		},
	)
	return func() tea.Msg {
		return message.ShowFormMsg{Form: restartForm}
	}
}

func buildRestartOptions(noDeps bool, timeoutStr string) client.ComposeRestartOptions {
	opts := client.ComposeRestartOptions{NoDeps: noDeps}
	if t := strings.TrimSpace(timeoutStr); t != "" {
		if d, err := time.ParseDuration(t); err == nil {
			opts.Timeout = &d
		}
	}
	return opts
}

func projectHasRunningServices(project client.ComposeProject) bool {
	for _, service := range project.Services {
		if service.State == "running" {
			return true
		}
	}

	return false
}

func composeUpForm(build, removeOrphans, wait *bool) *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Key("build").
				Title("Build").
				Description("Rebuild images before starting containers.").
				Value(build),

			huh.NewConfirm().
				Key("remove_orphans").
				Title("Remove Orphans").
				Description("Remove containers for services not defined in the Compose file.").
				Value(removeOrphans),

			huh.NewConfirm().
				Key("wait").
				Title("Wait").
				Description("Wait for services to be healthy before returning.").
				Value(wait),
		),
	)
}

func composeDownForm(removeOrphans, volumes *bool, images *string) *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Key("remove_orphans").
				Title("Remove Orphans").
				Description("Remove containers for services not defined in the Compose file.").
				Value(removeOrphans),

			huh.NewConfirm().
				Key("volumes").
				Title("Remove Volumes").
				Description("Remove named volumes declared in the Compose file.").
				Value(volumes),

			huh.NewSelect[string]().
				Key("images").
				Title("Remove Images").
				Value(images).
				Options(
					huh.NewOption("none", ""),
					huh.NewOption("local (images without a custom tag)", "local"),
					huh.NewOption("all (all images used by services)", "all"),
				),
		),
	)
}

func composeRestartForm(noDeps *bool, timeout *string) *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Key("no_deps").
				Title("No Deps").
				Description("Don't restart dependent services.").
				Value(noDeps),

			huh.NewInput().
				Key("timeout").
				Title("Timeout").
				Description("Shutdown timeout before killing (e.g. 10s, 1m). Leave blank for default.").
				Value(timeout).
				Validate(validateOptionalDuration),
		),
	)
}

func validateOptionalDuration(s string) error {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	_, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: use Go format e.g. 10s, 1m30s", s)
	}
	return nil
}
