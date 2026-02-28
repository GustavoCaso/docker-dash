package statusbar

import (
	"testing"

	"github.com/GustavoCaso/docker-dash/internal/ui/keys"
)

func TestStatusBarSetKeyMap(t *testing.T) {
	sb := New()
	sb.SetSize(80, 1)
	sb.SetKeyMap(keys.Keys.HeaderKeyMap())

	got := sb.View()
	if got == "" {
		t.Error("View() returned empty string after setting bindings")
	}
}
