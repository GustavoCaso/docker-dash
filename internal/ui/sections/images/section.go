package images

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/config"
	"github.com/GustavoCaso/docker-dash/internal/ui/components/form"
	"github.com/GustavoCaso/docker-dash/internal/ui/components/panel"
	"github.com/GustavoCaso/docker-dash/internal/ui/helper"
	"github.com/GustavoCaso/docker-dash/internal/ui/keys"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
	"github.com/GustavoCaso/docker-dash/internal/ui/sections"
	"github.com/GustavoCaso/docker-dash/internal/ui/sections/base"
	"github.com/GustavoCaso/docker-dash/internal/ui/theme"
)

// imagesLoadedMsg is sent when images have been loaded asynchronously.
type imagesLoadedMsg struct {
	error  error
	images []client.Image
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

type imagePullMsg struct {
	image string
	err   error
}

// imageItem implements list.Item interface.
type imageItem struct {
	image     client.Image
	hasUpdate bool
}

func (i imageItem) ID() string    { return i.image.ID }
func (i imageItem) Title() string { return fmt.Sprintf("%s:%s", i.image.Repo, i.image.Tag) }
func (i imageItem) Description() string {
	stateIcon := theme.GetImageStatusIcon(i.image.Containers)
	stateStyle := theme.GetImageStatusStyle(i.image.Containers)
	state := stateStyle.Render(stateIcon)
	desc := state + " " + helper.FormatSize(i.image.Size)
	if i.hasUpdate {
		desc += " " + theme.UpdateAvailableStyle.Render(theme.UpdateAvailableIcon)
	}
	return desc
}
func (i imageItem) FilterValue() string { return i.image.Repo + ":" + i.image.Tag }

// Section wraps bubbles/list.
type Section struct {
	*base.Section
	ctx              context.Context
	cfg              config.UpdateCheckConfig
	imageService     client.ImageService
	containerService client.ContainerService
	updateChecker    *updateChecker
	imageUpdates     map[string]bool
	currentImages    []client.Image
}

// New creates a new image section.
func New(ctx context.Context, images []client.Image, client client.Client, cfg config.UpdateCheckConfig) *Section {
	items := make([]list.Item, len(images))
	for i, img := range images {
		items[i] = imageItem{image: img}
	}

	il := &Section{
		ctx:              ctx,
		cfg:              cfg,
		imageService:     client.Images(),
		containerService: client.Containers(),
		currentImages:    images,
		imageUpdates:     make(map[string]bool),
		Section: base.New(
			sections.ImagesSection,
			items,
			[]panel.Panel{NewLayersPanel(ctx, client.Images())},
		),
	}

	il.LoadingText = "Refreshing..."
	il.ActivePanelInitFn = func(item list.Item) string {
		ii, ok := item.(imageItem)
		if !ok {
			return ""
		}
		return ii.ID()
	}
	il.RefreshCmd = il.updateImagesCmd
	il.PruneCmd = il.confirmImagePrune
	il.HandleMsg = il.handleMsg
	il.HandleKey = il.handleKey

	return il
}

// Init overrides the base Init to also fire the update checker tick if enabled.
func (s *Section) Init() tea.Cmd {
	baseCmd := s.Section.Init()

	if s.cfg.Enabled {
		d, err := time.ParseDuration(s.cfg.Interval)
		errorMessage := ""
		if err != nil {
			errorMessage = fmt.Sprintf(
				"Invalid update check interval %q, skipping update check: %v",
				s.cfg.Interval,
				err.Error(),
			)
		}

		if err == nil && d <= 0 {
			errorMessage = fmt.Sprintf("Non-positive update check interval %q, skipping update check", s.cfg.Interval)
		}

		if errorMessage != "" {
			return tea.Batch(baseCmd, func() tea.Msg {
				return message.ShowBannerMsg{Message: errorMessage, IsError: true}
			})
		}
		s.updateChecker = &updateChecker{interval: d, svc: s.imageService}
	}

	if s.updateChecker != nil {
		return tea.Batch(baseCmd, s.updateChecker.tickCmd())
	}
	return baseCmd
}

func (s *Section) handleMsg(msg tea.Msg) base.UpdateResult {
	switch msg := msg.(type) {
	case updateCheckTickMsg:
		// tickCmd is only scheduled when updateChecker != nil, so this is always non-nil here.
		return base.UpdateResult{
			Cmd:     s.updateChecker.checkCmd(s.ctx, s.currentImages),
			Handled: true,
		}
	case imageUpdatesMsg:
		s.imageUpdates = msg.updates
		items := s.List.Items()
		for i, item := range items {
			ii, ok := item.(imageItem)
			if !ok {
				continue
			}
			ii.hasUpdate = s.imageUpdates[ii.image.ID]
			items[i] = ii
		}
		var cmd tea.Cmd
		if s.updateChecker != nil {
			cmd = s.updateChecker.tickCmd()
		}
		return base.UpdateResult{
			Cmd:     tea.Batch(s.List.SetItems(items), cmd),
			Handled: true,
		}
	case imagesLoadedMsg:
		log.Printf("[images] imagesLoadedMsg: count=%d", len(msg.images))
		if msg.error != nil {
			return base.UpdateResult{
				Cmd: func() tea.Msg {
					return message.ShowBannerMsg{
						Message: fmt.Sprintf("Error loading images: %s", msg.error.Error()),
						IsError: true,
					}
				},
				Handled:     true,
				StopSpinner: true,
			}
		}
		s.currentImages = msg.images
		items := make([]list.Item, len(msg.images))
		for idx, img := range msg.images {
			update, ok := s.imageUpdates[img.ID]
			if !ok {
				update = false
			}

			items[idx] = imageItem{image: img, hasUpdate: update}
		}
		return base.UpdateResult{Cmd: s.List.SetItems(items), Handled: true, StopSpinner: true}
	case imagesPrunedMsg:
		log.Printf(
			"[images] imagesPrunedMsg: deleted=%d spaceReclaimed=%d",
			msg.report.ItemsDeleted,
			msg.report.SpaceReclaimed,
		)
		if msg.err != nil {
			return base.UpdateResult{
				Cmd: func() tea.Msg {
					return message.ShowBannerMsg{
						Message: "Error pruning images: " + msg.err.Error(),
						IsError: true,
					}
				},
				Handled:     true,
				StopSpinner: true,
			}
		}
		summary := fmt.Sprintf(
			"Pruned %d images, reclaimed %s",
			msg.report.ItemsDeleted,
			helper.FormatSize(msg.report.SpaceReclaimed),
		)
		return base.UpdateResult{
			Cmd: tea.Batch(s.updateImagesCmd(), func() tea.Msg {
				return message.ShowBannerMsg{Message: summary, IsError: false}
			}),
			Handled: true,
		}
	case imageRemovedMsg:
		log.Printf("[images] imageRemovedMsg: imageID=%q err=%v", msg.ID, msg.Error)
		if msg.Error != nil {
			return base.UpdateResult{
				Cmd: func() tea.Msg {
					return message.ShowBannerMsg{
						Message: fmt.Sprintf("Error deleting image: %s", msg.Error.Error()),
						IsError: true,
					}
				},
				Handled:     true,
				StopSpinner: true,
			}
		}
		return base.UpdateResult{
			Cmd: tea.Batch(s.RemoveItemAndUpdatePanel(msg.Idx), func() tea.Msg {
				return message.ShowBannerMsg{
					Message: fmt.Sprintf("Image %s deleted", helper.ShortID(msg.ID)),
					IsError: false,
				}
			}),
			Handled:     true,
			StopSpinner: true,
		}
	case imagePullMsg:
		log.Printf("[images] imagePullMsg: err=%v", msg.err)
		if msg.err != nil {
			return base.UpdateResult{
				Cmd: func() tea.Msg {
					return message.ShowBannerMsg{
						Message: fmt.Sprintf("Error pulling image: %s", msg.err.Error()),
						IsError: true,
					}
				},
				Handled:     true,
				StopSpinner: true,
			}
		}
		pullMessage := fmt.Sprintf("Image %s Pulled", msg.image)
		return base.UpdateResult{
			Cmd: tea.Batch(s.updateImagesCmd(), func() tea.Msg {
				return message.ShowBannerMsg{Message: pullMessage, IsError: false}
			}),
			Handled: true,
		}
	case containerRunMsg:
		log.Printf("[images] containerRunMsg: containerID=%q err=%v", msg.containerID, msg.error)
		if msg.error != nil {
			return base.UpdateResult{
				Cmd: func() tea.Msg {
					return message.ShowBannerMsg{
						Message: msg.error.Error(),
						IsError: true,
					}
				},
				Handled:     true,
				StopSpinner: true,
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
		return base.UpdateResult{
			Cmd:         tea.Batch(banner, refreshComponents),
			Handled:     true,
			StopSpinner: true,
		}
	}
	return base.UpdateResult{}
}

func (s *Section) handleKey(msg tea.KeyMsg) base.UpdateResult {
	switch {
	case key.Matches(msg, keys.Keys.PullImage):
		f := pullImageForm()
		pullForm := form.New("Pull Image", f, func(finishForm *huh.Form) tea.Cmd {
			image := finishForm.GetString("image")
			platform := finishForm.GetString("platform")
			return s.WithSpinner(s.pullImageCmd(image, platform))
		})
		return base.UpdateResult{
			Cmd: func() tea.Msg {
				return message.ShowFormMsg{Form: pullForm}
			},
			Handled: true,
		}
	case key.Matches(msg, keys.Keys.PullImageUpdate):
		return s.pullUpdateCmd()
	case key.Matches(msg, keys.Keys.Delete):
		return base.UpdateResult{Cmd: s.confirmImageDelete(), Handled: true}
	case key.Matches(msg, keys.Keys.CreateAndRunContainer):
		return base.UpdateResult{Cmd: s.showRunContainerForm(), Handled: true}
	}
	return base.UpdateResult{}
}

func (s *Section) deleteImageCmd() tea.Cmd {
	ctx := s.ctx
	svc := s.imageService
	items := s.List.Items()
	idx := s.List.Index()
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

		return imagesLoadedMsg{images: images}
	}
}

func (s *Section) pullImageCmd(image, platform string) tea.Cmd {
	ctx := s.ctx
	svc := s.imageService

	return func() tea.Msg {
		err := svc.Pull(ctx, image, platform)

		return imagePullMsg{err: err, image: image}
	}
}

func (s *Section) pullUpdateCmd() base.UpdateResult {
	items := s.List.Items()
	idx := s.List.Index()
	if idx < 0 || idx >= len(items) {
		return base.UpdateResult{}
	}
	dockerImage, ok := items[idx].(imageItem)
	if !ok {
		return base.UpdateResult{}
	}
	if !dockerImage.hasUpdate {
		return base.UpdateResult{
			Cmd: func() tea.Msg {
				return message.ShowBannerMsg{
					Message: fmt.Sprintf("No update available for %s", dockerImage.Title()),
					IsError: false,
				}
			},
			Handled: true,
		}
	}
	image := fmt.Sprintf("%s:%s", dockerImage.image.Repo, dockerImage.image.Tag)
	platform := "" // auto-detect
	return base.UpdateResult{
		Cmd:     s.WithSpinner(s.pullImageCmd(image, platform)),
		Handled: true,
	}
}

func (s *Section) showRunContainerForm() tea.Cmd {
	items := s.List.Items()
	idx := s.List.Index()
	if idx < 0 || idx >= len(items) {
		return nil
	}

	dockerImage, ok := items[idx].(imageItem)
	if !ok {
		return nil
	}

	f := runContainerForm(dockerImage.image)
	runForm := form.New(
		fmt.Sprintf("Run Container from %s", dockerImage.Title()),
		f,
		func(finishForm *huh.Form) tea.Cmd {
			name := finishForm.GetString("name")
			portsRaw := finishForm.GetString("ports")
			envRaw := finishForm.GetString("env")

			opts := client.RunOptions{
				Name:  name,
				Ports: parseCSV(portsRaw),
				Env:   parseCSV(envRaw),
			}

			return s.createContainerCmdAndRun(dockerImage.image, opts)
		},
	)

	return func() tea.Msg {
		return message.ShowFormMsg{Form: runForm}
	}
}

func (s *Section) createContainerCmdAndRun(img client.Image, opts client.RunOptions) tea.Cmd {
	svc := s.containerService
	ctx := s.ctx
	return s.WithSpinner(func() tea.Msg {
		containerID, err := svc.Run(ctx, img, opts)
		return containerRunMsg{containerID: containerID, error: err}
	})
}

// parseCSV splits a comma-separated string into a trimmed slice, ignoring empty entries.
func parseCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			result = append(result, v)
		}
	}
	return result
}

func runContainerForm(img client.Image) *huh.Form {
	portsDesc := "Comma-separated host:container pairs, e.g. 8080:80,443:443"
	if exposed := exposedPortsList(img); exposed != "" {
		portsDesc = fmt.Sprintf("Image exposes: %s — map as host:container, e.g. 8080:80", exposed)
	}

	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Key("name").
				Title("Container Name").
				Description("Optional. Leave blank to let Docker generate a name."),

			huh.NewInput().
				Key("ports").
				Title("Port Mappings").
				Description(portsDesc).
				Validate(validatePorts),

			huh.NewInput().
				Key("env").
				Title("Environment Variables").
				Description("Comma-separated KEY=VAL pairs, e.g. DEBUG=1").
				Validate(validateEnv),
		),
	)
}

// validatePorts checks that each comma-separated entry is in "hostPort:containerPort" format
// with valid port numbers (1–65535). An empty value is accepted (all fields are optional).
func validatePorts(s string) error {
	for _, entry := range parseCSV(s) {
		parts := strings.SplitN(entry, ":", 2) //nolint:mnd // splitting "hostPort:containerPort"
		if len(parts) != 2 {                   //nolint:mnd // need exactly host:container
			return fmt.Errorf("invalid port mapping %q: expected host:container", entry)
		}
		if err := validatePort(parts[0], entry); err != nil {
			return err
		}
		// Container port may include a protocol suffix like "80/tcp"; strip it before validating.
		containerPort := strings.SplitN(parts[1], "/", 2)[0] //nolint:mnd // strip optional /proto
		if err := validatePort(containerPort, entry); err != nil {
			return err
		}
	}
	return nil
}

const maxPort = 65535

func validatePort(portStr, entry string) error {
	n, err := strconv.Atoi(portStr)
	if err != nil || n < 1 || n > maxPort {
		return fmt.Errorf("invalid port in %q: %q must be a number between 1 and 65535", entry, portStr)
	}
	return nil
}

// validateEnv checks that each comma-separated entry is in "KEY=VALUE" format,
// where KEY is a non-empty identifier. An empty value is accepted.
func validateEnv(s string) error {
	for _, entry := range parseCSV(s) {
		parts := strings.SplitN(entry, "=", 2) //nolint:mnd // splitting "KEY=VALUE"
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
			return fmt.Errorf("invalid env var %q: expected KEY=VALUE", entry)
		}
	}
	return nil
}

// exposedPortsList returns a human-readable list of ports the image exposes,
// e.g. "80/tcp, 443/tcp". Returns empty string if none are defined.
func exposedPortsList(img client.Image) string {
	if img.Config == nil || len(img.Config.ExposedPorts) == 0 {
		return ""
	}
	ports := make([]string, 0, len(img.Config.ExposedPorts))
	for port := range img.Config.ExposedPorts {
		ports = append(ports, port)
	}
	slices.Sort(ports)
	return strings.Join(ports, ", ")
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
			OnConfirm: s.WithSpinner(pruneCmd),
		}
	}
}

func (s *Section) confirmImageDelete() tea.Cmd {
	items := s.List.Items()
	idx := s.List.Index()
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
			OnConfirm: s.WithSpinner(deleteCmd),
		}
	}
}

func pullImageForm() *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Key("image").
				Title("Image").
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return errors.New("image name cannot be empty")
					}
					return nil
				}),

			huh.NewSelect[string]().
				Key("platform").
				Title("Platform").
				Options(
					huh.NewOption("auto", ""),
					huh.NewOption("linux/amd64", "linux/amd64"),
					huh.NewOption("linux/arm64", "linux/arm64"),
					huh.NewOption("linux/arm/v7", "linux/arm/v7"),
					huh.NewOption("linux/386", "linux/386"),
					huh.NewOption("windows/amd64", "windows/amd64"),
				),
		),
	)
}
