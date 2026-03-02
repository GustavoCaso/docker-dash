package containers

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
)

func newTestDetailsPanel() *detailsPanel {
	return NewDetailsPanel(client.NewMockClient().Containers()).(*detailsPanel)
}

func TestDetailsPanelInitReturnsCmd(t *testing.T) {
	dp := newTestDetailsPanel()
	cmd := dp.Init("abc123def456")
	if cmd == nil {
		t.Fatal("Init should return a non-nil command")
	}
}

func TestDetailsPanelUpdateSetsContent(t *testing.T) {
	dp := newTestDetailsPanel()
	dp.width = 80

	msg := dp.Init("abc123def456")()
	cmd := dp.Update(msg)

	if cmd != nil {
		t.Errorf("Update should return nil on success, got %v", cmd)
	}
	if dp.content == "" {
		t.Error("Update should set content")
	}
	if !strings.Contains(dp.content, "nginx-proxy") {
		t.Errorf("content should contain container name, got: %q", dp.content)
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

func TestDetailsPanelUpdateIgnoresOtherMessages(t *testing.T) {
	dp := newTestDetailsPanel()
	dp.content = "existing"

	cmd := dp.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Errorf("Update should return nil for unhandled messages")
	}
	if dp.content != "existing" {
		t.Error("Update should not modify content for unhandled messages")
	}
}

func TestDetailsPanelCloseResetsContent(t *testing.T) {
	dp := newTestDetailsPanel()
	dp.content = "some content"

	dp.Close()

	if dp.content != "" {
		t.Errorf("Close should reset content, got %q", dp.content)
	}
}

func TestDetailsPanelCloseIsIdempotent(t *testing.T) {
	dp := newTestDetailsPanel()
	dp.Close()
	dp.Close() // must not panic
}

func TestDetailsPanelViewReturnsContent(t *testing.T) {
	dp := newTestDetailsPanel()
	dp.content = "hello"

	if dp.View() != "hello" {
		t.Errorf("View() = %q, want %q", dp.View(), "hello")
	}
}

func TestDetailsPanelSetSizeStoresWidth(t *testing.T) {
	dp := newTestDetailsPanel()
	dp.SetSize(100, 30)
	if dp.width != 100 {
		t.Errorf("width = %d, want 100", dp.width)
	}
}
