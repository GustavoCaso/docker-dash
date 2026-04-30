package systeminfo

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
	"github.com/GustavoCaso/docker-dash/internal/ui/theme"
)

var (
	modalStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Border).
			Padding(1)
	modalStyleX, modalStyleY = modalStyle.GetFrameSize()
	titleStyle               = lipgloss.NewStyle().Bold(true)
	diskTitleStyle           = lipgloss.NewStyle().Bold(true)
	hintStyle                = lipgloss.NewStyle().Faint(true)
)

const (
	defaultModalWidth  = 100
	defaultModalHeight = 30
	columnsNumber      = 2
)

var title = titleStyle.Render("System Information")
var hintText = hintStyle.Render("[i/esc] exit")

type Model struct {
	ctx        context.Context
	client     client.Client
	systemInfo *client.SystemInfo
	width      int
	height     int
}

func New(ctx context.Context, c client.Client) Model {
	return Model{
		ctx:    ctx,
		client: c,
		width:  defaultModalWidth,
		height: defaultModalHeight,
	}
}

func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m Model) Init() tea.Cmd {
	return func() tea.Msg {
		systemInfo, err := m.client.Info(m.ctx)
		if err != nil {
			return message.SystemInfoOutputMsg{Err: err}
		}
		return message.SystemInfoOutputMsg{Info: &systemInfo}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	sysMsg, ok := msg.(message.SystemInfoOutputMsg)
	if !ok {
		return m, nil
	}

	if sysMsg.Err != nil || sysMsg.Info == nil {
		return m, nil
	}

	m.systemInfo = sysMsg.Info
	return m, nil
}

func (m Model) View() string {
	modalWidth := m.width - modalStyleX
	modalHeight := m.height - modalStyleY
	columnsSize := (modalWidth / columnsNumber) - modalStyleX

	if m.systemInfo == nil {
		return modalStyle.Width(modalWidth).Height(modalHeight).Render(
			lipgloss.Place(modalWidth, modalHeight, lipgloss.Center, lipgloss.Center, "Loading..."),
		)
	}

	kernelTruncated := m.systemInfo.Kernel
	if len(kernelTruncated) > columnsSize {
		kernelTruncated = kernelTruncated[:columnsSize] + "\n" + kernelTruncated[columnsSize:]
	}

	var driverStatus strings.Builder
	for _, pair := range m.systemInfo.StorageDriver.DriverStatus {
		key := pair[0]
		value := pair[1]
		maxValueLen := max(columnsSize-len(key), 0)
		if len(value) > maxValueLen {
			value = value[:maxValueLen] + "..."
		}
		fmt.Fprintf(&driverStatus, "\n%s: %s", key, value)
	}

	var leftCol strings.Builder
	leftCol.WriteString("\n\n")
	fmt.Fprintf(&leftCol, "Docker Version: %s\n\n", m.systemInfo.DockerVersion)
	fmt.Fprintf(&leftCol, "API Version: %s\n\n", m.systemInfo.APIVersion)
	fmt.Fprintf(&leftCol, "OS: %s\n\n", m.systemInfo.OS)
	fmt.Fprintf(&leftCol, "Arch: %s\n\n", m.systemInfo.Arch)
	fmt.Fprintf(&leftCol, "Kernel: %s\n\n", kernelTruncated)
	fmt.Fprintf(&leftCol, "Driver: %s\n\n", m.systemInfo.StorageDriver.Driver)
	fmt.Fprintf(&leftCol, "Driver Status:%s\n\n", driverStatus.String())

	totalMem := formatBytes(m.systemInfo.TotalMemoryBytes)
	layerSize := formatBytes(m.systemInfo.DiskUsage.LayerSize)
	containersSize := formatBytes(m.systemInfo.DiskUsage.ContainersSize)
	imagesSize := formatBytes(m.systemInfo.DiskUsage.ImagesSize)
	volumesSize := formatBytes(m.systemInfo.DiskUsage.VolumesSize)

	var rightCol strings.Builder
	rightCol.WriteString("\n\n")
	fmt.Fprintf(&rightCol, "CPUs: %d\n\n", m.systemInfo.CPUs)
	fmt.Fprintf(&rightCol, "Total Memory: %s\n\n", totalMem)
	rightCol.WriteString(diskTitleStyle.Render("Disk Usage") + "\n\n")
	fmt.Fprintf(&rightCol, "Layer size: %s\n\n", layerSize)
	fmt.Fprintf(&rightCol, "Containers size: %s\n\n", containersSize)
	fmt.Fprintf(&rightCol, "Images size: %s\n\n", imagesSize)
	fmt.Fprintf(&rightCol, "Volumes size: %s", volumesSize)

	columns := lipgloss.JoinHorizontal(
		lipgloss.Left,
		lipgloss.PlaceHorizontal(columnsSize, lipgloss.Center, leftCol.String()),
		lipgloss.PlaceHorizontal(columnsSize, lipgloss.Center, rightCol.String()),
	)

	var warnings strings.Builder
	if len(m.systemInfo.Warnings) > 0 {
		warnings.WriteString("\nWarnings:\n")
		for _, warning := range m.systemInfo.Warnings {
			fmt.Fprintf(&warnings, "  - %s\n", warning)
		}
	}

	content := lipgloss.JoinVertical(
		lipgloss.Top,
		lipgloss.PlaceHorizontal(modalWidth, lipgloss.Left, hintText),
		lipgloss.PlaceHorizontal(modalWidth, lipgloss.Center, title),
		columns,
		lipgloss.NewStyle().Width(modalWidth).Render(warnings.String()),
	)

	return modalStyle.Width(modalWidth).Height(modalHeight).Render(content)
}

func formatBytes(b int64) string {
	if b < 0 {
		return "N/A"
	}
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}
