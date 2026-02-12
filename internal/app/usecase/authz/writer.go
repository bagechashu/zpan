package authz

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/saltbo/zpan/internal/pkg/logger"
)

// MaxBufferSize is the maximum size of response we'll buffer in memory (10MB)
// Larger responses are streamed directly without buffering
const MaxBufferSize = 10 * 1024 * 1024 // 10MB

type Writer struct {
	gin.ResponseWriter

	// Buffer for small responses (for audit checks)
	buffer *bytes.Buffer

	// Whether buffer contains the complete response or if response was streamed
	isComplete bool

	// Current buffer size (to prevent unbounded growth)
	bufferSize int
}

func NewWriter(rw gin.ResponseWriter) *Writer {
	return &Writer{
		ResponseWriter: rw,
		buffer:         &bytes.Buffer{},
		isComplete:     true, // Assume complete until we exceed buffer limit
	}
}

// Write implements io.Writer interface
// Returns the number of bytes written and any error
func (w *Writer) Write(p []byte) (n int, err error) {
	// If we've already exceeded buffer size, stream directly
	if w.bufferSize >= MaxBufferSize {
		return w.ResponseWriter.Write(p)
	}

	// Check if this write would exceed our buffer limit
	newSize := w.bufferSize + len(p)
	if newSize > MaxBufferSize {
		// Buffer is full - flush what we have and mark as incomplete
		if w.buffer.Len() > 0 {
			_, _ = w.ResponseWriter.Write(w.buffer.Bytes())
			w.buffer.Reset()
		}
		w.isComplete = false
		w.bufferSize = newSize
		return w.ResponseWriter.Write(p)
	}

	// Still within buffer size - keep buffering
	w.bufferSize = newSize
	return w.buffer.Write(p)
}

// extractResource attempts to parse the buffered response as JSON for audit checking
// Returns nil if response was not completely buffered or if it's not valid JSON
func (w *Writer) extractResource() any {
	// If the response was not completely buffered (streamed), skip resource extraction
	if !w.isComplete || w.buffer.Len() == 0 {
		return nil
	}

	// Try to parse the buffered response as JSON
	var resource interface{}
	dec := json.NewDecoder(bytes.NewReader(w.buffer.Bytes()))
	dec.UseNumber()

	if err := dec.Decode(&resource); err != nil {
		// Not JSON or malformed - this is normal for non-JSON responses
		logger.Debug("Response is not JSON or failed to decode", "error", err)
		return nil
	}

	return resource
}

// WriteNow flushes the buffered content to the underlying ResponseWriter
func (w *Writer) WriteNow() (int, error) {
	if w.buffer.Len() == 0 {
		// Nothing to write
		return 0, nil
	}

	// Write buffered content to the actual response writer
	n, err := w.ResponseWriter.Write(w.buffer.Bytes())
	w.buffer.Reset()
	return n, err
}

// Flush implements http.Flusher interface to support streaming
func (w *Writer) Flush() {
	if w.isComplete && w.buffer.Len() > 0 {
		// Flush buffer first
		_, _ = w.ResponseWriter.Write(w.buffer.Bytes())
		w.buffer.Reset()
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}
