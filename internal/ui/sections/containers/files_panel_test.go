package containers

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
)

func newTestFileTreePanel() *filesPanel {
	return newFilesPanel(context.Background(), client.NewMockClient().Containers()).(*filesPanel)
}

// loadFileTree runs Init's commands through Update and follows the file tree
// stream until the load completes.
func loadFileTree(t *testing.T, p *filesPanel, containerID string) {
	t.Helper()
	cmd := p.Init(containerItem{container: client.Container{ID: containerID}})
	for _, msg := range runBatch(cmd) {
		next := p.Update(msg)
		for _, chunk := chunkMsg(msg); chunk && next != nil; _, chunk = chunkMsg(msg) {
			msg = next()
			next = p.Update(msg)
		}
	}
	if p.loading {
		t.Fatal("file tree still loading after draining stream")
	}
}

func chunkMsg(msg tea.Msg) (fileTreeChunkMsg, bool) {
	chunk, ok := msg.(fileTreeChunkMsg)
	return chunk, ok
}

// fakeFileTreeService streams the configured batches, optionally blocking
// until its context is cancelled.
type fakeFileTreeService struct {
	client.ContainerService

	calls      int
	batches    [][]client.FileEntry
	err        error
	block      bool
	cancelled  chan struct{}
	cancelOnce sync.Once
}

func (s *fakeFileTreeService) FileTree(ctx context.Context, _ string, emit func([]client.FileEntry)) error {
	s.calls++
	for _, b := range s.batches {
		emit(b)
	}
	if s.block {
		<-ctx.Done()
		s.cancelOnce.Do(func() { close(s.cancelled) })
		return ctx.Err()
	}
	return s.err
}

func TestFileTreePanelInitFetchesTree(t *testing.T) {
	p := newTestFileTreePanel()
	cmd := p.Init(containerItem{container: client.Container{ID: "abc123def456"}})
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
		case fileTreeChunkMsg:
			containersTreeLoaded = true
		case message.AddContextualKeyBindingsMsg:
			extendCmd = true
		case message.ShowSpinnerMsg:
			tickMsg = true
		}
	}

	if !containersTreeLoaded {
		t.Fatal("Init() not returned fileTreeChunkMsg msg")
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

	loadFileTree(t, p, "abc123def456")

	if p.loading {
		t.Error("Update() Must set loading state to false. Got true")
	}

	_ = p.View()
}

func TestFileTreePanelUpdateWithError(t *testing.T) {
	p := newTestFileTreePanel()
	p.SetSize(80, 40)

	p.builder = client.NewFileTreeBuilder()
	errMsg := fileTreeChunkMsg{done: true, err: errors.New("fetch failed")}
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
	p.Close()
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

func TestFileTreePanelShowsMockEntries(t *testing.T) {
	p := newTestFileTreePanel()
	p.SetSize(80, 40)

	loadFileTree(t, p, "abc123def456")

	view := p.View()
	if !strings.Contains(view, "etc") {
		t.Errorf("View() should show mock directory 'etc', got: %q", view)
	}
	if !strings.Contains(view, "nginx.conf") {
		t.Errorf("View() should show mock file 'nginx.conf', got: %q", view)
	}
}

func TestFileTreePanelDirectoryToggle(t *testing.T) {
	p := newTestFileTreePanel()
	p.SetSize(80, 40)

	loadFileTree(t, p, "abc123def456")

	// cursor starts at 0 (first visible node); toggle collapse via Space
	if len(p.visible) == 0 {
		t.Fatal("expected visible nodes after loading mock file tree")
	}
	firstNode := p.visible[0]
	if !firstNode.IsDir {
		t.Skip("first visible node is not a directory; skip toggle test")
	}

	before := len(p.visible)
	p.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	after := len(p.visible)

	if after >= before {
		t.Errorf("Space on expanded dir should collapse it: before=%d after=%d", before, after)
	}

	// Toggle again to expand
	p.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	expanded := len(p.visible)
	if expanded != before {
		t.Errorf("Second Space should expand dir back: want=%d got=%d", before, expanded)
	}
}

func TestFileTreePanelCursorNavigation(t *testing.T) {
	p := newTestFileTreePanel()
	p.SetSize(80, 40)

	loadFileTree(t, p, "abc123def456")

	if len(p.visible) < 2 {
		t.Skip("need at least 2 visible nodes for cursor navigation test")
	}

	if p.cursor != 0 {
		t.Fatalf("cursor should start at 0, got %d", p.cursor)
	}

	p.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if p.cursor != 1 {
		t.Errorf("down arrow should move cursor to 1, got %d", p.cursor)
	}

	p.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if p.cursor != 0 {
		t.Errorf("up arrow should move cursor back to 0, got %d", p.cursor)
	}
}

func TestCopyFromContainer(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	p := newTestFileTreePanel()
	p.SetSize(80, 40)

	containerID := "abc123def456"
	loadFileTree(t, p, containerID)

	if len(p.visible) < 2 {
		t.Skip("need at least 2 visible nodes for cursor navigation test")
	}

	if p.cursor != 0 {
		t.Fatalf("cursor should start at 0, got %d", p.cursor)
	}

	p.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if p.cursor != 1 {
		t.Errorf("down arrow should move cursor to 1, got %d", p.cursor)
	}

	cpFromContainerKey := rune('c')
	cmd := p.Update(tea.KeyPressMsg{Code: cpFromContainerKey})
	showBannerMsg, ok := cmd().(message.ShowBannerMsg)
	if !ok {
		t.Fatalf("copyFromContainerCmd() returned %T, want message.ShowBannerMsg", cmd())
	}

	if showBannerMsg.IsError {
		t.Errorf("unexpected error copying from container %s", containerID)
	}
}

func TestFileTreePanelStreamsBatchesProgressively(t *testing.T) {
	svc := &fakeFileTreeService{
		batches: [][]client.FileEntry{
			{{Path: "etc", IsDir: true}, {Path: "etc/hosts"}},
			{{Path: "usr/bin/env"}},
		},
		block:     true,
		cancelled: make(chan struct{}),
	}
	p := newFilesPanel(context.Background(), svc).(*filesPanel)
	p.SetSize(80, 40)

	var chunk fileTreeChunkMsg
	for _, msg := range runBatch(p.Init(containerItem{container: client.Container{ID: "c1"}})) {
		if c, ok := chunkMsg(msg); ok {
			chunk = c
		}
	}
	if chunk.done {
		t.Fatal("stream should still be open while the service blocks")
	}

	next := p.Update(chunk)
	if next == nil {
		t.Fatal("expected a command waiting for further batches")
	}
	if !p.loading {
		t.Error("panel should still be loading")
	}
	view := p.View()
	if !strings.Contains(view, "hosts") {
		t.Errorf("partial tree should already be rendered, got %q", view)
	}
	if !strings.Contains(view, "loading") {
		t.Errorf("status bar should report loading progress, got %q", view)
	}

	p.Close()
	select {
	case <-svc.cancelled:
	case <-time.After(time.Second):
		t.Fatal("Close() did not cancel the file tree walk")
	}
}

func TestFileTreePanelSwitchingContainerCancelsPreviousWalk(t *testing.T) {
	svc := &fakeFileTreeService{block: true, cancelled: make(chan struct{})}
	p := newFilesPanel(context.Background(), svc).(*filesPanel)

	initCmd := p.Init(containerItem{container: client.Container{ID: "c1"}})
	// Start the walk; it blocks until cancelled.
	go runBatch(initCmd)

	cmd := p.Init(containerItem{container: client.Container{ID: "c2"}})
	select {
	case <-svc.cancelled:
	case <-time.After(time.Second):
		t.Fatal("switching container did not cancel the previous walk")
	}

	// Close so the second walk (which also blocks) returns when drained.
	p.Close()
	foundCancel := false
	for _, msg := range runBatch(cmd) {
		if m, ok := msg.(message.CancelSpinnerMsg); ok && m.ID == p.spinnerID(1) {
			foundCancel = true
		}
	}
	if !foundCancel {
		t.Error("expected the previous load's spinner to be cancelled")
	}
}

func TestFileTreePanelIgnoresStaleChunks(t *testing.T) {
	p := newTestFileTreePanel()
	loadFileTree(t, p, "abc123def456")
	before := len(p.visible)

	stale := fileTreeChunkMsg{requestID: p.requestID - 1, entries: []client.FileEntry{{Path: "stale"}}}
	if cmd := p.Update(stale); cmd != nil {
		t.Error("stale chunk should be ignored")
	}
	if len(p.visible) != before {
		t.Errorf("stale chunk changed the tree: before=%d after=%d", before, len(p.visible))
	}
}

func TestFileTreePanelUsesCache(t *testing.T) {
	svc := &fakeFileTreeService{batches: [][]client.FileEntry{{{Path: "etc/hosts"}}}}
	p := newFilesPanel(context.Background(), svc).(*filesPanel)
	p.SetSize(80, 40)

	loadFileTree(t, p, "c1")
	p.Update(tea.KeyPressMsg{Code: tea.KeyDown})

	// A list refresh re-initialises the panel for the same container: keep
	// the tree and the cursor.
	p.Init(containerItem{container: client.Container{ID: "c1"}})
	if p.cursor != 1 {
		t.Errorf("cursor should be preserved on re-init, got %d", p.cursor)
	}

	// Leaving and coming back is served from the cache too.
	p.Close()
	p.Init(containerItem{container: client.Container{ID: "c1"}})
	if p.loading {
		t.Error("cached tree should not trigger a load")
	}
	if !strings.Contains(p.View(), "hosts") {
		t.Errorf("cached tree should render, got %q", p.View())
	}
	if svc.calls != 1 {
		t.Errorf("FileTree calls: got %d, want 1", svc.calls)
	}

	// Once expired, the tree is fetched again.
	p.cache.now = func() time.Time { return time.Now().Add(2 * fileTreeCacheTTL) }
	p.Close()
	loadFileTree(t, p, "c1")
	if svc.calls != 2 {
		t.Errorf("FileTree calls after expiry: got %d, want 2", svc.calls)
	}
}

func TestFileTreePanelDoesNotCacheFailedLoads(t *testing.T) {
	svc := &fakeFileTreeService{
		batches: [][]client.FileEntry{{{Path: "etc/hosts"}}},
		err:     errors.New("boom"),
	}
	p := newFilesPanel(context.Background(), svc).(*filesPanel)

	loadFileTree(t, p, "c1")
	p.Close()
	loadFileTree(t, p, "c1")
	if svc.calls != 2 {
		t.Errorf("failed load must not be cached: calls=%d", svc.calls)
	}
}

func TestFileTreeCacheEvictsOldest(t *testing.T) {
	c := newFileTreeCache(time.Hour, 2)
	now := time.Now()
	c.now = func() time.Time { return now }
	c.put("a", &client.FileNode{})
	now = now.Add(time.Second)
	c.put("b", &client.FileNode{})
	now = now.Add(time.Second)
	c.put("c", &client.FileNode{})

	if _, ok := c.get("a"); ok {
		t.Error("oldest entry should have been evicted")
	}
	for _, id := range []string{"b", "c"} {
		if _, ok := c.get(id); !ok {
			t.Errorf("entry %q should still be cached", id)
		}
	}
}
