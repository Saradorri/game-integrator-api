package middleware

import (
	"bytes"
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/saradorri/gameintegrator/internal/infrastructure/logger"
)

// responseWriter wraps gin.ResponseWriter to capture response data
type responseWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w responseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// LoggerMiddleware creates a middleware that logs HTTP requests in structured format
func LoggerMiddleware(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		blw := &responseWriter{
			ResponseWriter: c.Writer,
			body:           bytes.NewBufferString(""),
		}
		c.Writer = blw
		c.Next()

		latency := time.Since(start)
		latencyStr := latency.String()

		clientIP := c.ClientIP()

		requestID := ""
		if id, exists := c.Get("request_id"); exists {
			requestID = id.(string)
		}

		ctx := c.Request.Context()
		if requestID != "" {
			ctx = context.WithValue(ctx, "request_id", requestID)
		}

		log.WithRequest(
			ctx,
			c.Request.Method,
			c.Request.URL.Path,
			clientIP,
			c.Writer.Status(),
			latencyStr,
			blw.body.Len(),
		).Info("HTTP Request Processed")
	}
}
