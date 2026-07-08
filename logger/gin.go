package logger

import (
	"fmt"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	colorReset      = "\033[0m"
	colorGreen      = "\033[32m"
	colorYellow     = "\033[33m"
	colorRed        = "\033[31m"
	colorCyan       = "\033[36m"
	colorGreenBg    = "\033[42;30m"
	colorYellowBg   = "\033[43;30m"
	colorRedBg      = "\033[41;97m"
	colorCyanBg     = "\033[46;30m"
	colorGray       = "\033[90m"
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
		durationMs := latency.Milliseconds()
		if durationMs == 0 && latency > 0 {
			durationMs = 1
		}

		logTime := start.Format("2006-01-02 15:04:05.000")
		writeHTTPLogLine(method, path, int64(status), durationMs, logTime, clientIP)
	}
}

func writeHTTPLogLine(method, path string, status int64, durationMs int64, logTime, clientIP string) {
	methodColor := colorGray
	statusColor := colorGreenBg
	switch {
	case status >= 500:
		methodColor = colorRed
		statusColor = colorRedBg
	case status >= 400:
		methodColor = colorYellow
		statusColor = colorYellowBg
	case status >= 300:
		methodColor = colorCyan
		statusColor = colorCyanBg
	default:
		methodColor = colorGreen
		statusColor = colorGreenBg
	}

	method = fmt.Sprintf("%-7s", method)
	path = truncatePath(path, 28)
	path = fmt.Sprintf("%-28s", path)
	statusText := fmt.Sprintf("%3d", status)
	_, _ = fmt.Fprintf(os.Stdout, "%s%s%s %s%s%s %6dms %-23s %-15s%s\n", methodColor, method, colorReset, statusColor, statusText, colorReset, durationMs, logTime, clientIP, colorReset)
}

func truncatePath(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}

	keepFront := (max - 3) / 2
	keepBack := max - 3 - keepFront
	return s[:keepFront] + "..." + s[len(s)-keepBack:]
}
