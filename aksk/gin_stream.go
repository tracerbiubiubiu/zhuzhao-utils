package aksk

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/gin-gonic/gin"
)

// ErrBodyStore indicates a server-side temporary body storage failure. Callers
// should retry after storage recovery rather than changing their signed body.
var ErrBodyStore = errors.New("aksk: temporary request body storage")

// GinStreamingMiddleware authenticates large request bodies with bounded memory.
// It validates headers first, hashes while spooling into a private 0600 temporary
// file, and rewinds only after authentication succeeds. No downstream handler is
// entered before verification. MaxBodyBytes applies even without Content-Length.
// tempDir empty uses os.TempDir; the caller must provision sufficient disk space.
// The temporary file is closed and removed after c.Next returns, including panic
// unwinding. Process crashes/SIGKILL require external temporary-file housekeeping.
// It preserves GinMiddleware's caller/operator attribution and onFail contract;
// ErrBodyStore maps to 503 in the default and response.AKSKFail implementations.
func GinStreamingMiddleware(v *Verifier, tempDir string, onFail func(c *gin.Context, err error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, err := v.verificationHeader(c.Request); err != nil {
			fail(c, v, err, onFail)
			return
		}
		file, err := os.CreateTemp(tempDir, "aksk-body-*")
		if err != nil {
			fail(c, v, fmt.Errorf("%w: create: %w", ErrBodyStore, err), onFail)
			return
		}
		defer func() {
			_ = file.Close()
			if err := os.Remove(file.Name()); err != nil && !errors.Is(err, os.ErrNotExist) {
				logger := v.Logger
				if logger == nil {
					logger = slog.Default()
				}
				logger.Error("aksk: temporary body cleanup failed", "file", file.Name(), "err", err)
			}
		}()
		var body io.Reader = c.Request.Body
		if body == nil {
			body = emptyBody{}
		}
		if err := v.VerifyReader(c.Request, io.TeeReader(body, streamStoreWriter{file})); err != nil {
			fail(c, v, err, onFail)
			return
		}
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			fail(c, v, fmt.Errorf("%w: rewind: %w", ErrBodyStore, err), onFail)
			return
		}
		original := c.Request.Body
		c.Request.Body = file
		defer func() { c.Request.Body = original }()
		c.Set(ContextKeyCaller, CredentialOf(c.Request))
		c.Set(ContextKeyOperator, operatorOf(c.Request))
		c.Next()
	}
}

type emptyBody struct{}

func (emptyBody) Read([]byte) (int, error) { return 0, io.EOF }

type streamStoreWriter struct{ io.Writer }

func (w streamStoreWriter) Write(p []byte) (int, error) {
	n, err := w.Writer.Write(p)
	if err != nil {
		return n, fmt.Errorf("%w: write: %w", ErrBodyStore, err)
	}
	return n, nil
}
