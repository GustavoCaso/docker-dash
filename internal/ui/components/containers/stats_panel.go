package containers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/NimbleMarkets/ntcharts/canvas/runes"
	"github.com/NimbleMarkets/ntcharts/linechart/streamlinechart"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/docker/docker/api/types/container"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/ui/components/panel"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
	"github.com/GustavoCaso/docker-dash/internal/ui/theme"
)

const (
	chartHalves      = 2     // divisor to split chart space in half
	netIOChartLines  = 2     // lines reserved for net/io chart label and legend
	cpuPercentFactor = 100.0 // multiplier to convert CPU ratio to percentage
)

func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

// statsOutputMsg is sent when stats output is received.
type statsOutputMsg struct {
	cpuPercetange    float64
	memoryPercentage float64
	memoryUsage      float64
	memoryLimit      float64
	networkRead      float64
	netwrokWrite     float64
	ioRead           float64
	ioWrite          float64
	err              error
}

type statsSessionStartedMsg struct {
	session *client.StatsSession
}

type statsPanel struct {
	service      client.ContainerService
	session      *client.StatsSession
	cpuChart     streamlinechart.Model
	memChart     streamlinechart.Model
	networkChart streamlinechart.Model
	ioChart      streamlinechart.Model
	lastView     string
	width        int
	height       int
}

func NewStatsPanel(svc client.ContainerService) panel.Panel {
	return &statsPanel{
		service: svc,
		cpuChart: streamlinechart.New(
			1, 1,
			streamlinechart.WithStyles(runes.ArcLineStyle, noStyle.Foreground(theme.DockerBlue)),
		),
		memChart: streamlinechart.New(
			1, 1,
			streamlinechart.WithStyles(runes.ArcLineStyle, noStyle.Foreground(theme.StatusRunning)),
		),
		networkChart: streamlinechart.New(
			1, 1,
			streamlinechart.WithStyles(runes.ArcLineStyle, noStyle.Foreground(theme.StatusRunning)),
			streamlinechart.WithDataSetStyles(
				"write",
				runes.ArcLineStyle,
				noStyle.Foreground(theme.StatusPaused),
			),
		),
		ioChart: streamlinechart.New(
			1, 1,
			streamlinechart.WithStyles(runes.ArcLineStyle, noStyle.Foreground(theme.DockerBlue)),
			streamlinechart.WithDataSetStyles(
				"write",
				runes.ArcLineStyle,
				noStyle.Foreground(theme.StatusError),
			),
		),
	}
}

func (s *statsPanel) Init(containerID string) tea.Cmd {
	return s.startSession(containerID)
}

func (s *statsPanel) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case statsSessionStartedMsg:
		s.session = msg.session
		return s.readOutput()
	case statsOutputMsg:
		if msg.err != nil {
			if s.session == nil {
				return nil
			}
			s.Close()
			return func() tea.Msg {
				return message.ShowBannerMsg{
					Message: fmt.Sprintf("Stats session error. Err: %s", msg.err),
					IsError: true,
				}
			}
		}

		s.cpuChart.Push(msg.cpuPercetange)
		s.cpuChart.Draw()
		s.memChart.Push(msg.memoryPercentage)
		s.memChart.Draw()
		s.networkChart.Push(msg.networkRead)
		s.networkChart.PushDataSet("write", msg.netwrokWrite)
		s.networkChart.DrawAll()
		s.ioChart.Push(msg.ioRead)
		s.ioChart.PushDataSet("write", msg.ioWrite)
		s.ioChart.DrawAll()

		cpuLabel := fmt.Sprintf("CPU %.2f%%", msg.cpuPercetange)
		memLabel := fmt.Sprintf("MEM %.2f%% (%s / %s)",
			msg.memoryPercentage,
			formatBytes(uint64(msg.memoryUsage)),
			formatBytes(uint64(msg.memoryLimit)),
		)
		netLabel := fmt.Sprintf("NET  rx:%s tx:%s",
			formatBytes(uint64(msg.networkRead)),
			formatBytes(uint64(msg.netwrokWrite)),
		)
		ioLabel := fmt.Sprintf("I/O  r:%s w:%s",
			formatBytes(uint64(msg.ioRead)),
			formatBytes(uint64(msg.ioWrite)),
		)

		netReadLegend := noStyle.Foreground(theme.StatusRunning).Render("● read")
		netWriteLegend := noStyle.Foreground(theme.StatusPaused).Render("● write")
		netLegend := netReadLegend + "  " + netWriteLegend

		ioReadLegend := noStyle.Foreground(theme.DockerBlue).Render("● read")
		ioWriteLegend := noStyle.Foreground(theme.StatusError).Render("● write")
		ioLegend := ioReadLegend + "  " + ioWriteLegend

		row1 := lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.JoinVertical(lipgloss.Left, cpuLabel, s.cpuChart.View()),
			lipgloss.JoinVertical(lipgloss.Left, memLabel, s.memChart.View()),
		)
		row2 := lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.JoinVertical(lipgloss.Left, netLabel, netLegend, s.networkChart.View()),
			lipgloss.JoinVertical(lipgloss.Left, ioLabel, ioLegend, s.ioChart.View()),
		)
		s.lastView = lipgloss.JoinVertical(lipgloss.Left, row1, row2)
		return s.readOutput()
	}
	return nil
}

func (s *statsPanel) View() string {
	return s.lastView
}

func (s *statsPanel) Close() {
	if s.session != nil {
		s.session.Close()
		s.session = nil
	}
	s.lastView = ""
}

func (s *statsPanel) SetSize(width, height int) {
	s.width = width
	s.height = height
	chartWidth := width / chartHalves
	cpuMemChartHeight := height/chartHalves - 1
	netIOChartHeight := height/chartHalves - netIOChartLines
	s.cpuChart.Resize(chartWidth, cpuMemChartHeight)
	s.memChart.Resize(chartWidth, cpuMemChartHeight)
	s.networkChart.Resize(chartWidth, netIOChartHeight)
	s.ioChart.Resize(chartWidth, netIOChartHeight)
}

func (s *statsPanel) startSession(containerID string) tea.Cmd {
	svc := s.service
	return func() tea.Msg {
		ctx := context.Background()
		session, err := svc.Stats(ctx, containerID)
		if err != nil {
			return statsOutputMsg{err: err}
		}
		return statsSessionStartedMsg{session: session}
	}
}

func (s *statsPanel) readOutput() tea.Cmd {
	session := s.session
	if session == nil {
		return nil
	}
	return func() tea.Msg {
		buf := make([]byte, readBufSize)
		n, err := session.Reader.Read(buf)
		if err != nil {
			return statsOutputMsg{err: err}
		}
		var stats container.StatsResponse
		err = json.Unmarshal(buf[:n], &stats)
		if err != nil {
			return statsOutputMsg{err: err}
		}

		cpuDelta := stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage
		systemCPUDelta := stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage
		numberCpus := stats.CPUStats.OnlineCPUs

		percentageCPUUsage := float64(cpuDelta) / float64(systemCPUDelta) * float64(numberCpus) * cpuPercentFactor

		memUsage := float64(stats.MemoryStats.Usage) - float64(stats.MemoryStats.Stats["cache"])
		memLimit := float64(stats.MemoryStats.Limit)
		var memPercentage float64
		if memLimit > 0 {
			memPercentage = memUsage / memLimit * cpuPercentFactor
		}

		networkRead := float64(0)
		networkWrite := float64(0)
		for _, stat := range stats.Networks {
			networkRead += float64(stat.RxBytes)
			networkWrite += float64(stat.TxBytes)
		}

		ioRead := float64(0)
		ioWrite := float64(0)
		for _, stat := range stats.BlkioStats.IoServiceBytesRecursive {
			if stat.Op == "read" {
				ioRead += float64(stat.Value)
			}
			if stat.Op == "write" {
				ioWrite += float64(stat.Value)
			}
		}

		return statsOutputMsg{
			cpuPercetange:    percentageCPUUsage,
			memoryPercentage: memPercentage,
			memoryUsage:      memUsage,
			memoryLimit:      memLimit,
			networkRead:      networkRead,
			netwrokWrite:     networkWrite,
			ioRead:           ioRead,
			ioWrite:          ioWrite,
		}
	}
}
