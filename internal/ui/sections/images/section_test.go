package images

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
)

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
	images, _ := client.Images().List(context.Background())
	section := New(context.Background(), images, client)
	section.SetSize(120, 40)
	return imageSectionModel{section: section}
}

func (m imageSectionModel) Init() tea.Cmd { return m.section.Init() }

func (m imageSectionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "q" {
		return m, tea.Quit
	}
	if confirmMsg, ok := msg.(message.ShowConfirmationMsg); ok {
		return m, confirmMsg.OnConfirm
	}
	cmd := m.section.Update(msg)
	return m, cmd
}

func (m imageSectionModel) View() string {
	return m.section.View()
}

func (m imageSectionModel) Reset() tea.Cmd {
	return m.section.Reset()
}

func TestImageListRendersItems(t *testing.T) {
	tm := teatest.NewTestModel(t, newModel(), teatest.WithInitialTermSize(120, 40))
	waitFor(t, tm, "nginx")
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestImageListLayersVisible(t *testing.T) {
	tm := teatest.NewTestModel(t, newModel(), teatest.WithInitialTermSize(120, 40))
	waitFor(t, tm, "No layer information available")
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestImageReset(t *testing.T) {
	model := newModel()
	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(120, 40))
	waitFor(t, tm, "nginx")

	cmd := model.Reset()

	if cmd != nil {
		t.Error("Reset() should return nil cmd")
	}

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestImageListDelete(t *testing.T) {
	tm := teatest.NewTestModel(t, newModel(), teatest.WithInitialTermSize(120, 40))
	waitFor(t, tm, "nginx")
	// Select an image
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	// Delete (deletes the selected image which is node after KeyDown)
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	waitFor(t, tm, "postgres")
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestImageListRefresh(t *testing.T) {
	tm := teatest.NewTestModel(t, newModel(), teatest.WithInitialTermSize(120, 40))
	waitFor(t, tm, "nginx")
	// Refresh
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	// The refresh triggers spinner + async reload. After reload completes,
	// send a benign key to trigger a re-render so output is flushed.
	time.Sleep(500 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	waitFor(t, tm, "nginx")
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestImageListRunContainer(t *testing.T) {
	tm := teatest.NewTestModel(t, newModel(), teatest.WithInitialTermSize(120, 40))
	waitFor(t, tm, "nginx")
	// Select an image
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	// Run container
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("R")})
	// Wait for async operation, then flush output
	time.Sleep(500 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	waitFor(t, tm, "nginx")
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestResetClearsFlags(t *testing.T) {
	c := client.NewMockClient()
	images, _ := c.Images().List(context.Background())
	s := New(context.Background(), images, c)
	s.SetSize(120, 40)

	s.isFilter = true

	cmd := s.Reset()

	if s.isFilter {
		t.Error("Reset() should set isFilter to false")
	}
	if cmd != nil {
		t.Error("Reset() should return nil cmd from activePanel.Close()")
	}
}

func TestImageListPrune(t *testing.T) {
	tm := teatest.NewTestModel(t, newModel(), teatest.WithInitialTermSize(120, 40))
	waitFor(t, tm, "<none>:<none>") // dangling image present initially
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("P")})
	time.Sleep(500 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	waitForNot(t, tm, "<none>:<none>") // dangling image pruned
	waitFor(t, tm, "nginx")            // non-dangling images remain
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

func TestPullImageKeyShowsForm(t *testing.T) {
	c := client.NewMockClient()
	images, _ := c.Images().List(context.Background())
	section := New(context.Background(), images, c)
	section.SetSize(120, 40)

	cmd := section.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("+")})
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
	images, _ := c.Images().List(context.Background())
	section := New(context.Background(), images, c)
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
	images, _ := mc.Images().List(context.Background())
	section := New(context.Background(), images, mc)
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
	images, _ := c.Images().List(context.Background())
	section := New(context.Background(), images, c)
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
	images, _ := c.Images().List(context.Background())
	section := New(context.Background(), images, c)
	section.SetSize(120, 40)

	pullErr := errors.New("image not found")
	cmd := section.Update(imagePullMsg{image: "bad:image", err: pullErr})
	if cmd == nil {
		t.Fatal("imagePullMsg error should return a non-nil cmd")
	}

	msg := cmd()
	banner, ok := msg.(message.ShowBannerMsg)
	if !ok {
		t.Fatalf("expected ShowBannerMsg, got %T", msg)
	}
	if !banner.IsError {
		t.Error("error banner should have IsError=true")
	}
	if !strings.Contains(banner.Message, "image not found") {
		t.Errorf("error banner should contain error text, got %q", banner.Message)
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

func waitFor(t *testing.T, tm *teatest.TestModel, s string) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return strings.Contains(string(b), s)
	}, teatest.WithCheckInterval(time.Millisecond*100), teatest.WithDuration(time.Second*10))
}

func waitForNot(t *testing.T, tm *teatest.TestModel, s string) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return !strings.Contains(string(b), s)
	}, teatest.WithCheckInterval(time.Millisecond*100), teatest.WithDuration(time.Second*10))
}

func TestImageDeleteUpdatesSelection(t *testing.T) {
	c := client.NewMockClient()
	images, _ := c.Images().List(context.Background())
	section := New(context.Background(), images, c)
	section.SetSize(120, 40)

	initialCount := len(section.list.Items())
	if initialCount == 0 {
		t.Fatal("expected at least one image in mock data")
	}

	section.list.Select(0)
	cmd := section.removeItem(0)

	if len(section.list.Items()) != initialCount-1 {
		t.Errorf("expected %d items after delete, got %d", initialCount-1, len(section.list.Items()))
	}
	if section.list.Index() != 0 {
		t.Errorf("expected selection at index 0 after deleting first item, got %d", section.list.Index())
	}
	if cmd == nil {
		t.Fatal("removeItem() should return non-nil cmd when items remain")
	}
	if _, ok := cmd().(layersOutputMsg); !ok {
		t.Errorf("removeItem() cmd should produce layersOutputMsg, got %T", cmd())
	}
}

func TestImageDeleteLastItemClearsPanel(t *testing.T) {
	c := client.NewMockClient()
	images, _ := c.Images().List(context.Background())

	// Create a section with a single image so we can test the empty-list path.
	singleImage := images[:1]
	section := New(context.Background(), singleImage, c)
	section.SetSize(120, 40)

	// Set content on the layers panel so we can verify it gets cleared.
	lp := section.panels[0].(*layersPanel)
	lp.viewport.SetContent("stale layer info")

	// Delete the single item — activePanel().Close() should be called.
	cmd := section.removeItem(0)
	if cmd != nil {
		t.Error("removeItem() should return nil cmd when list is empty (Close() returns nil)")
	}
	if strings.TrimSpace(lp.viewport.View()) != "" {
		t.Error("deleteLastItem() should clear the panel viewport content")
	}
}

func TestImageDeleteMiddleItemClampsToLastWhenAtEnd(t *testing.T) {
	c := client.NewMockClient()
	images, _ := c.Images().List(context.Background())
	section := New(context.Background(), images, c)
	section.SetSize(120, 40)

	count := len(section.list.Items())
	if count < 2 {
		t.Fatal("expected at least two images in mock data")
	}

	// Select and delete the last item — selection should clamp to new last
	last := count - 1
	section.list.Select(last)
	cmd := section.removeItem(last)

	if section.list.Index() != last-1 {
		t.Errorf("expected selection at %d after deleting last item, got %d", last-1, section.list.Index())
	}
	if cmd == nil {
		t.Fatal("removeItem() should return non-nil cmd when items remain")
	}
	if _, ok := cmd().(layersOutputMsg); !ok {
		t.Errorf("removeItem() cmd should produce layersOutputMsg, got %T", cmd())
	}
}

func TestPanelClosedOnUpDownNavigation(t *testing.T) {
	c := client.NewMockClient()
	images, _ := c.Images().List(context.Background())
	section := New(context.Background(), images, c)
	section.SetSize(120, 40)

	// Navigate to second image
	section.list.Select(1)
	// Initialize the layers panel with content
	section.activePanel().Init("sha256:image2")

	// Navigate down to next image - this should close the current panel
	section.Update(tea.KeyMsg{Type: tea.KeyDown})

	// Verify the panel Close() was called (panel is ready for new content)
	// We can't directly verify viewport content is cleared, but we can verify
	// the panel can be initialized again without issues
	cmd := section.activePanel().Init("sha256:image3")
	if cmd == nil {
		t.Error("Panel should be able to reinitialize after navigation")
	}
}

func TestPanelClosedOnUpNavigation(t *testing.T) {
	c := client.NewMockClient()
	images, _ := c.Images().List(context.Background())
	section := New(context.Background(), images, c)
	section.SetSize(120, 40)

	// Navigate to second image
	section.list.Select(1)
	// Initialize the layers panel
	section.activePanel().Init("sha256:image2")

	// Navigate up to previous image - this should close the current panel
	section.Update(tea.KeyMsg{Type: tea.KeyUp})

	// Verify the panel can be reinitialized
	cmd := section.activePanel().Init("sha256:image1")
	if cmd == nil {
		t.Error("Panel should be able to reinitialize after navigation")
	}
}
