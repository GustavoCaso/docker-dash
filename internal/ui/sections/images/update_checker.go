package images

import (
	"context"
	"log"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/GustavoCaso/docker-dash/internal/client"
)

// updateCheckTickMsg is fired when the ticker interval elapses.
type updateCheckTickMsg struct{}

// imageUpdatesMsg is fired after a check completes, carrying the results.
type imageUpdatesMsg struct {
	updates map[string]bool // imageID → hasUpdate
}

// updateChecker drives periodic image update checks.
type updateChecker struct {
	interval time.Duration
	svc      client.ImageService
}

// tickCmd returns a Bubble Tea Cmd that fires updateCheckTickMsg after the interval.
func (c *updateChecker) tickCmd() tea.Cmd {
	return tea.Tick(c.interval, func(time.Time) tea.Msg {
		return updateCheckTickMsg{}
	})
}

// checkCmd returns a Bubble Tea Cmd that calls CheckUpdate for each image
// and emits imageUpdatesMsg with the results.
func (c *updateChecker) checkCmd(
	ctx context.Context,
	images []client.Image,
) tea.Cmd {
	svc := c.svc
	return func() tea.Msg {
		results := make(map[string]bool, len(images))
		for _, img := range images {
			hasUpdate, err := svc.CheckUpdate(ctx, img)
			if err != nil {
				log.Printf("[images] updateChecker: CheckUpdate error for image %q: %v", img.ID, err)
				continue
			}
			results[img.ID] = hasUpdate
		}
		return imageUpdatesMsg{updates: results}
	}
}
