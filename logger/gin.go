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
		line := fmt.Sprintf("%s %s %d %dms %s %s",
			method,
			path,
			status,
			durationMs,
			logTime,
			clientIP,
		)
		writeHTTPLogLine(line, status)
	}
}

func writeHTTPLogLine(line string, status int) {
	textColor := colorGray
	statusColor := colorGreenBg
	switch {
	case status >= 500:
		textColor = colorRed
		statusColor = colorRedBg
	case status >= 400:
		textColor = colorYellow
		statusColor = colorYellowBg
	case status >= 300:
		textColor = colorCyan
		statusColor = colorCyanBg
	default:
		textColor = colorGreen
		statusColor = colorGreenBg
	}

	parts := splitHTTPLine(line)
	if len(parts) != 6 {
		_, _ = fmt.Fprintln(os.Stdout, textColor+line+colorReset)
		return
	}

	_, _ = fmt.Fprintf(os.Stdout, "%s%s %s%s%s %s%s %s %s %s%s\n",
		textColor,
		parts[0],
		parts[1],
		statusColor,
		parts[2],
		colorReset,
		textColor,
		parts[3],
		parts[4],
		parts[5],
		colorReset,
	)
}

func splitHTTPLine(line string) []string {
	fields := make([]string, 0, 6)
	start := 0
	for i := 0; i < len(line); i++ {
		if line[i] == ' ' {
			if start < i {
				fields = append(fields, line[start:i])
			}
			start = i + 1
		}
	}
	if start < len(line) {
		fields = append(fields, line[start:])
	}
	return fields
}
