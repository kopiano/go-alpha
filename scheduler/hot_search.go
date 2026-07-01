package scheduler

import (
	"log/slog"
	"os/exec"
	"path/filepath"
	"time"
)

const hotSearchInterval = 6 * time.Hour

// StartHotSearchScheduler 启动定时抓取微博热搜，每 6 小时执行一次
func StartHotSearchScheduler() {
	slog.Info("Hot search scheduler started, interval: 6h")

	// 启动后立即抓取一次
	go runHotSearch()

	// 每 6 小时轮询
	ticker := time.NewTicker(hotSearchInterval)
	go func() {
		for range ticker.C {
			runHotSearch()
		}
	}()
}

func runHotSearch() {
	slog.Info("Scheduled hot search fetch starting...")

	script, _ := filepath.Abs("cmd/weibo.py")
	cmd := exec.Command("python3", script, "--json")
	output, err := cmd.CombinedOutput()

	if err != nil {
		slog.Error("Scheduled hot search failed", "error", err, "output", string(output))
		return
	}

	slog.Info("Scheduled hot search completed", "output_size", len(output))
}
