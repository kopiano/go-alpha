package controller

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	musicCache   []byte
	musicCacheMu sync.RWMutex
	musicCachedAt time.Time
	musicTTL     = 30 * time.Minute
)

func MusicList(c *gin.Context) {
	musicCacheMu.RLock()
	valid := musicCache != nil && time.Since(musicCachedAt) < musicTTL
	musicCacheMu.RUnlock()

	if valid {
		c.Data(http.StatusOK, "application/json; charset=utf-8", musicCache)
		return
	}

	script, _ := filepath.Abs("cmd/music.py")
	cmd := exec.Command("python3", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to fetch music list",
			"details": string(output),
		})
		return
	}

	// Validate JSON before caching
	var result map[string]any
	if err := json.Unmarshal(output, &result); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse response"})
		return
	}

	musicCacheMu.Lock()
	musicCache = output
	musicCachedAt = time.Now()
	musicCacheMu.Unlock()

	code := 200
	if c, ok := result["code"].(float64); ok {
		code = int(c)
	}
	c.JSON(code, result)
}
