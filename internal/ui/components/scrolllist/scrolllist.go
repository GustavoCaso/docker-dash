package scrolllist

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/GustavoCaso/docker-dash/internal/ui/keys"
	"github.com/GustavoCaso/docker-dash/internal/ui/theme"
)

const (
	ellipsisWidth = 1
	hScrollStep   = 10
)

// line is a single text line displayed in the list.
type line struct {
	Content string
}

func (l line) Title() string       { return l.Content }
func (l line) Description() string { return "" }
func (l line) FilterValue() string { return l.Content }

// delegate is the list item delegate that handles horizontal scrolling.
type delegate struct {
	hOffset int
}

// newDelegate creates a new delegate with zero offset.
func newDelegate() *delegate {
	return &delegate{}
}

// setHOffset sets the horizontal scroll offset.
func (d *delegate) setHOffset(offset int) {
	d.hOffset = offset
}

func (d *delegate) Height() int                             { return 1 }
func (d *delegate) Spacing() int                            { return 0 }
func (d *delegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d *delegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	lineItem, ok := item.(line)
	if !ok {
		return
	}
	width := m.Width()
	if width < 2 { //nolint:mnd // minimum width for content + ellipsis
		return
	}
	content := lineItem.Content
	if index == m.Index() {
		runes := []rune(content)
		start := min(d.hOffset, max(0, len(runes)-1))
		visible := string(runes[start:])
		if len([]rune(visible)) > width {
			visible = string([]rune(visible)[:width])
		}
		fmt.Fprint(w, theme.SelectedLogLine.Render(visible))
	} else {
		runes := []rune(content)
		if len(runes) > width-ellipsisWidth {
			content = string(runes[:width-ellipsisWidth]) + "…"
		}
		fmt.Fprint(w, content)
	}
}

// Model is a scrollable list that supports horizontal scrolling on the selected line.
type Model struct {
	list      list.Model
	delegate  *delegate
	prevIndex int
}

// New creates a new Model with no items.
func New() Model {
	d := newDelegate()
	l := list.New([]list.Item{}, d, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)
	return Model{list: l, delegate: d}
}

// SetLines replaces all items with the given lines.
func (m *Model) SetLines(lines []string) {
	items := make([]list.Item, len(lines))
	for i, s := range lines {
		items[i] = line{Content: s}
	}
	m.list.SetItems(items)
	if len(items) > 0 {
		m.list.Select(0)
	}
	m.delegate.hOffset = 0
	m.prevIndex = 0
}

// AppendLine adds a line at the end of the list.
func (m *Model) AppendLine(content string) {
	m.list.InsertItem(len(m.list.Items()), line{Content: content})
}

// Reset clears all items and resets scroll state.
func (m *Model) Reset() {
	m.list.SetItems([]list.Item{})
	m.delegate.hOffset = 0
	m.prevIndex = 0
}

// SetSize sets the width and height of the list.
func (m *Model) SetSize(width, height int) {
	m.list.SetSize(width, height)
}

// Update handles key messages for scroll and delegates the rest to the list.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, keys.Keys.LogScrollLeft):
			m.delegate.hOffset = max(0, m.delegate.hOffset-hScrollStep)
			return nil
		case key.Matches(keyMsg, keys.Keys.LogScrollRight):
			selected := m.list.SelectedItem()
			if selected != nil {
				if lineItem, lineOk := selected.(line); lineOk {
					maxOffset := max(0, len([]rune(lineItem.Content))-m.list.Width())
					m.delegate.hOffset = min(m.delegate.hOffset+hScrollStep, maxOffset)
				}
			}
			return nil
		}
	}

	prevIdx := m.list.Index()
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	newIndex := m.list.Index()

	// Skip over empty lines: nudge selection in the direction of travel.
	item := m.list.SelectedItem()
	if item != nil {
		if lineItem, ok := item.(line); ok && lineItem.Content == "" {
			items := m.list.Items()
			step := 1
			if newIndex < prevIdx {
				step = -1
			}
			for i := newIndex + step; i >= 0 && i < len(items); i += step {
				if l, ok := items[i].(line); ok && l.Content != "" {
					m.list.Select(i)
					newIndex = i
					break
				}
			}
		}
	}

	if newIndex != m.prevIndex {
		m.delegate.hOffset = 0
		m.prevIndex = newIndex
	}
	return cmd
}

// Items returns all list items.
func (m *Model) Items() []list.Item {
	return m.list.Items()
}

// Width returns the list width.
func (m *Model) Width() int {
	return m.list.Width()
}

// View renders the list.
func (m *Model) View() string {
	return m.list.View()
}
