package middleware

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/saltbo/zpan/internal/pkg/auth"
	"github.com/saltbo/zpan/internal/pkg/logger"
	"github.com/spf13/viper"
)

type Config struct {
	ShareAllFiles bool `json:"share_all_files"`
}

type Input struct {
	Uid        int64       `json:"uid"`
	Roles      []string    `json:"roles"`
	Path       string      `json:"path"`
	Method     string      `json:"method"`
	PathParams []gin.Param `json:"path_params"`
	Resource   any         `json:"resource"`
	Config     Config      `json:"config"`
}

//go:embed opa.rego
var oparules string

func OpaMiddleware(c *gin.Context) {
	// Check authorization BEFORE processing the request
	// This prevents unnecessary resource initialization and database queries for unauthorized requests
	input := &Input{
		Uid:        auth.UidGet(c),
		Roles:      c.GetStringSlice("role"),
		Path:       c.Request.URL.Path,
		Method:     c.Request.Method,
		PathParams: c.Params,
		Resource:   nil, // Resource is not available before processing the request
		Config: Config{
			ShareAllFiles: viper.GetBool("share.all_files"),
		},
	}

	if rs, err := decision(c, input); err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	} else if !rs.Allowed() {
		if input.Uid == 0 {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	// Authorization passed, now process the request with response buffering for audit checks
	bw := NewWriter(c.Writer)
	c.Writer = bw
	c.Next()

	// Optionally: Perform post-response audit checks here
	// by accessing bw.extractResource() if needed
	bw.WriteNow()
}

func decision(ctx context.Context, input *Input) (rego.ResultSet, error) {
	r := rego.New(
		rego.Query("data.middleware.allow"),
		rego.Module("opa.rego", oparules),
	)

	query, err := r.PrepareForEval(ctx)
	if err != nil {
		return nil, err
	}

	return query.Eval(ctx, rego.EvalInput(input))
}

type Writer struct {
	gin.ResponseWriter

	// Buffer for small responses (for audit checks)
	buffer *bytes.Buffer

	// Whether buffer contains the complete response or if response was streamed
	isComplete bool

	// Current buffer size (to prevent unbounded growth)
	bufferSize int
}

// maxBufferSize is the maximum size of response we'll buffer in memory (1MB)
// Larger responses are streamed directly without buffering
const maxBufferSize = 1024 * 1024 // 1MB

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
	if w.bufferSize >= maxBufferSize {
		return w.ResponseWriter.Write(p)
	}

	// Check if this write would exceed our buffer limit
	newSize := w.bufferSize + len(p)
	if newSize > maxBufferSize {
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
