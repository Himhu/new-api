package service

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"

	"github.com/gin-gonic/gin"
)

// allowedUpstreamHeaders lists response headers we relay to end clients.
// Everything else — upstream Request-Id, rate-limit, tracing, server identity —
// is dropped so downstream cannot observe which upstream we called.
var allowedUpstreamHeaders = func() map[string]struct{} {
	names := []string{
		"Content-Type",
		"Content-Encoding",
		"Content-Language",
		"Content-Disposition",
		"Content-Range",
		"Accept-Ranges",
		"Cache-Control",
		"ETag",
		"Last-Modified",
		"Expires",
		"Retry-After",
		"Vary",
	}
	m := make(map[string]struct{}, len(names))
	for _, n := range names {
		m[http.CanonicalHeaderKey(n)] = struct{}{}
	}
	return m
}()

// SanitizeUpstreamHeaders copies whitelisted response headers from src to dst,
// dropping any header that could leak upstream identity or per-request IDs.
// Content-Length is never copied; each caller sets its own.
func SanitizeUpstreamHeaders(dst, src http.Header) {
	if dst == nil || src == nil {
		return
	}
	for key, values := range src {
		canonical := http.CanonicalHeaderKey(key)
		if canonical == "Content-Length" {
			continue
		}
		if _, ok := allowedUpstreamHeaders[canonical]; !ok {
			continue
		}
		dst.Del(canonical)
		for _, v := range values {
			dst.Add(canonical, v)
		}
	}
}

func CloseResponseBodyGracefully(httpResponse *http.Response) {
	if httpResponse == nil || httpResponse.Body == nil {
		return
	}
	err := httpResponse.Body.Close()
	if err != nil {
		common.SysError("failed to close response body: " + err.Error())
	}
}

func IOCopyBytesGracefully(c *gin.Context, src *http.Response, data []byte) {
	if c.Writer == nil {
		return
	}

	body := io.NopCloser(bytes.NewBuffer(data))

	// We shouldn't set the header before we parse the response body, because the parse part may fail.
	// And then we will have to send an error response, but in this case, the header has already been set.
	// So the httpClient will be confused by the response.
	// For example, Postman will report error, and we cannot check the response at all.
	if src != nil {
		SanitizeUpstreamHeaders(c.Writer.Header(), src.Header)
	}

	// set Content-Length header manually BEFORE calling WriteHeader
	c.Writer.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))

	// Write header with status code (this sends the headers)
	if src != nil {
		c.Writer.WriteHeader(src.StatusCode)
	} else {
		c.Writer.WriteHeader(http.StatusOK)
	}

	_, err := io.Copy(c.Writer, body)
	if err != nil {
		logger.LogError(c, fmt.Sprintf("failed to copy response body: %s", err.Error()))
	}
	c.Writer.Flush()
}
