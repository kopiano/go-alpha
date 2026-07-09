package controller

import (
	"encoding/json"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go-alpha/response"
)

type stockCacheEntry struct {
	data    map[string]any
	expires time.Time
}

var (
	stockCacheMu sync.RWMutex
	stockCache   = map[string]stockCacheEntry{}
)

const stockTTL = 30 * time.Second

func stockCacheKey(symbol, rangeKey string) string {
	return strings.ToUpper(symbol) + ":" + rangeKey
}

func getStockCache(symbol, rangeKey string) (map[string]any, bool) {
	key := stockCacheKey(symbol, rangeKey)
	stockCacheMu.RLock()
	defer stockCacheMu.RUnlock()
	entry, ok := stockCache[key]
	if !ok || time.Now().After(entry.expires) {
		return nil, false
	}
	return entry.data, true
}

func setStockCache(symbol, rangeKey string, data map[string]any) {
	key := stockCacheKey(symbol, rangeKey)
	stockCacheMu.Lock()
	defer stockCacheMu.Unlock()
	stockCache[key] = stockCacheEntry{data: data, expires: time.Now().Add(stockTTL)}
}

func GetStockQuote(c *gin.Context) {
	symbol := strings.ToUpper(strings.TrimSpace(c.DefaultQuery("symbol", "SPCX")))
	rangeKey := strings.ToLower(strings.TrimSpace(c.DefaultQuery("range", "5d")))
	if rangeKey == "" {
		rangeKey = "5d"
	}
	if cached, ok := getStockCache(symbol, rangeKey); ok {
		response.Success("ok", cached, c)
		return
	}

	script, _ := filepath.Abs("cmd/stocks.py")
	cmd := exec.Command("python3", script, "--symbol", symbol, "--range", rangeKey)
	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("stocks: python failed", "error", err, "output", string(output))
		response.Failed("failed to fetch stock data", c)
		return
	}

	var result map[string]any
	if err := json.Unmarshal(output, &result); err != nil {
		slog.Error("stocks: parse failed", "error", err, "output", string(output))
		response.Failed("failed to parse stock data", c)
		return
	}

	code := 200
	if v, ok := result["code"].(float64); ok {
		code = int(v)
	}
	if code != 200 {
		response.Failed("failed to fetch stock data", c)
		return
	}

	if data, ok := result["data"].(map[string]any); ok {
		setStockCache(symbol, rangeKey, data)
		response.Success("ok", data, c)
		return
	}
	response.Success("ok", result, c)
}
