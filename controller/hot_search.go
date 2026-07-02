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

// ── 36kr in-memory cache ──

type kr36Cache struct {
	mu      sync.RWMutex
	data    map[string]any
	expires time.Time
}

var kr36 = &kr36Cache{}

const kr36TTL = 5 * time.Minute

func (c *kr36Cache) get() (map[string]any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.data != nil && time.Now().Before(c.expires) {
		return c.data, true
	}
	return nil, false
}

func (c *kr36Cache) set(data map[string]any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = data
	c.expires = time.Now().Add(kr36TTL)
}

// ── Weibo in-memory cache (keyed by date) ──

type weiboCache struct {
	mu      sync.RWMutex
	data    map[string]cacheEntry
}

type cacheEntry struct {
	result  map[string]any
	expires time.Time
}

var weibo = &weiboCache{data: make(map[string]cacheEntry)}

const weiboTtl = 3 * time.Minute

func (c *weiboCache) get(key string) (map[string]any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.data[key]
	if ok && time.Now().Before(entry.expires) {
		return entry.result, true
	}
	return nil, false
}

func (c *weiboCache) set(key string, data map[string]any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = cacheEntry{result: data, expires: time.Now().Add(weiboTtl)}
}

// ── handlers ──

func HotSearch(c *gin.Context) {
	date := c.Query("date") // MM-DD 格式，为空则代表今日实时

	cacheKey := date
	if cacheKey == "" {
		cacheKey = time.Now().Format("01-02")
	}

	// 命中缓存 → 直接返回
	if cached, ok := weibo.get(cacheKey); ok {
		cached["cached"] = true
		c.JSON(200, cached)
		return
	}

	script, _ := filepath.Abs("cmd/weibo.py")

	args := []string{script, "--json"}
	if date != "" {
		args = append(args, "--date", date)
	}

	cmd := exec.Command("python3", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to fetch hot search",
			"details": string(output),
		})
		return
	}

	var result map[string]any
	if err := json.Unmarshal(output, &result); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse response"})
		return
	}

	// 缓存（仅成功时）
	code := 200
	if c, ok := result["code"].(float64); ok {
		code = int(c)
	}
	if code == 200 {
		weibo.set(cacheKey, result)
	}

	// 透传 Python 返回的状态码
	c.JSON(code, result)
}

func Kr36Hot(c *gin.Context) {
	// 命中缓存 → 直接返回
	if cached, ok := kr36.get(); ok {
		cached["cached"] = true
		c.JSON(200, cached)
		return
	}

	script, _ := filepath.Abs("cmd/36kr.py")

	cmd := exec.Command("python3", script, "--json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to fetch 36kr hot list",
			"details": string(output),
		})
		return
	}

	var result map[string]any
	if err := json.Unmarshal(output, &result); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse response"})
		return
	}

	// 写入缓存（仅成功时）
	code := 200
	if c, ok := result["code"].(float64); ok {
		code = int(c)
	}
	if code == 200 {
		kr36.set(result)
	}

	c.JSON(code, result)
}
