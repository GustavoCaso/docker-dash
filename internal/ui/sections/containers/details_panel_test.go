package containers

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
)

func newTestDetailsPanel() *detailsPanel {
	return NewDetailsPanel(context.Background(), client.NewMockClient().Containers()).(*detailsPanel)
}

func TestDetailsPanelInitReturnsCmd(t *testing.T) {
	dp := newTestDetailsPanel()
	cmd := dp.Init("abc123def456")
	if cmd == nil {
		t.Fatal("Init should return a non-nil command")
	}

	msg := cmd()

	if _, ok := msg.(detailsMsg); !ok {
		t.Fatalf("Init() cmd should return detailsMsg, got %T", msg)
	}
}

func TestDetailsPanelUpdateSetsContent(t *testing.T) {
	dp := newTestDetailsPanel()
	dp.SetSize(100, 100)

	cmd := dp.Init("abc123def456")
	msg := cmd()

	dm, ok := msg.(detailsMsg)
	if !ok {
		t.Fatalf("Init() cmd should return detailsMsg, got %T", msg)
	}

	updateCmd := dp.Update(dm)
	if updateCmd != nil {
		t.Errorf("Update should return nil on success, got %v", updateCmd)
	}
	if dp.View() == "" {
		t.Error("Update should set content")
	}
	if !strings.Contains(dp.View(), "nginx-proxy") {
		t.Errorf("content should contain container name, got: %q", dp.View())
	}
}

func TestDetailsPanelUpdateWithError(t *testing.T) {
	dp := newTestDetailsPanel()

	errTest := errors.New("test error")
	cmd := dp.Update(detailsMsg{err: errTest})
	if cmd == nil {
		t.Fatal("Update with error should return a command")
	}

	result := cmd()
	banner, ok := result.(message.ShowBannerMsg)
	if !ok {
		t.Fatalf("expected ShowBannerMsg, got %T", result)
	}
	if !banner.IsError {
		t.Error("banner should be an error")
	}
}
func TestDetailsPanelCloseResetsContent(t *testing.T) {
	dp := newTestDetailsPanel()
	dp.viewport.SetContent("hello")

	dp.Close()

	if dp.View() != "" {
		t.Errorf("Close should reset viewport, got %q", dp.View())
	}
}

func TestDetailsPanelCloseIsIdempotent(t *testing.T) {
	dp := newTestDetailsPanel()
	dp.Close()
	dp.Close() // must not panic
}

func TestDetailsPanelViewReturnsViewportContent(t *testing.T) {
	dp := newTestDetailsPanel()
	dp.SetSize(100, 30)
	dp.viewport.SetContent("hello")

	if !strings.Contains(dp.View(), "hello") {
		t.Errorf("View() = %q, want %q", dp.View(), "hello")
	}
}

func TestDetailsPanelSetSizeViewport(t *testing.T) {
	dp := newTestDetailsPanel()
	dp.SetSize(100, 30)
	if dp.viewport.Width != 100 {
		t.Errorf("viewport.Width = %d, want 100", dp.viewport.Width)
	}
	if dp.viewport.Height != 30 {
		t.Errorf("viewport.Height = %d, want 29", dp.viewport.Height)
	}
}
