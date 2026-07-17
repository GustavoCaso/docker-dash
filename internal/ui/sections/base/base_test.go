package base

import (
	"slices"
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"

	"github.com/GustavoCaso/docker-dash/internal/ui/message"
	"github.com/GustavoCaso/docker-dash/internal/ui/sections"
)

type fakeItem struct {
	name string
}

func (i fakeItem) ID() string          { return i.name }
func (i fakeItem) Title() string       { return i.name }
func (i fakeItem) Description() string { return "" }
func (i fakeItem) FilterValue() string { return i.name }
func (i fakeItem) InnerItem() any      { return i.name }

type fakePanel struct {
	name          string
	view          string
	ids           []string
	updatedKeys   []string
	closed        bool
	width, height int
}

type fakePanelInitCmd struct{}

func (f *fakePanel) Name() string {
	return f.name
}

func (f *fakePanel) Init(item sections.ListItem) tea.Cmd {
	f.ids = append(f.ids, item.ID())
	return func() tea.Msg {
		return fakePanelInitCmd{}
	}
}

func (f *fakePanel) Update(msg tea.Msg) tea.Cmd {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		f.updatedKeys = append(f.updatedKeys, keyMsg.String())
	}
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

// newSectionWithItems creates a Section.
func newSectionWithItems(items []list.Item, panels []sections.Panel) *Section {
	s := New("test", panels)
	s.List.SetItems(items)
	return s
}

func TestRemoveItemAndUpdatePanel(t *testing.T) {
	fp := &fakePanel{}
	items := []list.Item{fakeItem{name: "test"}}

	section := newSectionWithItems(items, []sections.Panel{fp})

	cmd := section.RemoveItemAndUpdatePanel(0)
	if cmd != nil {
		t.Error("removeItem() should return nil cmd when list is empty (Close() returns nil)")
	}
	if !fp.closed {
		t.Error("RemoveItemAndUpdatePanel() should close the panel when there is not item left")
	}
}

func TestRemoveItemAndUpdatePanelUpdatesSelection(t *testing.T) {
	fp := &fakePanel{}
	items := []list.Item{
		fakeItem{name: "item1"},
		fakeItem{name: "item2"},
	}
	section := newSectionWithItems(items, []sections.Panel{fp})

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
	fp := &fakePanel{}
	items := []list.Item{
		fakeItem{name: "item1"},
		fakeItem{name: "item2"},
		fakeItem{name: "item3"},
	}
	section := newSectionWithItems(items, []sections.Panel{fp})

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
	fp := &fakePanel{}
	items := []list.Item{
		fakeItem{name: "item1"},
		fakeItem{name: "item2"},
		fakeItem{name: "item3"},
	}
	section := newSectionWithItems(items, []sections.Panel{fp})

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
	fp := &fakePanel{}
	items := []list.Item{
		fakeItem{name: "item1"},
		fakeItem{name: "item2"},
		fakeItem{name: "item3"},
	}
	section := newSectionWithItems(items, []sections.Panel{fp})

	// Delete items until one remains
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
	section := New("test", []sections.Panel{})

	section.isFilter = true

	section.Reset()

	if section.isFilter {
		t.Error("Reset() should set isFilter to false")
	}
}

func TestPanelClosedOnDownNavigation(t *testing.T) {
	fp := &fakePanel{}
	items := []list.Item{
		fakeItem{name: "item1"},
		fakeItem{name: "item2"},
	}
	section := newSectionWithItems(items, []sections.Panel{fp})

	// Navigate down to next container - this should close the current panel
	section.Update(tea.KeyPressMsg{Code: tea.KeyDown})

	// Verify the fake panel was closed and receive the new id
	if !fp.closed {
		t.Error("Navigation should close fake panel")
	}
	if !slices.Contains(fp.ids, "item2") {
		t.Errorf("fake panel should receive the update for the new active item, got %q", fp.ids)
	}
}

func TestPanelClosedOnUpNavigation(t *testing.T) {
	fp := &fakePanel{}
	items := []list.Item{
		fakeItem{name: "item1"},
		fakeItem{name: "item2"},
	}
	section := newSectionWithItems(items, []sections.Panel{fp})

	// Navigate up — selection wraps/stays, panel gets closed and re-inited
	section.Update(tea.KeyPressMsg{Code: tea.KeyUp})

	// Verify the fake panel was closed and receive the new id
	if !fp.closed {
		t.Error("Navigation should close fake panel")
	}
	if !slices.Contains(fp.ids, "item1") {
		t.Errorf("fake panel should receive the update for the new active item, got %q", fp.ids)
	}
}

func TestPanelNextSwitchesActivePanel(t *testing.T) {
	panelA := &fakePanel{name: "panelA"}
	panelB := &fakePanel{name: "panelB"}
	items := []list.Item{fakeItem{name: "item1"}}
	section := newSectionWithItems(items, []sections.Panel{panelA, panelB})

	// Starts on panelA (index 0)
	if section.activePanelIdx != 0 {
		t.Fatalf("expected activePanelIdx=0 initially, got %d", section.activePanelIdx)
	}

	// Set focus on panels
	section.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	// Press PanelNext (shift+right)
	section.Update(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModShift})

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
	section := newSectionWithItems(items, []sections.Panel{panelA, panelB})

	// Advance to the last panel manually
	section.activePanelIdx = 1

	// Set focus on panels
	section.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	// Press PanelNext — should wrap back to index 0
	section.Update(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModShift})

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
	section := newSectionWithItems(items, []sections.Panel{panelA, panelB})

	// Start on panelB (index 1)
	section.activePanelIdx = 1

	// Set focus on panels
	section.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	// Press PanelPrev (shift+left)
	section.Update(tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModShift})

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
	section := newSectionWithItems(items, []sections.Panel{panelA, panelB})

	// Set focus on panels
	section.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	// Start on panelA (index 0) — pressing prev should wrap to last
	section.Update(tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModShift})

	if section.activePanelIdx != 1 {
		t.Errorf("expected activePanelIdx=1 after wrapping back, got %d", section.activePanelIdx)
	}
	if !slices.Contains(panelB.ids, "item1") {
		t.Errorf("PanelPrev wrap should init panelB with the selected item id, got %q", panelB.ids)
	}
}

func TestFilterEscClearsFilterMode(t *testing.T) {
	section := newSectionWithItems([]list.Item{fakeItem{name: "item1"}}, nil)
	section.isFilter = true

	handled, cmds := section.handleFilterKey(tea.KeyPressMsg{Code: tea.KeyEscape})

	if !handled {
		t.Error("handleFilterKey should return handled=true when filter is active")
	}
	if section.isFilter {
		t.Error("Esc should set isFilter to false")
	}
	var gotClear bool
	for _, cmd := range cmds {
		if cmd == nil {
			continue
		}
		if _, ok := cmd().(message.ClearContextualKeyBindingsMsg); ok {
			gotClear = true
		}
	}
	if !gotClear {
		t.Error("Esc should emit ClearContextualKeyBindingsMsg")
	}
}

func TestFilterEnterResetsFilterAndSelectsItem(t *testing.T) {
	fp := &fakePanel{name: "panel"}
	items := []list.Item{
		fakeItem{name: "item1"},
		fakeItem{name: "item2"},
	}
	section := newSectionWithItems(items, []sections.Panel{fp})

	section.isFilter = true

	handled, cmds := section.handleFilterKey(tea.KeyPressMsg{Code: tea.KeyEnter})

	if !handled {
		t.Error("handleFilterKey should return handled=true when filter is active")
	}
	if section.isFilter {
		t.Error("Enter should set isFilter to false")
	}
	var gotClear bool
	for _, cmd := range cmds {
		if cmd == nil {
			continue
		}
		if _, ok := cmd().(message.ClearContextualKeyBindingsMsg); ok {
			gotClear = true
		}
	}
	if !gotClear {
		t.Error("Enter should emit ClearContextualKeyBindingsMsg")
	}
	if !fp.closed {
		t.Error("Enter should close the active panel")
	}
}

func TestFilterHandlesNonKeyMsg(t *testing.T) {
	section := newSectionWithItems([]list.Item{fakeItem{name: "item1"}}, nil)
	section.isFilter = true

	// A non-key message should still be forwarded to the list and return handled=true
	handled, _ := section.handleFilterKey(tea.WindowSizeMsg{Width: 80, Height: 24})

	if !handled {
		t.Error("handleFilterKey should return handled=true for non-key msgs when filter is active")
	}
	if !section.isFilter {
		t.Error("non-key msg should not exit filter mode")
	}
}

func TestFilterInactiveIgnoresMsgs(t *testing.T) {
	section := newSectionWithItems([]list.Item{fakeItem{name: "item1"}}, nil)

	handled, cmds := section.handleFilterKey(tea.KeyPressMsg{Code: tea.KeyEscape})

	if handled {
		t.Error("handleFilterKey should return handled=false when filter is not active")
	}
	if len(cmds) != 0 {
		t.Error("handleFilterKey should return no cmds when filter is not active")
	}
}

func TestCopyIDSuccessBanner(t *testing.T) {
	if clipboard.Unsupported {
		t.Skip("clipboard not available")
	}
	items := []list.Item{fakeItem{name: "abc123"}}
	section := newSectionWithItems(items, nil)

	cmd := section.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	if cmd == nil {
		t.Fatal("CopyID should return a cmd")
	}
	msg, ok := cmd().(message.ShowBannerMsg)
	if !ok {
		t.Fatalf("expected ShowBannerMsg, got %T", cmd())
	}
	if msg.IsError {
		t.Error("expected success banner, got error banner")
	}
}

func TestCopyIDNoItemBanner(t *testing.T) {
	section := newSectionWithItems([]list.Item{}, nil)

	cmd := section.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	if cmd == nil {
		t.Fatal("CopyID with no selection should return a cmd")
	}
	msg, ok := cmd().(message.ShowBannerMsg)
	if !ok {
		t.Fatalf("expected ShowBannerMsg, got %T", cmd())
	}
	if !msg.IsError {
		t.Error("expected error banner when no item selected")
	}
}

func TestCopyIDIgnoredWhenPanelFocused(t *testing.T) {
	fp := &fakePanel{name: "panel"}
	items := []list.Item{fakeItem{name: "abc123"}}
	section := newSectionWithItems(items, []sections.Panel{fp})

	// Move focus to panel
	section.Update(tea.KeyPressMsg{Code: tea.KeyTab})

	cmd := section.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})

	// cmd may be nil or come from the panel — the list should not produce a ShowBannerMsg
	if cmd != nil {
		if _, ok := cmd().(message.ShowBannerMsg); ok {
			t.Error("CopyID should not fire when focus is on panel")
		}
	}
	if !slices.Contains(fp.updatedKeys, "y") {
		t.Error("panel should receive the 'y' key when panel is focused")
	}
}

func TestTabTogglesFocusBetweenListAndPanel(t *testing.T) {
	fp := &fakePanel{name: "panel"}
	section := newSectionWithItems([]list.Item{fakeItem{name: "item1"}}, []sections.Panel{fp})

	if section.focus != focusList {
		t.Fatalf("expected initial focus to be list, got %d", section.focus)
	}

	section.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if section.focus != focusPanel {
		t.Fatalf("expected focus to switch to panel, got %d", section.focus)
	}

	section.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if section.focus != focusList {
		t.Fatalf("expected focus to switch back to list, got %d", section.focus)
	}
}

func TestPanelFocusedUpDownIsHandledByPanel(t *testing.T) {
	fp := &fakePanel{name: "panel"}
	items := []list.Item{
		fakeItem{name: "item1"},
		fakeItem{name: "item2"},
	}
	section := newSectionWithItems(items, []sections.Panel{fp})

	section.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	section.Update(tea.KeyPressMsg{Code: tea.KeyDown})

	if section.List.Index() != 0 {
		t.Fatalf("expected list selection to remain unchanged when panel is focused, got %d", section.List.Index())
	}
	if !slices.Contains(fp.updatedKeys, "down") {
		t.Fatalf("expected panel to receive down key when focused, got %v", fp.updatedKeys)
	}
}

func TestUpdateItemsSetsItems(t *testing.T) {
	fp := &fakePanel{name: "panel"}
	section := New("test", []sections.Panel{fp})

	items := []list.Item{fakeItem{name: "item1"}, fakeItem{name: "item2"}}
	cmds := section.UpdateItems(items)

	if len(section.List.Items()) != 2 {
		t.Errorf("expected 2 items, got %d", len(section.List.Items()))
	}
	if len(cmds) != 2 {
		t.Errorf("expected 2 cmds (SetItems + UpdateActivePanel), got %d", len(cmds))
	}
	// UpdateActivePanel (cmds[1]) should produce fakePanelInitCmd for the selected item
	if cmds[1] == nil {
		t.Fatal("cmds[1] (UpdateActivePanel) should not be nil")
	}
	if _, ok := cmds[1]().(fakePanelInitCmd); !ok {
		t.Errorf("cmds[1] should produce fakePanelInitCmd, got %T", cmds[1]())
	}
	if !slices.Contains(fp.ids, "item1") {
		t.Errorf("UpdateItems should init the active panel with the selected item, got %q", fp.ids)
	}
}

func TestUpdateItemsEmptyResetsSection(t *testing.T) {
	fp := &fakePanel{name: "panel"}
	items := []list.Item{fakeItem{name: "item1"}}
	section := newSectionWithItems(items, []sections.Panel{fp})

	section.isFilter = true

	cmds := section.UpdateItems([]list.Item{})

	if len(section.List.Items()) != 0 {
		t.Errorf("expected 0 items after clearing, got %d", len(section.List.Items()))
	}
	if section.isFilter {
		t.Error("UpdateItems with empty items should reset isFilter via Reset()")
	}
	if len(cmds) != 2 {
		t.Errorf("expected 2 cmds (SetItems + Reset), got %d", len(cmds))
	}
	// cmds[1] comes from Reset() → ActivePanel().Close() which fakePanel returns nil
	if cmds[1] != nil {
		t.Errorf("cmds[1] (Reset/Close) should be nil for fakePanel, got non-nil")
	}
}

// collectMsgs runs a command tree (following tea.BatchMsg) and returns every
// message it produces.
func collectMsgs(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, collectMsgs(c)...)
		}
		return out
	}
	return []tea.Msg{msg}
}

func findBubbleUpMsg(t *testing.T, msgs []tea.Msg) (message.BubbleUpMsg, bool) {
	t.Helper()
	for _, msg := range msgs {
		if bubble, ok := msg.(message.BubbleUpMsg); ok {
			return bubble, true
		}
	}
	return message.BubbleUpMsg{}, false
}

type fakeActionMsg struct{}

func TestRefreshAllSectionsEmitsBroadcastRefresh(t *testing.T) {
	section := newSectionWithItems([]list.Item{fakeItem{name: "item1"}}, nil)
	section.HandleMsg = func(msg tea.Msg) UpdateResult {
		if _, ok := msg.(fakeActionMsg); ok {
			return UpdateResult{Handled: true, RefreshAllSections: true}
		}
		return UpdateResult{}
	}

	msgs := collectMsgs(section.Update(fakeActionMsg{}))

	bubble, found := findBubbleUpMsg(t, msgs)
	if !found {
		t.Fatal("RefreshAllSections should emit a BubbleUpMsg")
	}
	if bubble.OnlyActive {
		t.Error("broadcast refresh must target all sections, got OnlyActive=true")
	}
	if bubble.KeyMsg.Text != "r" {
		t.Errorf("BubbleUpMsg key = %q, want %q (refresh)", bubble.KeyMsg.Text, "r")
	}
}

func TestHandledResultWithoutRefreshAllSectionsDoesNotBroadcast(t *testing.T) {
	section := newSectionWithItems([]list.Item{fakeItem{name: "item1"}}, nil)
	section.HandleMsg = func(msg tea.Msg) UpdateResult {
		if _, ok := msg.(fakeActionMsg); ok {
			return UpdateResult{Handled: true}
		}
		return UpdateResult{}
	}

	msgs := collectMsgs(section.Update(fakeActionMsg{}))

	if _, found := findBubbleUpMsg(t, msgs); found {
		t.Error("BubbleUpMsg should not be emitted when RefreshAllSections is false")
	}
}
