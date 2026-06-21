package images

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/config"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
)

func getImageIDs(m imageSectionModel) []string {
	var ids []string
	for _, item := range m.section.List.Items() {
		if ii, ok := item.(imageItem); ok {
			ids = append(ids, ii.image.ID)
		}
	}
	return ids
}

// errMockImageService wraps an ImageService and overrides Pull to return an error.
type errMockImageService struct {
	client.ImageService
	pullErr error
}

func (s *errMockImageService) Pull(_ context.Context, _, _ string) error {
	return s.pullErr
}

type imageSectionModel struct {
	section *Section
}

func newModel() imageSectionModel {
	client := client.NewMockClient()
	section := New(context.Background(), client, config.UpdateCheckConfig{})
	section.SetSize(120, 40)
	return imageSectionModel{section: section}
}

func (m imageSectionModel) Init() tea.Cmd { return m.section.Init() }

func (m imageSectionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok && keyMsg.String() == "q" {
		return m, tea.Quit
	}
	if confirmMsg, ok := msg.(message.ShowConfirmationMsg); ok {
		return m, confirmMsg.OnConfirm
	}
	cmd := m.section.Update(msg)
	return m, cmd
}

func (m imageSectionModel) View() tea.View {
	return tea.NewView(m.section.View())
}

func (m imageSectionModel) Reset() tea.Cmd {
	return m.section.Reset()
}

func TestInitShowsBannerForInvalidUpdateCheckInterval(t *testing.T) {
	t.Parallel()

	c := client.NewMockClient()
	section := New(context.Background(), c, config.UpdateCheckConfig{
		Enabled:  true,
		Interval: "not-a-duration",
	})
	section.SetSize(120, 40)

	msgs := runBatch(section.Init())

	for _, msg := range msgs {
		banner, ok := msg.(message.ShowBannerMsg)
		if !ok {
			continue
		}

		if !banner.IsError {
			t.Fatal("expected invalid interval banner to be marked as an error")
		}

		if !strings.Contains(banner.Message, `Invalid update check interval "not-a-duration"`) {
			t.Fatalf("unexpected banner message: %q", banner.Message)
		}

		return
	}

	t.Fatal("expected ShowBannerMsg for invalid update check interval")
}

func TestInitShowsBannerForNonPositiveUpdateCheckInterval(t *testing.T) {
	t.Parallel()

	c := client.NewMockClient()
	section := New(context.Background(), c, config.UpdateCheckConfig{
		Enabled:  true,
		Interval: "0s",
	})
	section.SetSize(120, 40)

	msgs := runBatch(section.Init())

	for _, msg := range msgs {
		banner, ok := msg.(message.ShowBannerMsg)
		if !ok {
			continue
		}

		if !banner.IsError {
			t.Fatal("expected non-positive interval banner to be marked as an error")
		}

		if !strings.Contains(banner.Message, `Non-positive update check interval "0s"`) {
			t.Fatalf("unexpected banner message: %q", banner.Message)
		}

		return
	}

	t.Fatal("expected ShowBannerMsg for non-positive update check interval")
}

func TestImageReset(t *testing.T) {
	model := newModel()
	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(120, 40))

	cmd := model.Reset()

	if cmd != nil {
		t.Error("Reset() should return nil cmd")
	}

	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestImageListDelete(t *testing.T) {
	tm := teatest.NewTestModel(t, newModel(), teatest.WithInitialTermSize(120, 40))
	time.Sleep(500 * time.Millisecond)
	// Select second image (node:18-alpine, sha256:node456)
	tm.Send(tea.KeyPressMsg{Code: tea.KeyDown})
	// Delete selected image
	tm.Send(tea.KeyPressMsg{Code: 'd', Text: "d"})
	time.Sleep(500 * time.Millisecond)
	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))

	fm := tm.FinalModel(t, teatest.WithFinalTimeout(time.Second))

	m, ok := fm.(imageSectionModel)
	if !ok {
		t.Fatal("unexpected model type")
	}

	ids := getImageIDs(m)
	for _, id := range ids {
		if id == "sha256:node456" {
			t.Fatal("expected node:18-alpine to be deleted, but found in list")
		}
	}
}

func TestRunContainerKeyShowsForm(t *testing.T) {
	c := client.NewMockClient()
	section := New(context.Background(), c, config.UpdateCheckConfig{})
	section.SetSize(120, 40)
	section.Update(section.RefreshCmd()())

	cmd := section.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	if cmd == nil {
		t.Fatal("pressing 'c' should return a non-nil cmd")
	}

	msg := cmd()
	formMsg, ok := msg.(message.ShowFormMsg)
	if !ok {
		t.Fatalf("expected ShowFormMsg, got %T", msg)
	}
	if formMsg.Form == nil {
		t.Error("ShowFormMsg.Form should not be nil")
	}
}

func TestImageListPrune(t *testing.T) {
	tm := teatest.NewTestModel(t, newModel(), teatest.WithInitialTermSize(120, 40))
	time.Sleep(500 * time.Millisecond)
	tm.Send(tea.KeyPressMsg{Code: 'P', Text: "P"})
	time.Sleep(500 * time.Millisecond)
	tm.Send(tea.KeyPressMsg{Code: tea.KeyDown})
	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))

	fm := tm.FinalModel(t, teatest.WithFinalTimeout(time.Second))

	m, ok := fm.(imageSectionModel)
	if !ok {
		t.Fatal("unexpected model type")
	}

	danglingIDs := []string{"sha256:dangling001", "sha256:dangling002"}
	ids := getImageIDs(m)

	for _, id := range ids {
		if slices.Contains(danglingIDs, id) {
			t.Fatalf("expected dangling image %s to be pruned, but found in list", id)
		}
	}

	// non-dangling images remain
	if !slices.Contains(ids, "sha256:nginx123") {
		t.Fatal("expected nginx image to remain after prune, but not found in list")
	}
}

func TestPullImageKeyShowsForm(t *testing.T) {
	c := client.NewMockClient()
	section := New(context.Background(), c, config.UpdateCheckConfig{})
	section.SetSize(120, 40)

	cmd := section.Update(tea.KeyPressMsg{Code: '+', Text: "+"})
	if cmd == nil {
		t.Fatal("pressing '+' should return a non-nil cmd")
	}

	msg := cmd()
	formMsg, ok := msg.(message.ShowFormMsg)
	if !ok {
		t.Fatalf("expected ShowFormMsg, got %T", msg)
	}
	if formMsg.Form == nil {
		t.Error("ShowFormMsg.Form should not be nil")
	}
}

func TestPullImageCmdSuccess(t *testing.T) {
	c := client.NewMockClient()
	section := New(context.Background(), c, config.UpdateCheckConfig{})
	section.SetSize(120, 40)

	cmd := section.pullImageCmd("nginx:latest", "")
	if cmd == nil {
		t.Fatal("pullImageCmd should return a non-nil tea.Cmd")
	}

	msg := cmd()
	pullMsg, ok := msg.(imagePullMsg)
	if !ok {
		t.Fatalf("expected imagePullMsg, got %T", msg)
	}
	if pullMsg.err != nil {
		t.Errorf("expected no error, got %v", pullMsg.err)
	}
	if pullMsg.image != "nginx:latest" {
		t.Errorf("expected image %q, got %q", "nginx:latest", pullMsg.image)
	}
}

func TestPullImageCmdError(t *testing.T) {
	mc := client.NewMockClient()
	errImageSvc := &errMockImageService{
		ImageService: mc.Images(),
		pullErr:      errors.New("pull failed"),
	}
	section := New(context.Background(), mc, config.UpdateCheckConfig{})
	section.imageService = errImageSvc
	section.SetSize(120, 40)

	cmd := section.pullImageCmd("bad:image", "")
	msg := cmd()

	pullMsg, ok := msg.(imagePullMsg)
	if !ok {
		t.Fatalf("expected imagePullMsg, got %T", msg)
	}
	if pullMsg.err == nil {
		t.Error("expected an error from pullImageCmd")
	}
}

func TestPullImageMsgSuccess_ShowsBanner(t *testing.T) {
	c := client.NewMockClient()
	section := New(context.Background(), c, config.UpdateCheckConfig{})
	section.SetSize(120, 40)

	cmd := section.Update(imagePullMsg{image: "nginx:latest", err: nil})
	if cmd == nil {
		t.Fatal("imagePullMsg success should return a non-nil cmd")
	}

	// Run the batch to find the ShowBannerMsg.
	found := false
	msgs := runBatch(cmd)
	for _, m := range msgs {
		if banner, ok := m.(message.ShowBannerMsg); ok {
			if !strings.Contains(banner.Message, "nginx:latest") {
				t.Errorf("banner message should contain image name, got %q", banner.Message)
			}
			if banner.IsError {
				t.Error("success banner should not be an error")
			}
			found = true
		}
	}
	if !found {
		t.Error("expected ShowBannerMsg in batch result")
	}
}

func TestPullImageMsgError_ShowsErrorBanner(t *testing.T) {
	c := client.NewMockClient()
	section := New(context.Background(), c, config.UpdateCheckConfig{})
	section.SetSize(120, 40)

	pullErr := errors.New("image not found")
	cmd := section.Update(imagePullMsg{image: "bad:image", err: pullErr})
	if cmd == nil {
		t.Fatal("imagePullMsg error should return a non-nil cmd")
	}

	found := false
	msgs := runBatch(cmd)
	for _, m := range msgs {
		banner, ok := m.(message.ShowBannerMsg)
		if !ok {
			continue
		}
		if !banner.IsError {
			t.Error("error banner should have IsError=true")
		}
		if !strings.Contains(banner.Message, "image not found") {
			t.Errorf("error banner should contain error text, got %q", banner.Message)
		}
		found = true
	}
	if !found {
		t.Fatal("expected ShowBannerMsg in batch result")
	}
}

// runBatch executes a tea.Cmd and collects all messages, recursing into tea.BatchMsg.
func runBatch(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var msgs []tea.Msg
		for _, c := range batch {
			msgs = append(msgs, runBatch(c)...)
		}
		return msgs
	}
	return []tea.Msg{msg}
}

func TestValidatePorts(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"", false},
		{"8080:80", false},
		{"8080:80,443:443", false},
		{"  8080:80 , 443:443 ", false},
		{"8080:80/tcp", false}, // container port with protocol suffix
		{"nocodon", true},      // missing colon
		{"abc:80", true},       // non-numeric host port
		{"8080:abc", true},     // non-numeric container port
		{"0:80", true},         // port out of range (0)
		{"8080:65536", true},   // port out of range (>65535)
		{"8080:", true},        // empty container port
		{":80", true},          // empty host port
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			err := validatePorts(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePorts(%q) error=%v, wantErr=%v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateEnv(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"", false},
		{"KEY=VALUE", false},
		{"KEY=VALUE,FOO=BAR", false},
		{"KEY=", false},          // empty value is allowed
		{"KEY=VAL=EXTRA", false}, // value may contain '='
		{"  KEY=VAL , FOO=1 ", false},
		{"NOEQUALS", true}, // missing '='
		{"=VALUE", true},   // empty key
		{"  =VALUE", true}, // whitespace-only key
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			err := validateEnv(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateEnv(%q) error=%v, wantErr=%v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestImageItemDescriptionShowsUpdateIcon(t *testing.T) {
	img := client.Image{ID: "sha256:abc", Repo: "nginx", Tag: "latest"}
	withUpdate := imageItem{image: img, hasUpdate: true}
	withoutUpdate := imageItem{image: img, hasUpdate: false}

	if !strings.Contains(withUpdate.Description(), "⬆") {
		t.Errorf("Description() with hasUpdate=true should contain update icon, got: %q", withUpdate.Description())
	}
	if strings.Contains(withoutUpdate.Description(), "⬆") {
		t.Errorf(
			"Description() with hasUpdate=false should not contain update icon, got: %q",
			withoutUpdate.Description(),
		)
	}
}

func TestImageUpdatesMsg_UpdatesListItems(t *testing.T) {
	c := client.NewMockClient()
	section := New(context.Background(), c, config.UpdateCheckConfig{})
	section.SetSize(120, 40)

	// Load images by running the RefreshCmd synchronously
	cmd := section.RefreshCmd()
	msg := cmd()
	section.Update(msg)

	// Find the nginx image ID (mock has nginx:latest first)
	var nginxID string
	for _, img := range section.currentImages {
		if img.Repo == "nginx" && img.Tag == "latest" {
			nginxID = img.ID
			break
		}
	}
	if nginxID == "" {
		t.Fatal("nginx:latest not found in mock images")
	}

	// Dispatch imageUpdatesMsg marking nginx as having an update
	section.Update(imageUpdatesMsg{updates: map[string]bool{nginxID: true}})

	// Verify the list item for nginx has hasUpdate=true
	found := false
	for _, it := range section.List.Items() {
		ii, ok := it.(imageItem)
		if !ok {
			continue
		}
		if ii.image.ID == nginxID {
			found = true
			if !ii.hasUpdate {
				t.Errorf("expected hasUpdate=true for nginx after imageUpdatesMsg, got false")
			}
			break
		}
	}
	if !found {
		t.Error("nginx image not found in list after imageUpdatesMsg")
	}
}

func TestPullUpdateCmd_NoUpdateShowsBanner(t *testing.T) {
	c := client.NewMockClient()
	section := New(context.Background(), c, config.UpdateCheckConfig{})
	section.SetSize(120, 40)
	section.Update(section.RefreshCmd()())

	// Default: no imageUpdatesMsg sent, so hasUpdate=false for all items.
	cmd := section.Update(tea.KeyPressMsg{Code: 'u', Text: "u"})
	if cmd == nil {
		t.Fatal("pressing 'u' with no update should return a non-nil cmd")
	}
	msg := cmd()
	banner, ok := msg.(message.ShowBannerMsg)
	if !ok {
		t.Fatalf("expected ShowBannerMsg, got %T", msg)
	}
	if banner.IsError {
		t.Error("'No update available' banner should not be an error")
	}
	if !strings.Contains(banner.Message, "No update available") {
		t.Errorf("expected 'No update available' in banner, got %q", banner.Message)
	}
}

func TestPullUpdateCmd_WithUpdate_FiresPull(t *testing.T) {
	c := client.NewMockClient()
	section := New(context.Background(), c, config.UpdateCheckConfig{})
	section.SetSize(120, 40)

	// Load images synchronously first
	loadCmd := section.RefreshCmd()
	section.Update(loadCmd())

	// Mark first image as having an update.
	section.Update(imageUpdatesMsg{updates: map[string]bool{section.currentImages[0].ID: true}})

	cmd := section.Update(tea.KeyPressMsg{Code: 'u', Text: "u"})
	if cmd == nil {
		t.Fatal("pressing 'u' with an available update should return a non-nil cmd")
	}
	msgs := runBatch(cmd)
	found := false
	for _, m := range msgs {
		if _, ok := m.(imagePullMsg); ok {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected imagePullMsg in batch result when update is available")
	}
}

func TestImagesLoadedMsgError(t *testing.T) {
	c := client.NewMockClient()
	section := New(context.Background(), c, config.UpdateCheckConfig{})
	section.SetSize(120, 40)

	// imagesLoadedMsg has an `err` field — check the field name in section.go
	cmd := section.Update(imagesLoadedMsg{error: errors.New("connection refused")})
	if cmd == nil {
		t.Fatal("expected a non-nil cmd for imagesLoadedMsg error")
	}
	msgs := runBatch(cmd)
	found := false
	for _, m := range msgs {
		banner, ok := m.(message.ShowBannerMsg)
		if !ok {
			continue
		}
		if !banner.IsError {
			t.Error("expected IsError=true for load error")
		}
		found = true
	}
	if !found {
		t.Fatal("expected ShowBannerMsg for imagesLoadedMsg error")
	}
}

func TestImagesLoadedMsgCallsUpdateItems(t *testing.T) {
	c := client.NewMockClient()
	section := New(context.Background(), c, config.UpdateCheckConfig{})
	section.SetSize(120, 40)

	if len(section.List.Items()) != 0 {
		t.Fatal("expected empty list before loading")
	}

	loadedMsg := section.RefreshCmd()()
	cmd := section.Update(loadedMsg)

	if len(section.List.Items()) == 0 {
		t.Fatal("UpdateItems should populate the list after imagesLoadedMsg")
	}
	if cmd == nil {
		t.Error("Update should return a non-nil cmd after imagesLoadedMsg")
	}
}

func TestImagesLoadedMsgEmptyCallsUpdateItemsReset(t *testing.T) {
	c := client.NewMockClient()
	section := New(context.Background(), c, config.UpdateCheckConfig{})
	section.SetSize(120, 40)

	section.Update(section.RefreshCmd()())
	section.Update(tea.KeyPressMsg{Code: '/', Text: "/"})

	cmd := section.Update(imagesLoadedMsg{images: []client.Image{}})

	if len(section.List.Items()) != 0 {
		t.Errorf("expected 0 items after empty imagesLoadedMsg, got %d", len(section.List.Items()))
	}
	if section.IsFilter() {
		t.Error("Reset via UpdateItems should clear isFilter")
	}
	if cmd == nil {
		t.Error("Update should return a non-nil cmd (SetItems) after empty imagesLoadedMsg")
	}
}

func TestPanelClosedOnUpNavigation(t *testing.T) {
	c := client.NewMockClient()
	section := New(context.Background(), c, config.UpdateCheckConfig{})
	section.SetSize(120, 40)

	// Navigate to second image
	section.List.Select(1)
	// Initialize the layers panel
	section.ActivePanel().Init(imageItem{image: client.Image{ID: "sha256:image2"}})

	// Navigate up to previous image - this should close the current panel
	section.Update(tea.KeyPressMsg{Code: tea.KeyUp})

	// Verify the panel can be reinitialized
	cmd := section.ActivePanel().Init(imageItem{image: client.Image{ID: "sha256:image1"}})
	if cmd == nil {
		t.Error("Panel should be able to reinitialize after navigation")
	}
}
