package client

import (
	"archive/tar"
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strconv"
	"strings"
)

// FileTreeBatchSize is the number of entries collected before they are handed
// to the FileTree emit callback.
const FileTreeBatchSize = 1000

const (
	findOutputBufSize = 64 * 1024
	gnuFindFields     = 5
	statFields        = 3
	octalBase         = 8
	hexBase           = 16
	modeBitSize       = 32
)

// Output format markers printed by fileTreeScript before the listing.
const (
	findFormatGNU  = "gnu"
	findFormatStat = "stat"
)

// fileTreeScript lists the container filesystem metadata without reading file
// contents. GNU find prints NUL separated fields; images that ship busybox
// (e.g. alpine) lack -printf, so fall back to `stat` there. -xdev keeps the walk
// on the root filesystem (skipping /proc, /sys and volumes), like an export.
const fileTreeScript = `if find / -maxdepth 0 -printf '' >/dev/null 2>&1; then
  echo ` + findFormatGNU + `
  exec find / -xdev -printf '%y\0%s\0%m\0%p\0%l\0' 2>/dev/null
fi
echo ` + findFormatStat + `
exec find / -xdev -exec stat -c '%f %s %N' {} + 2>/dev/null`

// errNoFileTreeOutput is returned when the listing command produced nothing
// usable (no shell, no find, container not running...).
var errNoFileTreeOutput = errors.New("file listing produced no output")

// FileEntry is the metadata of a single filesystem entry inside a container.
type FileEntry struct {
	Path     string
	IsDir    bool
	Linkname string
	Size     int64
	Mode     fs.FileMode
}

// FileTreeBuilder incrementally assembles a FileNode tree from FileEntry
// values. Entries may arrive in any order; missing parent directories are
// created on demand. Node paths are absolute ("/etc/hosts"). It is not safe
// for concurrent use.
type FileTreeBuilder struct {
	root  *FileNode
	dirs  map[string]*FileNode
	count int
}

// NewFileTreeBuilder returns a builder holding an empty root node.
func NewFileTreeBuilder() *FileTreeBuilder {
	root := &FileNode{Name: ".", Path: ".", IsDir: true}
	return &FileTreeBuilder{root: root, dirs: map[string]*FileNode{"": root}}
}

// Root returns the root node of the tree built so far.
func (b *FileTreeBuilder) Root() *FileNode {
	return b.root
}

// Len returns the number of nodes (excluding the root) in the tree.
func (b *FileTreeBuilder) Len() int {
	return b.count
}

// Add inserts an entry into the tree.
func (b *FileTreeBuilder) Add(e FileEntry) {
	p := normalizeEntryPath(e.Path)
	if p == "" {
		return
	}

	if e.IsDir {
		dir := b.ensureDir(p)
		dir.Mode = e.Mode
		return
	}

	parentPath, name := splitEntryPath(p)
	parent := b.ensureDir(parentPath)
	parent.Children = append(parent.Children, &FileNode{
		Name:     name,
		Path:     "/" + p,
		Linkname: e.Linkname,
		Size:     e.Size,
		Mode:     e.Mode,
		Parent:   parent,
		Depth:    parent.Depth + 1,
	})
	b.count++
}

func (b *FileTreeBuilder) ensureDir(p string) *FileNode {
	if n, ok := b.dirs[p]; ok {
		return n
	}
	parentPath, name := splitEntryPath(p)
	parent := b.ensureDir(parentPath)
	n := &FileNode{Name: name, Path: "/" + p, IsDir: true, Parent: parent, Depth: parent.Depth + 1}
	parent.Children = append(parent.Children, n)
	b.dirs[p] = n
	b.count++
	return n
}

// normalizeEntryPath turns "/etc/", "./etc" or "etc" into "etc"; the root
// directory becomes "". The result keys the builder's directory index.
func normalizeEntryPath(p string) string {
	return strings.TrimPrefix(path.Clean("/"+p), "/")
}

func splitEntryPath(p string) (string, string) {
	idx := strings.LastIndexByte(p, '/')
	if idx < 0 {
		return "", p
	}
	return p[:idx], p[idx+1:]
}

// batcher accumulates entries and flushes them to emit in fixed-size batches.
type batcher struct {
	emit  func([]FileEntry)
	batch []FileEntry
	total int
}

func newBatcher(emit func([]FileEntry)) *batcher {
	return &batcher{emit: emit, batch: make([]FileEntry, 0, FileTreeBatchSize)}
}

func (b *batcher) add(e FileEntry) {
	b.batch = append(b.batch, e)
	b.total++
	if len(b.batch) == FileTreeBatchSize {
		b.flush()
	}
}

func (b *batcher) flush() {
	if len(b.batch) == 0 {
		return
	}
	b.emit(b.batch)
	b.batch = make([]FileEntry, 0, FileTreeBatchSize)
}

// readTarEntries streams the headers of a tar archive to emit in batches and
// returns the number of entries read. File contents are skipped.
func readTarEntries(r io.Reader, emit func([]FileEntry)) (int, error) {
	tr := tar.NewReader(r)
	b := newBatcher(emit)
	defer b.flush()

	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return b.total, nil
		}
		if err != nil {
			return b.total, err
		}
		b.add(FileEntry{
			Path:     hdr.Name,
			IsDir:    hdr.Typeflag == tar.TypeDir,
			Linkname: hdr.Linkname,
			Size:     hdr.Size,
			Mode:     hdr.FileInfo().Mode(),
		})
	}
}

// parseFindOutput parses the output of fileTreeScript, streaming entries to
// emit in batches. It returns the number of entries parsed.
func parseFindOutput(r io.Reader, emit func([]FileEntry)) (int, error) {
	br := bufio.NewReaderSize(r, findOutputBufSize)
	format, err := br.ReadString('\n')
	if err != nil {
		if errors.Is(err, io.EOF) {
			return 0, errNoFileTreeOutput
		}
		return 0, err
	}

	b := newBatcher(emit)
	defer b.flush()

	switch strings.TrimSpace(format) {
	case findFormatGNU:
		err = parseGNUFind(br, b)
	case findFormatStat:
		err = parseStatFind(br, b)
	default:
		return 0, fmt.Errorf("%w: unexpected format %q", errNoFileTreeOutput, format)
	}

	if err == nil && b.total == 0 {
		err = errNoFileTreeOutput
	}
	return b.total, err
}

// parseGNUFind parses records of `find -printf '%y\0%s\0%m\0%p\0%l\0'`.
func parseGNUFind(br *bufio.Reader, b *batcher) error {
	var fields [gnuFindFields]string
	for {
		for i := range fields {
			field, err := br.ReadString(0)
			if err != nil {
				if errors.Is(err, io.EOF) && i == 0 && field == "" {
					return nil
				}
				return fmt.Errorf("reading find output: %w", err)
			}
			fields[i] = field[:len(field)-1]
		}

		entry, ok := gnuFindEntry(fields)
		if ok {
			b.add(entry)
		}
	}
}

func gnuFindEntry(fields [gnuFindFields]string) (FileEntry, bool) {
	kind, sizeField, modeField, name, link := fields[0], fields[1], fields[2], fields[3], fields[4]
	size, err := strconv.ParseInt(sizeField, 10, 64)
	if err != nil {
		return FileEntry{}, false
	}
	perm, err := strconv.ParseUint(modeField, octalBase, modeBitSize)
	if err != nil {
		return FileEntry{}, false
	}

	hdr := tar.Header{Mode: int64(perm), Typeflag: findTypeflag(kind)}
	mode := hdr.FileInfo().Mode()
	return FileEntry{Path: name, IsDir: mode.IsDir(), Linkname: link, Size: size, Mode: mode}, true
}

func findTypeflag(kind string) byte {
	switch kind {
	case "d":
		return tar.TypeDir
	case "l":
		return tar.TypeSymlink
	case "b":
		return tar.TypeBlock
	case "c":
		return tar.TypeChar
	case "p":
		return tar.TypeFifo
	default:
		return tar.TypeReg
	}
}

// parseStatFind parses lines of `stat -c '%f %s %N'` (raw hex mode, size and
// quoted name, with "-> 'target'" appended for symlinks).
func parseStatFind(br *bufio.Reader, b *batcher) error {
	for {
		line, err := br.ReadString('\n')
		if line = strings.TrimSuffix(line, "\n"); line != "" {
			if entry, ok := statEntry(line); ok {
				b.add(entry)
			}
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("reading stat output: %w", err)
		}
	}
}

func statEntry(line string) (FileEntry, bool) {
	parts := strings.SplitN(line, " ", statFields)
	if len(parts) != statFields {
		return FileEntry{}, false
	}
	raw, err := strconv.ParseUint(parts[0], hexBase, modeBitSize)
	if err != nil {
		return FileEntry{}, false
	}
	size, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return FileEntry{}, false
	}

	hdr := tar.Header{Mode: int64(raw)}
	mode := hdr.FileInfo().Mode()

	name, link := parts[2], ""
	if mode&fs.ModeSymlink != 0 {
		if before, after, found := strings.Cut(name, " -> "); found {
			name, link = before, unquoteStatName(after)
		}
	}

	return FileEntry{Path: unquoteStatName(name), IsDir: mode.IsDir(), Linkname: link, Size: size, Mode: mode}, true
}

func unquoteStatName(s string) string {
	if len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'' {
		return s[1 : len(s)-1]
	}
	return s
}
