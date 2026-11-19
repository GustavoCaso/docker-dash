package containers

import (
	"time"

	"github.com/GustavoCaso/docker-dash/internal/client"
)

type cachedFileTree struct {
	root     *client.FileNode
	loadedAt time.Time
}

// fileTreeCache keeps recently loaded container file trees so revisiting a
// container (or a list refresh re-initialising the panel) does not walk the
// filesystem again. Entries expire after ttl; the oldest is evicted once
// maxEntries is reached, since large trees hold a lot of memory.
type fileTreeCache struct {
	ttl        time.Duration
	maxEntries int
	now        func() time.Time
	entries    map[string]cachedFileTree
}

func newFileTreeCache(ttl time.Duration, maxEntries int) *fileTreeCache {
	return &fileTreeCache{
		ttl:        ttl,
		maxEntries: maxEntries,
		now:        time.Now,
		entries:    make(map[string]cachedFileTree),
	}
}

func (c *fileTreeCache) get(containerID string) (*client.FileNode, bool) {
	entry, ok := c.entries[containerID]
	if !ok {
		return nil, false
	}
	if c.now().Sub(entry.loadedAt) > c.ttl {
		delete(c.entries, containerID)
		return nil, false
	}
	return entry.root, true
}

func (c *fileTreeCache) put(containerID string, root *client.FileNode) {
	if c.ttl <= 0 {
		return
	}
	if _, exists := c.entries[containerID]; !exists && len(c.entries) >= c.maxEntries {
		c.evictOldest()
	}
	c.entries[containerID] = cachedFileTree{root: root, loadedAt: c.now()}
}

func (c *fileTreeCache) evictOldest() {
	var oldestID string
	var oldest time.Time
	for id, entry := range c.entries {
		if oldestID == "" || entry.loadedAt.Before(oldest) {
			oldestID, oldest = id, entry.loadedAt
		}
	}
	delete(c.entries, oldestID)
}
