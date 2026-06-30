package logger

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// GinMiddleware 记录每个 HTTP 请求的方法、路径、状态码、耗时和客户端 IP
func GinMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		status := c.Writer.Status()
		latency := time.Since(start)
		clientIP := c.ClientIP()
		method := c.Request.Method
		bodySize := c.Writer.Size()

		attrs := []slog.Attr{
			slog.Int("status", status),
			slog.String("method", method),
			slog.String("path", path),
			slog.String("ip", clientIP),
			slog.Duration("took", latency),
			slog.Int("size", bodySize),
		}

		if len(c.Errors) > 0 {
			attrs = append(attrs, slog.String("err", c.Errors.String()))
		}

		switch {
		case status >= 500:
			slog.LogAttrs(c.Request.Context(), slog.LevelError, "[HTTP]", attrs...)
		case status >= 400:
			slog.LogAttrs(c.Request.Context(), slog.LevelWarn, "[HTTP]", attrs...)
		default:
			slog.LogAttrs(c.Request.Context(), slog.LevelInfo, "[HTTP]", attrs...)
		}
	}
}
