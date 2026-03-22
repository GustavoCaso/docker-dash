package images

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/ui/components/panel"
	"github.com/GustavoCaso/docker-dash/internal/ui/helper"
	"github.com/GustavoCaso/docker-dash/internal/ui/keys"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
	"github.com/GustavoCaso/docker-dash/internal/ui/theme"
)

// imagesLoadedMsg is sent when images have been loaded asynchronously.
type imagesLoadedMsg struct {
	error error
	items []list.Item
}

// containerRunMsg is sent when a container is created.
type containerRunMsg struct {
	containerID string
	error       error
}

// imagesPrunedMsg is sent when an image prune completes.
type imagesPrunedMsg struct {
	report client.PruneReport
	err    error
}

// imageRemovedMsg is sent when an image deletion completes.
type imageRemovedMsg struct {
	ID    string
	Idx   int
	Error error
}

// imageItem implements list.Item interface.
type imageItem struct {
	image client.Image
}

func (i imageItem) ID() string    { return i.image.ID }
func (i imageItem) Title() string { return fmt.Sprintf("%s:%s", i.image.Repo, i.image.Tag) }
func (i imageItem) Description() string {
	stateIcon := theme.GetImageStatusIcon(i.image.Containers)
	stateStyle := theme.GetImageStatusStyle(i.image.Containers)
	state := stateStyle.Render(stateIcon)
	return state + " " + helper.FormatSize(i.image.Size)
}
func (i imageItem) FilterValue() string { return i.image.Repo + ":" + i.image.Tag }

// Section wraps bubbles/list.
type Section struct {
	ctx                          context.Context
	list                         list.Model
	imageService                 client.ImageService
	containerService             client.ContainerService
	panels                       []panel.Panel
	activePanelIdx               int
	originalWidth, orginalHeight int
	panelWidth, panelHeight      int
	loading                      bool
	isFilter                     bool
	spinner                      spinner.Model
}

// New creates a new image section.
func New(ctx context.Context, images []client.Image, client client.Client) *Section {
	items := make([]list.Item, len(images))
	for i, img := range images {
		items[i] = imageItem{image: img}
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(true)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	il := &Section{
		ctx:              ctx,
		list:             l,
		imageService:     client.Images(),
		containerService: client.Containers(),
		panels:           []panel.Panel{NewLayersPanel(ctx, client.Images())},
		activePanelIdx:   0,
		spinner:          sp,
	}

	return il
}

func (s *Section) Init() tea.Cmd {
	selected := s.list.SelectedItem()
	if selected == nil {
		return nil
	}
	item, ok := selected.(imageItem)
	if !ok {
		return nil
	}
	return s.panels[s.activePanelIdx].Init(item.ID())
}

// SetSize sets dimensions.
func (s *Section) SetSize(width, height int) {
	s.originalWidth = width
	s.orginalHeight = height

	// Account for details menu height
	menuHeight := lipgloss.Height(s.detailsMenu())
	menuX, menuY := theme.Tab.GetFrameSize()

	// Account for padding and borders
	listX, listY := theme.ListStyle.GetFrameSize()

	// Panel Style
	panelX, panelY := theme.NoBorders.GetFrameSize()

	listWidth := int(float64(width) * theme.SplitRatio)
	detailWidth := width - listWidth

	s.list.SetSize(listWidth-listX, height-listY)
	s.panelWidth = detailWidth - panelX - menuX
	// TODO: Figure out the + 1
	s.panelHeight = height - menuHeight - menuY - panelY + 1
	s.activePanel().SetSize(s.panelWidth, s.panelHeight)
}

// Update handles messages.
func (s *Section) Update(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd

	// Handle spinner ticks while loading
	if s.loading {
		var spinnerCmd tea.Cmd
		s.spinner, spinnerCmd = s.spinner.Update(msg)
		cmds = append(cmds, spinnerCmd)
	}

	switch msg := msg.(type) {
	case imagesLoadedMsg:
		log.Printf("[images] imagesLoadedMsg: count=%d", len(msg.items))
		s.loading = false
		if msg.error != nil {
			return func() tea.Msg {
				return message.ShowBannerMsg{
					Message: fmt.Sprintf("Error loading images: %s", msg.error.Error()),
					IsError: true,
				}
			}
		}
		cmd := s.list.SetItems(msg.items)
		cmds = append(cmds, cmd)
		return tea.Batch(cmds...)
	case imagesPrunedMsg:
		log.Printf(
			"[images] imagesPrunedMsg: deleted=%d spaceReclaimed=%d",
			msg.report.ItemsDeleted,
			msg.report.SpaceReclaimed,
		)
		if msg.err != nil {
			return func() tea.Msg {
				return message.ShowBannerMsg{
					Message: "Error pruning images: " + msg.err.Error(),
					IsError: true,
				}
			}
		}
		summary := fmt.Sprintf(
			"Pruned %d images, reclaimed %s",
			msg.report.ItemsDeleted,
			helper.FormatSize(msg.report.SpaceReclaimed),
		)
		return tea.Batch(s.updateImagesCmd(), func() tea.Msg {
			return message.ShowBannerMsg{Message: summary, IsError: false}
		})
	case imageRemovedMsg:
		log.Printf("[images] imageRemovedMsg: imageID=%q err=%v", msg.ID, msg.Error)
		if msg.Error != nil {
			return func() tea.Msg {
				return message.ShowBannerMsg{
					Message: fmt.Sprintf("Error deleting image: %s", msg.Error.Error()),
					IsError: true,
				}
			}
		}
		return tea.Batch(s.removeItem(msg.Idx), func() tea.Msg {
			return message.ShowBannerMsg{
				Message: fmt.Sprintf("Image %s deleted", helper.ShortID(msg.ID)),
				IsError: false,
			}
		})
	case containerRunMsg:
		log.Printf("[images] containerRunMsg: containerID=%q err=%v", msg.containerID, msg.error)
		s.loading = false
		if msg.error != nil {
			return func() tea.Msg {
				return message.ShowBannerMsg{
					Message: msg.error.Error(),
					IsError: true,
				}
			}
		}

		banner := func() tea.Msg {
			return message.ShowBannerMsg{
				Message: fmt.Sprintf("Container %s created", helper.ShortID(msg.containerID)),
				IsError: false,
			}
		}

		refreshComponents := func() tea.Msg {
			return message.BubbleUpMsg{
				KeyMsg: tea.KeyMsg{
					Type:  tea.KeyRunes,
					Runes: []rune{'r'},
				},
				OnlyActive: false,
			}
		}

		return tea.Batch(banner, refreshComponents)

	case tea.KeyMsg:
		log.Printf("[images] KeyMsg: key=%q", msg.String())
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
		case key.Matches(msg, keys.Keys.PanelNext):
			currentPanel := s.activePanel()
			s.activePanelIdx = (s.activePanelIdx + 1) % len(s.panels)
			return tea.Batch(currentPanel.Close(), s.updateActivePanel())
		case key.Matches(msg, keys.Keys.PanelPrev):
			currentPanel := s.activePanel()
			s.activePanelIdx = (s.activePanelIdx - 1 + len(s.panels)) % len(s.panels)
			return tea.Batch(currentPanel.Close(), s.updateActivePanel())
		case key.Matches(msg, keys.Keys.Refresh):
			s.loading = true
			return tea.Batch(s.spinner.Tick, s.updateImagesCmd())
		case key.Matches(msg, keys.Keys.Prune):
			return s.confirmImagePrune()
		case key.Matches(msg, keys.Keys.Delete):
			return s.confirmImageDelete()
		case key.Matches(msg, keys.Keys.CreateAndRunContainer):
			return s.createContainerCmdAndRun()
		case key.Matches(msg, keys.Keys.Up, keys.Keys.Down):
			currentPanel := s.activePanel()
			var listCmd tea.Cmd
			s.list, listCmd = s.list.Update(msg)
			return tea.Batch(listCmd, currentPanel.Close(), s.updateActivePanel())
		case key.Matches(msg, keys.Keys.Filter):
			s.isFilter = !s.isFilter
			var listCmd tea.Cmd
			s.list, listCmd = s.list.Update(msg)
			return tea.Batch(listCmd, s.extendFilterHelpCommand())
		}
	}

	// Send the remaining of msg to active panel
	cmds = append(cmds, s.activePanel().Update(msg))

	return tea.Batch(cmds...)
}

// View renders the list.
func (s *Section) View() string {
	s.SetSize(s.originalWidth, s.orginalHeight)

	listContent := s.list.View()

	// Overlay spinner in bottom right corner when loading
	if s.loading {
		spinnerText := s.spinner.View() + " Refreshing..."
		listContent = helper.OverlayBottomRight(1, listContent, spinnerText, s.list.Width())
	}

	listView := theme.ListStyle.
		Width(s.list.Width()).
		Render(listContent)

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

// Reset reset internal state to when a component isfirst initialized.
func (s *Section) Reset() tea.Cmd {
	s.isFilter = false
	cmd := s.activePanel().Close()
	s.SetSize(s.originalWidth, s.orginalHeight)
	return cmd
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

func (s *Section) deleteImageCmd() tea.Cmd {
	ctx := s.ctx
	svc := s.imageService
	items := s.list.Items()
	idx := s.list.Index()
	if idx < 0 || idx >= len(items) {
		return nil
	}

	dockerImage, ok := items[idx].(imageItem)
	if !ok {
		return nil
	}

	return func() tea.Msg {
		err := svc.Remove(ctx, dockerImage.ID(), true)

		return imageRemovedMsg{ID: dockerImage.ID(), Idx: idx, Error: err}
	}
}

func (s *Section) updateImagesCmd() tea.Cmd {
	ctx := s.ctx
	svc := s.imageService
	return func() tea.Msg {
		images, err := svc.List(ctx)
		if err != nil {
			return imagesLoadedMsg{error: err}
		}
		items := make([]list.Item, len(images))
		for idx, img := range images {
			items[idx] = imageItem{image: img}
		}
		return imagesLoadedMsg{items: items}
	}
}

func (s *Section) createContainerCmdAndRun() tea.Cmd {
	svc := s.containerService
	items := s.list.Items()
	idx := s.list.Index()
	if idx < 0 || idx >= len(items) {
		return nil
	}

	dockerImage, ok := items[idx].(imageItem)
	if !ok {
		return nil
	}
	s.loading = true
	ctx := s.ctx
	return func() tea.Msg {
		containerID, err := svc.Run(ctx, dockerImage.image)

		return containerRunMsg{containerID: containerID, error: err}
	}
}

func (s *Section) pruneImagesCmd() tea.Cmd {
	ctx, svc := s.ctx, s.imageService
	return func() tea.Msg {
		report, err := svc.Prune(ctx, client.PruneOptions{All: true})
		return imagesPrunedMsg{report: report, err: err}
	}
}

func (s *Section) confirmImagePrune() tea.Cmd {
	pruneCmd := s.pruneImagesCmd()
	return func() tea.Msg {
		return message.ShowConfirmationMsg{
			Title:     "Prune Images",
			Body:      "Remove all unused images (including non-dangling)?",
			OnConfirm: pruneCmd,
		}
	}
}

func (s *Section) confirmImageDelete() tea.Cmd {
	items := s.list.Items()
	idx := s.list.Index()
	if idx < 0 || idx >= len(items) {
		return nil
	}
	dockerImage, ok := items[idx].(imageItem)
	if !ok {
		return nil
	}
	deleteCmd := s.deleteImageCmd()
	return func() tea.Msg {
		return message.ShowConfirmationMsg{
			Title:     "Delete Image",
			Body:      fmt.Sprintf("Delete image %s?", helper.ShortID(dockerImage.ID())),
			OnConfirm: deleteCmd,
		}
	}
}

func (s *Section) activePanel() panel.Panel {
	return s.panels[s.activePanelIdx]
}

func (s *Section) removeItem(idx int) tea.Cmd {
	s.list.RemoveItem(idx)
	if len(s.list.Items()) > 0 {
		s.list.Select(min(idx, len(s.list.Items())-1))
	}
	return s.updateActivePanel()
}

func (s *Section) updateActivePanel() tea.Cmd {
	selected := s.list.SelectedItem()
	if selected == nil {
		return nil
	}
	item, ok := selected.(imageItem)
	if !ok {
		return nil
	}
	return s.activePanel().Init(item.ID())
}
