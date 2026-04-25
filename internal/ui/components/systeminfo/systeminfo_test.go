package systeminfo

import (
	"context"
	"strings"
	"testing"

	"github.com/GustavoCaso/docker-dash/internal/client"
	"github.com/GustavoCaso/docker-dash/internal/ui/message"
)

func TestSystemInfoView(t *testing.T) {
	c := client.NewMockClient()
	m := New(context.Background(), c)
	cmd := m.Init()

	msg, ok := cmd().(message.SystemInfoOutputMsg)
	if !ok {
		t.Fatalf("unexpected msg got= %T, expected= %T", msg, message.SystemInfoOutputMsg{})
	}
	if msg.Err != nil {
		t.Fatalf("msg.Err got= %v, expected= nil", msg.Err)
	}
	if msg.Info == nil {
		t.Fatalf("msg.Info got= %v, expected= %v", msg.Info, m.systemInfo)
	}

	model, _ := m.Update(msg)
	systemM, ok := model.(Model)
	if !ok {
		t.Fatalf("unexpected model got= %T, expected= %T", model, Model{})
	}

	view := systemM.View()
	if !strings.Contains(view, systemM.systemInfo.DockerVersion) {
		t.Errorf("view does not contain DockerVersion")
	}
	if !strings.Contains(view, systemM.systemInfo.APIVersion) {
		t.Errorf("view does not contain APIVersion")
	}
	if !strings.Contains(view, systemM.systemInfo.OS) {
		t.Errorf("view does not contain OS")
	}
	if !strings.Contains(view, systemM.systemInfo.Arch) {
		t.Errorf("view does not contain Arch")
	}
}
