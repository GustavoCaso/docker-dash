package panel

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
)

func newTestFileTreePanel() *filesPanel {
	return NewFilesPanel(context.Background(), "containers", client.NewMockClient().Containers()).(*filesPanel)
}

func TestFileTreePanelInitFetchesTree(t *testing.T) {
	p := newTestFileTreePanel()
	cmd := p.Init("abc123def456")
	if cmd == nil {
		t.Fatal("Init() returned nil cmd")
	}
	if !p.loading {
		t.Error("Init() Must set loading state")
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("Init() cmd returned %T, want tea.BatchMsg", msg)
	}
	containersTreeLoaded := false
	tickMsg := false
	extendCmd := false

	for _, cmd := range batch {
		msg := cmd()
		switch msg.(type) {
		case fileNodeLoadedMsg:
			containersTreeLoaded = true
		case message.AddContextualKeyBindingsMsg:
			extendCmd = true
		case message.ShowSpinnerMsg:
			tickMsg = true
		}
	}

	if !containersTreeLoaded {
		t.Fatal("Init() not returned fileNodeLoadedMsg msg")
	}

	if !extendCmd {
		t.Fatal("Init() not returned AddContextualKeyBindingsMsg msg")
	}

	if !tickMsg {
		t.Fatal("Init() not returned ShowSpinnerMsg")
	}
}

func TestFileTreePanelUpdateSetsContent(t *testing.T) {
	p := newTestFileTreePanel()
	p.SetSize(80, 40)

	cmd := p.Init("abc123def456")
	msg := cmd()

	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("Init() cmd returned %T, want tea.BatchMsg", msg)
	}

	for _, cmd := range batch {
		// Update returns nil on success (no follow-up cmd needed)
		p.Update(cmd())
	}

	if p.loading {
		t.Error("Update() Must set loading state to false. Got true")
	}

	// MockClient.FileTree returns an empty tree — just ensure no panic and content is set.
	// An empty tree still renders as some string (possibly empty); what matters is no panic.
	_ = p.View()
}

func TestFileTreePanelUpdateWithError(t *testing.T) {
	p := newTestFileTreePanel()
	p.SetSize(80, 40)

	errMsg := fileNodeLoadedMsg{err: errors.New("fetch failed")}
	cmd := p.Update(errMsg)

	if cmd == nil {
		t.Fatal("Update with error should return banner cmd")
	}
	msgs := runBatch(cmd)
	foundBanner := false
	for _, result := range msgs {
		bannerMsg, ok := result.(message.ShowBannerMsg)
		if !ok {
			continue
		}
		if !bannerMsg.IsError {
			t.Error("ShowBannerMsg.IsError should be true")
		}
		foundBanner = true
	}
	if !foundBanner {
		t.Fatal("expected ShowBannerMsg in batch result")
	}
}

func TestFileTreePanelCloseResets(t *testing.T) {
	p := newTestFileTreePanel()

	p.Close()

	if p.loading {
		t.Error("Close() Must set loading state to false. Got true")
	}

	if p.View() != "" {
		t.Errorf("Close() should clear View, got %q", p.View())
	}
}

func TestFileTreePanelCloseIsIdempotent(t *testing.T) {
	p := newTestFileTreePanel()
	p.Close()
	p.Close() // must not panic
}

func TestFileTreePanelViewReturnsViewPort(t *testing.T) {
	p := newTestFileTreePanel()
	p.SetSize(80, 40)
	p.visible = []*client.FileNode{
		{
			Name:  "test",
			IsDir: true,
			Depth: 2,
		},
	}

	if !strings.Contains(p.View(), "▼ test/") {
		t.Errorf("View() = %q, want to contain '▼ test/'", p.View())
	}
}

func TestFileTreePanelViewReturnsLoadingState(t *testing.T) {
	p := newTestFileTreePanel()
	p.SetSize(80, 40)
	p.loading = true

	if p.View() != "" {
		t.Errorf("View() = %q, want empty view while app spinner is active", p.View())
	}
}

func TestFileTreePanelCloseCancelsSpinner(t *testing.T) {
	p := newTestFileTreePanel()
	p.loading = true
	p.requestID = 2

	cmd := p.Close()
	msgs := runBatch(cmd)
	foundCancel := false
	for _, result := range msgs {
		cancelMsg, ok := result.(message.CancelSpinnerMsg)
		if !ok {
			continue
		}
		if cancelMsg.ID != "containers.files.2" {
			t.Fatalf("CancelSpinnerMsg.ID = %q, want %q", cancelMsg.ID, "containers.files.2")
		}
		foundCancel = true
	}
	if !foundCancel {
		t.Fatal("expected CancelSpinnerMsg in batch result")
	}
}

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

func TestFileTreePanelSetSizeStoresWidth(t *testing.T) {
	p := newTestFileTreePanel()
	p.SetSize(100, 50)

	if p.width != 100 {
		t.Errorf("SetSize should store width=100, got %d", p.width)
	}

	if p.height != 50 {
		t.Errorf("SetSize should store height=50, got %d", p.height)
	}
}
