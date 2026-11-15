package client

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"testing"
)

func collectEntries(t *testing.T, output string) ([]FileEntry, error) {
	t.Helper()
	var got []FileEntry
	n, err := parseFindOutput(strings.NewReader(output), func(entries []FileEntry) {
		got = append(got, entries...)
	})
	if n != len(got) {
		t.Errorf("parseFindOutput returned n=%d but emitted %d entries", n, len(got))
	}
	return got, err
}

func TestFileTreeBuilder_OutOfOrderEntries(t *testing.T) {
	b := NewFileTreeBuilder()
	b.Add(FileEntry{Path: "/usr/bin/env", Size: 10})
	b.Add(FileEntry{Path: "/usr/bin", IsDir: true, Mode: fs.ModeDir | 0o755})
	b.Add(FileEntry{Path: "/usr/bin/sh", Linkname: "busybox", Mode: fs.ModeSymlink | 0o777})
	b.Add(FileEntry{Path: "/", IsDir: true})

	root := b.Root()
	if len(root.Children) != 1 {
		t.Fatalf("root children: got %d, want 1", len(root.Children))
	}
	usr := root.Children[0]
	if usr.Path != "/usr" || usr.Depth != 1 {
		t.Errorf("usr: got path=%q depth=%d", usr.Path, usr.Depth)
	}
	if len(usr.Children) != 1 {
		t.Fatalf("usr children: got %d, want 1 (explicit dir must reuse implicit node)", len(usr.Children))
	}
	bin := usr.Children[0]
	if bin.Mode != fs.ModeDir|0o755 {
		t.Errorf("bin mode: got %v, want explicit entry mode", bin.Mode)
	}
	if len(bin.Children) != 2 {
		t.Fatalf("bin children: got %d, want 2", len(bin.Children))
	}
	sh := bin.Children[1]
	if sh.Path != "/usr/bin/sh" || sh.Linkname != "busybox" || sh.Depth != 3 || sh.Parent != bin {
		t.Errorf("sh: got path=%q link=%q depth=%d", sh.Path, sh.Linkname, sh.Depth)
	}
	if b.Len() != 4 {
		t.Errorf("Len: got %d, want 4", b.Len())
	}
}

func BenchmarkFileTreeBuilder_WideDirectories(b *testing.B) {
	entries := make([]FileEntry, 0, 100_000)
	for d := range 1000 {
		for f := range 100 {
			entries = append(entries, FileEntry{Path: fmt.Sprintf("node_modules/pkg%d/file%d", d, f)})
		}
	}
	b.ResetTimer()
	for range b.N {
		tb := NewFileTreeBuilder()
		for _, e := range entries {
			tb.Add(e)
		}
	}
}

func TestParseFindOutput_GNU(t *testing.T) {
	record := func(fields ...string) string { return strings.Join(fields, "\x00") + "\x00" }
	output := "gnu\n" +
		record("d", "4096", "755", "/", "") +
		record("d", "4096", "755", "/etc", "") +
		record("f", "120", "644", "/etc/my file\twith tab", "") +
		record("l", "7", "777", "/bin/sh", "busybox") +
		record("f", "10", "4755", "/usr/bin/su", "")

	got, err := collectEntries(t, output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 5 {
		t.Fatalf("got %d entries, want 5", len(got))
	}
	if !got[1].IsDir || got[1].Mode != fs.ModeDir|0o755 {
		t.Errorf("/etc: got isDir=%v mode=%v", got[1].IsDir, got[1].Mode)
	}
	if got[2].Path != "/etc/my file\twith tab" || got[2].Size != 120 || got[2].Mode != 0o644 {
		t.Errorf("file: got %+v", got[2])
	}
	if got[3].Linkname != "busybox" || got[3].Mode&fs.ModeSymlink == 0 {
		t.Errorf("symlink: got %+v", got[3])
	}
	if got[4].Mode&fs.ModeSetuid == 0 {
		t.Errorf("setuid bit lost: got %v", got[4].Mode)
	}
}

func TestParseFindOutput_GNUTruncatedRecord(t *testing.T) {
	_, err := collectEntries(t, "gnu\nf\x00120\x00")
	if err == nil {
		t.Fatal("expected error for truncated record")
	}
}

func TestParseFindOutput_Stat(t *testing.T) {
	output := "stat\n" +
		"41ed 4096 '/'\n" +
		"41ed 4096 '/etc'\n" +
		"81a4 120 '/etc/hosts'\n" +
		"a1ff 12 '/bin/sh' -> '/bin/busybox'\n" +
		"81a4 5 /unquoted\n" +
		"garbage line\n"

	got, err := collectEntries(t, output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 5 {
		t.Fatalf("got %d entries, want 5: %+v", len(got), got)
	}
	if got[1].Path != "/etc" || !got[1].IsDir || got[1].Mode != fs.ModeDir|0o755 {
		t.Errorf("/etc: got %+v", got[1])
	}
	if got[2].Path != "/etc/hosts" || got[2].Size != 120 || got[2].Mode != 0o644 {
		t.Errorf("hosts: got %+v", got[2])
	}
	if got[3].Path != "/bin/sh" || got[3].Linkname != "/bin/busybox" || got[3].Mode&fs.ModeSymlink == 0 {
		t.Errorf("symlink: got %+v", got[3])
	}
	if got[4].Path != "/unquoted" {
		t.Errorf("unquoted: got %+v", got[4])
	}
}

func TestParseFindOutput_NoUsableOutput(t *testing.T) {
	for name, output := range map[string]string{
		"empty":        "",
		"shell error":  "OCI runtime exec failed: exec: \"/bin/sh\": not found\n",
		"empty gnu":    "gnu\n",
		"empty stat":   "stat\n",
		"partial head": "gn",
	} {
		t.Run(name, func(t *testing.T) {
			got, err := collectEntries(t, output)
			if !errors.Is(err, errNoFileTreeOutput) {
				t.Errorf("err: got %v, want errNoFileTreeOutput", err)
			}
			if len(got) != 0 {
				t.Errorf("got %d entries, want 0", len(got))
			}
		})
	}
}

func TestParseFindOutput_Batches(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("stat\n")
	total := FileTreeBatchSize*2 + 3
	for i := range total {
		fmt.Fprintf(&sb, "81a4 1 '/f%d'\n", i)
	}

	var batches []int
	n, err := parseFindOutput(strings.NewReader(sb.String()), func(entries []FileEntry) {
		batches = append(batches, len(entries))
	})
	if err != nil || n != total {
		t.Fatalf("got n=%d err=%v, want n=%d", n, err, total)
	}
	want := []int{FileTreeBatchSize, FileTreeBatchSize, 3}
	if fmt.Sprint(batches) != fmt.Sprint(want) {
		t.Errorf("batches: got %v, want %v", batches, want)
	}
}
