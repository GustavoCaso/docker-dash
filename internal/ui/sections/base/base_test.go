package base

import (
	"slices"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/GustavoCaso/docker-dash/internal/ui/components/panel"
)

type fakeItem struct {
	name string
}

func (i fakeItem) FilterValue() string { return i.name }

type fakePanel struct {
	name          string
	view          string
	ids           []string
	closed        bool
	width, height int
}

type fakePanelInitCmd struct{}

func (f *fakePanel) Name() string {
	return f.name
}

func (f *fakePanel) Init(id string) tea.Cmd {
	f.ids = append(f.ids, id)
	return func() tea.Msg {
		return fakePanelInitCmd{}
	}
}

func (f *fakePanel) Update(msg tea.Msg) tea.Cmd {
	return nil
}

func (f *fakePanel) View() string {
	return f.view
}

func (f *fakePanel) Close() tea.Cmd {
	f.closed = true
	return nil
}

func (f *fakePanel) SetSize(width, height int) {
	f.width = width
	f.height = height
}

func TestRemoveItemAndUpdatePanel(t *testing.T) {
	fakePanel := &fakePanel{}

	section := New("test", []list.Item{fakeItem{name: "test"}}, []panel.Panel{fakePanel})
	section.ActivePanelInitFn = func(item list.Item) string {
		i := item.(fakeItem)
		return i.name
	}

	cmd := section.RemoveItemAndUpdatePanel(0)
	if cmd != nil {
		t.Error("removeItem() should return nil cmd when list is empty (Close() returns nil)")
	}
	if !fakePanel.closed {
		t.Error("RemoveItemAndUpdatePanel() should close the panel when there is not item left")
	}
}

func TestRemoveItemAndUpdatePanelUpdatesSelection(t *testing.T) {
	fakePanel := &fakePanel{}
	items := []list.Item{
		fakeItem{name: "item1"},
		fakeItem{name: "item2"},
	}
	section := New("test", items, []panel.Panel{fakePanel})
	section.ActivePanelInitFn = func(item list.Item) string {
		i := item.(fakeItem)
		return i.name
	}

	initialCount := len(section.List.Items())
	section.List.Select(0)
	cmd := section.RemoveItemAndUpdatePanel(0)

	if len(section.List.Items()) != initialCount-1 {
		t.Errorf("expected %d items after delete, got %d", initialCount-1, len(section.List.Items()))
	}
	if section.List.Index() != 0 {
		t.Errorf("expected selection at index 0 after deleting first item, got %d", section.List.Index())
	}
	if cmd == nil {
		t.Fatal("removeItem() should return non-nil cmd when items remain")
	}
	if _, ok := cmd().(fakePanelInitCmd); !ok {
		t.Errorf("removeItem() cmd should produce fakePanelInitCmd, got %T", cmd())
	}
}

func TestRemoveItemAndUpdatePanelMiddleItemClampsToLastWhenAtEnd(t *testing.T) {
	fakePanel := &fakePanel{}
	items := []list.Item{
		fakeItem{name: "item1"},
		fakeItem{name: "item2"},
		fakeItem{name: "item3"},
	}
	section := New("test", items, []panel.Panel{fakePanel})
	section.ActivePanelInitFn = func(item list.Item) string {
		i := item.(fakeItem)
		return i.name
	}

	count := len(section.List.Items())
	// Select and delete the last item — selection should clamp to new last
	last := count - 1
	section.List.Select(last)
	cmd := section.RemoveItemAndUpdatePanel(last)

	if section.List.Index() != last-1 {
		t.Errorf("expected selection at %d after deleting last item, got %d", last-1, section.List.Index())
	}
	if cmd == nil {
		t.Fatal("removeItem() should return non-nil cmd when items remain")
	}
	if _, ok := cmd().(fakePanelInitCmd); !ok {
		t.Errorf("removeItem() cmd should produce detailsMsg, got %T", cmd())
	}
}

func TestRemoveItemUpdatesSelection(t *testing.T) {
	fakePanel := &fakePanel{}
	items := []list.Item{
		fakeItem{name: "item1"},
		fakeItem{name: "item2"},
		fakeItem{name: "item3"},
	}
	section := New("test", items, []panel.Panel{fakePanel})
	section.ActivePanelInitFn = func(item list.Item) string {
		i := item.(fakeItem)
		return i.name
	}

	initialCount := len(section.List.Items())

	// Select the first item and delete it
	section.List.Select(0)
	section.RemoveItem(0)

	if len(section.List.Items()) != initialCount-1 {
		t.Errorf("expected %d items after delete, got %d", initialCount-1, len(section.List.Items()))
	}
	if section.List.Index() != 0 {
		t.Errorf("expected selection at index 0 after deleting first item, got %d", section.List.Index())
	}
}

func TestRemoveItem_LastItemClampsSelection(t *testing.T) {
	fakePanel := &fakePanel{}
	items := []list.Item{
		fakeItem{name: "item1"},
		fakeItem{name: "item2"},
		fakeItem{name: "item3"},
	}
	section := New("test", items, []panel.Panel{fakePanel})
	section.ActivePanelInitFn = func(item list.Item) string {
		i := item.(fakeItem)
		return i.name
	}

	// Select and delete items until one remains
	for len(section.List.Items()) > 1 {
		section.RemoveItem(len(section.List.Items()) - 1)
	}

	// Delete the last item
	section.RemoveItem(0)

	if len(section.List.Items()) != 0 {
		t.Errorf("expected 0 items, got %d", len(section.List.Items()))
	}
}

func TestResetClearsFilterFlag(t *testing.T) {
	section := New("test", []list.Item{}, []panel.Panel{})

	section.isFilter = true

	section.Reset()

	if section.isFilter {
		t.Error("Reset() should set isFilter to false")
	}
}

func TestPanelClosedOnDownNavigation(t *testing.T) {
	fakePanel := &fakePanel{}
	items := []list.Item{
		fakeItem{name: "item1"},
		fakeItem{name: "item2"},
	}
	section := New("test", items, []panel.Panel{fakePanel})
	section.ActivePanelInitFn = func(item list.Item) string {
		i := item.(fakeItem)
		return i.name
	}

	// Navigate down to next container - this should close the current panel
	section.Update(tea.KeyMsg{Type: tea.KeyDown})

	// Verify the fake panel was closed and receieve the new id
	if !fakePanel.closed {
		t.Error("Navigation should close fake panel")
	}
	if !slices.Contains(fakePanel.ids, "item2") {
		t.Errorf("fake panel should receive the update for the new active item, got %q", fakePanel.ids)
	}
}

func TestPanelClosedOnUpNavigation(t *testing.T) {
	fakePanel := &fakePanel{}
	items := []list.Item{
		fakeItem{name: "item1"},
		fakeItem{name: "item2"},
	}
	section := New("test", items, []panel.Panel{fakePanel})
	section.ActivePanelInitFn = func(item list.Item) string {
		i := item.(fakeItem)
		return i.name
	}

	// Navigate down to next container - this should close the current panel
	section.Update(tea.KeyMsg{Type: tea.KeyUp})

	// Verify the fake panel was closed and receieve the new id
	if !fakePanel.closed {
		t.Error("Navigation should close fake panel")
	}
	if !slices.Contains(fakePanel.ids, "item1") {
		t.Errorf("fake panel should receive the update for the new active item, got %q", fakePanel.ids)
	}
}

func TestPanelNextSwitchesActivePanel(t *testing.T) {
	panelA := &fakePanel{name: "panelA"}
	panelB := &fakePanel{name: "panelB"}
	items := []list.Item{fakeItem{name: "item1"}}
	section := New("test", items, []panel.Panel{panelA, panelB})
	section.ActivePanelInitFn = func(item list.Item) string {
		i := item.(fakeItem)
		return i.name
	}

	// Starts on panelA (index 0)
	if section.activePanelIdx != 0 {
		t.Fatalf("expected activePanelIdx=0 initially, got %d", section.activePanelIdx)
	}

	// Press PanelNext (shift+right)
	section.Update(tea.KeyMsg{Type: tea.KeyShiftRight})

	if !panelA.closed {
		t.Error("PanelNext should close the previous active panel")
	}
	if section.activePanelIdx != 1 {
		t.Errorf("expected activePanelIdx=1 after PanelNext, got %d", section.activePanelIdx)
	}
	if !slices.Contains(panelB.ids, "item1") {
		t.Errorf("PanelNext should init the new active panel with the selected item id, got %q", panelB.ids)
	}
}

func TestPanelNextWrapsAround(t *testing.T) {
	panelA := &fakePanel{name: "panelA"}
	panelB := &fakePanel{name: "panelB"}
	items := []list.Item{fakeItem{name: "item1"}}
	section := New("test", items, []panel.Panel{panelA, panelB})
	section.ActivePanelInitFn = func(item list.Item) string {
		i := item.(fakeItem)
		return i.name
	}

	// Advance to the last panel manually
	section.activePanelIdx = 1

	// Press PanelNext — should wrap back to index 0
	section.Update(tea.KeyMsg{Type: tea.KeyShiftRight})

	if section.activePanelIdx != 0 {
		t.Errorf("expected activePanelIdx=0 after wrapping, got %d", section.activePanelIdx)
	}
	if !slices.Contains(panelA.ids, "item1") {
		t.Errorf("PanelNext wrap should init panelA with the selected item id, got %q", panelA.ids)
	}
}

func TestPanelPrevSwitchesActivePanel(t *testing.T) {
	panelA := &fakePanel{name: "panelA"}
	panelB := &fakePanel{name: "panelB"}
	items := []list.Item{fakeItem{name: "item1"}}
	section := New("test", items, []panel.Panel{panelA, panelB})
	section.ActivePanelInitFn = func(item list.Item) string {
		i := item.(fakeItem)
		return i.name
	}

	// Start on panelB (index 1)
	section.activePanelIdx = 1

	// Press PanelPrev (shift+left)
	section.Update(tea.KeyMsg{Type: tea.KeyShiftLeft})

	if !panelB.closed {
		t.Error("PanelPrev should close the previous active panel")
	}
	if section.activePanelIdx != 0 {
		t.Errorf("expected activePanelIdx=0 after PanelPrev, got %d", section.activePanelIdx)
	}
	if !slices.Contains(panelA.ids, "item1") {
		t.Errorf("PanelPrev should init the new active panel with the selected item id, got %q", panelA.ids)
	}
}

func TestPanelPrevWrapsAround(t *testing.T) {
	panelA := &fakePanel{name: "panelA"}
	panelB := &fakePanel{name: "panelB"}
	items := []list.Item{fakeItem{name: "item1"}}
	section := New("test", items, []panel.Panel{panelA, panelB})
	section.ActivePanelInitFn = func(item list.Item) string {
		i := item.(fakeItem)
		return i.name
	}

	// Start on panelA (index 0) — pressing prev should wrap to last
	section.Update(tea.KeyMsg{Type: tea.KeyShiftLeft})

	if section.activePanelIdx != 1 {
		t.Errorf("expected activePanelIdx=1 after wrapping back, got %d", section.activePanelIdx)
	}
	if !slices.Contains(panelB.ids, "item1") {
		t.Errorf("PanelPrev wrap should init panelB with the selected item id, got %q", panelB.ids)
	}
}
