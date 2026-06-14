package scrolllist

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

func newModel(width, height int) *Model {
	m := New()
	m.SetSize(width, height)
	return &m
}

func TestNewModelEmpty(t *testing.T) {
	m := New()
	if m.delegate.hOffset != 0 {
		t.Errorf("new model hOffset should be 0, got %d", m.delegate.hOffset)
	}
}

func TestSetLines(t *testing.T) {
	m := newModel(80, 20)
	m.SetLines([]string{"line one", "line two", "line three"})

	items := m.Items()
	for i, want := range []string{"line one", "line two", "line three"} {
		line, ok := items[i].(line)
		if !ok {
			t.Fatalf("item[%d] is not Line", i)
		}
		if line.Content != want {
			t.Errorf("item[%d] = %q, want %q", i, line.Content, want)
		}
	}
}

func TestSetLinesResetsScrollState(t *testing.T) {
	m := newModel(10, 10)
	m.SetLines([]string{strings.Repeat("x", 50)})
	// Scroll right
	for range 5 {
		m.Update(tea.KeyMsg{Type: tea.KeyRight})
	}
	if m.delegate.hOffset == 0 {
		t.Fatal("expected non-zero hOffset after scrolling right")
	}

	// SetLines should reset hOffset
	m.SetLines([]string{"new content"})
	if m.delegate.hOffset != 0 {
		t.Errorf("SetLines should reset hOffset to 0, got %d", m.delegate.hOffset)
	}
}

func TestAppendLine(t *testing.T) {
	m := newModel(80, 20)
	m.AppendLine("first")
	m.AppendLine("second")

	items := m.Items()
	line, ok := items[1].(line)
	if !ok {
		t.Fatal("item[1] is not Line")
	}
	if line.Content != "second" {
		t.Errorf("item[1] = %q, want 'second'", line.Content)
	}
}

func TestReset(t *testing.T) {
	m := newModel(10, 10)
	m.SetLines([]string{strings.Repeat("x", 50)})
	for range 3 {
		m.Update(tea.KeyMsg{Type: tea.KeyRight})
	}

	m.Reset()

	if len(m.Items()) != 0 {
		t.Errorf("Reset should clear items, got %d", len(m.Items()))
	}
	if m.delegate.hOffset != 0 {
		t.Errorf("Reset should clear hOffset, got %d", m.delegate.hOffset)
	}
	if m.prevIndex != 0 {
		t.Errorf("Reset should clear prevIndex, got %d", m.prevIndex)
	}
}

func TestScrollRightClampsAtMax(t *testing.T) {
	m := newModel(10, 10)
	content := "0123456789abcdef" // 16 chars, width=10 → maxOffset=6
	m.SetLines([]string{content})

	for range 20 {
		m.Update(tea.KeyMsg{Type: tea.KeyRight})
	}

	selected, ok := m.list.SelectedItem().(line)
	if !ok {
		t.Fatal("selected item is not Line")
	}
	maxOffset := max(0, len([]rune(selected.Content))-m.Width())
	if m.delegate.hOffset > maxOffset {
		t.Errorf("hOffset %d exceeds maxOffset %d", m.delegate.hOffset, maxOffset)
	}
}

func TestScrollLeftClampsAtZero(t *testing.T) {
	m := newModel(10, 10)
	m.SetLines([]string{strings.Repeat("x", 50)})
	// Scroll right first
	for range 5 {
		m.Update(tea.KeyMsg{Type: tea.KeyRight})
	}
	// Then scroll left past zero
	for range 20 {
		m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	}
	if m.delegate.hOffset != 0 {
		t.Errorf("hOffset should not go below 0, got %d", m.delegate.hOffset)
	}
}

func TestHOffsetResetsOnItemChange(t *testing.T) {
	m := newModel(10, 20)
	m.SetLines([]string{strings.Repeat("x", 50), strings.Repeat("y", 50)})
	for range 3 {
		m.Update(tea.KeyMsg{Type: tea.KeyRight})
	}
	if m.delegate.hOffset == 0 {
		t.Fatal("expected non-zero hOffset")
	}
	// Move selection down — hOffset should reset
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.delegate.hOffset != 0 {
		t.Errorf("hOffset should reset on index change, got %d", m.delegate.hOffset)
	}
}

func TestDelegateTruncatesNonSelectedLines(t *testing.T) {
	d := newDelegate()
	items := []list.Item{
		line{Content: strings.Repeat("x", 200)},
		line{Content: "short"},
	}
	m := list.New(items, d, 50, 10)
	m.Select(1) // select second item

	var buf strings.Builder
	d.Render(&buf, m, 0, items[0])
	rendered := buf.String()

	if !strings.Contains(rendered, "…") {
		t.Errorf("expected ellipsis in truncated non-selected line, got %q", rendered)
	}
	// rendered should be at most width chars + ellipsis
	if len([]rune(rendered)) > 52 {
		t.Errorf("non-selected line too long: len=%d", len(rendered))
	}
}

func TestDelegateAppliesHOffsetOnSelectedLine(t *testing.T) {
	d := newDelegate()
	d.setHOffset(5)

	content := "0123456789abcdef"
	items := []list.Item{line{Content: content}}
	m := list.New(items, d, 50, 10)
	m.Select(0)

	var buf strings.Builder
	d.Render(&buf, m, 0, items[0])
	rendered := buf.String()

	if strings.Contains(rendered, "01234") {
		t.Errorf("hOffset not applied: first 5 chars still visible in %q", rendered)
	}
}
