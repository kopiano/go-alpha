package logger

import (
	"io"
	"log/slog"
	"os"
	"time"
)

const (
	LevelTrace = slog.Level(-8)
	LevelDebug = slog.LevelDebug
	LevelInfo  = slog.LevelInfo
	LevelWarn  = slog.LevelWarn
	LevelError = slog.LevelError
)

func init() {
	opts := &slog.HandlerOptions{
		Level:     slog.LevelDebug,
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// 统一时间格式，去掉纳秒
			if a.Key == slog.TimeKey {
				if t, ok := a.Value.Any().(time.Time); ok {
					return slog.String(slog.TimeKey, t.Format("2006-01-02 15:04:05.000"))
				}
			}
			// 简化源文件路径为 pkg/file.go:line
			if a.Key == slog.SourceKey {
				if src, ok := a.Value.Any().(*slog.Source); ok {
					short := shortFile(src.File)
					return slog.String(slog.SourceKey, short)
				}
			}
			return a
		},
	}

	// 同时输出到 stdout 和根目录日志文件 app.log
	logFile, err := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		// 文件打开失败时只输出到 stdout
		handler := slog.NewTextHandler(os.Stdout, opts)
		slog.SetDefault(slog.New(handler))
		return
	}

	multiWriter := io.MultiWriter(os.Stdout, logFile)
	handler := slog.NewTextHandler(multiWriter, opts)
	slog.SetDefault(slog.New(handler))
}

// shortFile 截取 src/pkg/file.go 最后两级
func shortFile(file string) string {
	slash := 0
	for i := len(file) - 1; i >= 0; i-- {
		if file[i] == '/' {
			slash++
			if slash == 2 {
				return file[i+1:]
			}
		}
	}
	return file
}
