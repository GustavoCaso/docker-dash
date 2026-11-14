package containers

import (
	"errors"
	"io"
	"sync"
)

var errQueuedWriterClosed = errors.New("exec writer closed")

// queuedWriter decouples the terminal emulator from the exec connection.
//
// bubbleterm holds its emulator lock while writing input, and its read loop
// needs the same lock after every read. When the remote side echoes input
// before the write completes (always with a synchronous pipe, or with a large
// paste once socket buffers fill) both sides block forever. queuedWriter makes
// Write non-blocking: data is appended to an ordered queue and a single
// goroutine flushes it to the underlying writer, preserving byte order.
type queuedWriter struct {
	dst io.WriteCloser

	mu     sync.Mutex
	cond   *sync.Cond
	queue  [][]byte
	closed bool
	err    error
}

func newQueuedWriter(dst io.WriteCloser) *queuedWriter {
	w := &queuedWriter{dst: dst}
	w.cond = sync.NewCond(&w.mu)
	go w.flush()
	return w
}

func (w *queuedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return 0, errQueuedWriterClosed
	}
	if w.err != nil {
		return 0, w.err
	}
	w.queue = append(w.queue, append([]byte(nil), p...))
	w.cond.Signal()
	return len(p), nil
}

// Close stops the flusher and closes the underlying writer. Pending data is
// dropped: Close is only used when the session is torn down.
func (w *queuedWriter) Close() error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil
	}
	w.closed = true
	w.queue = nil
	w.cond.Signal()
	w.mu.Unlock()
	return w.dst.Close()
}

func (w *queuedWriter) flush() {
	for {
		w.mu.Lock()
		for len(w.queue) == 0 && !w.closed {
			w.cond.Wait()
		}
		if w.closed {
			w.mu.Unlock()
			return
		}
		chunk := w.queue[0]
		w.queue = w.queue[1:]
		w.mu.Unlock()

		if _, err := w.dst.Write(chunk); err != nil {
			w.mu.Lock()
			w.err = err
			w.queue = nil
			w.mu.Unlock()
			return
		}
	}
}
