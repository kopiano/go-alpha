package controller

import (
	"encoding/json"
	"log/slog"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go-alpha/models"
	"go-alpha/response"
)

// ── in-memory cache ──

type weatherCache struct {
	mu      sync.RWMutex
	data    map[string]any
	expires time.Time
}

var weatherCached = &weatherCache{}
const weatherTTL = 10 * time.Minute

func (c *weatherCache) get() (map[string]any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.data != nil && time.Now().Before(c.expires) {
		return c.data, true
	}
	return nil, false
}

func (c *weatherCache) set(data map[string]any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = data
	c.expires = time.Now().Add(weatherTTL)
}

// ── handler ──

func GetWeather(c *gin.Context) {
	city := c.DefaultQuery("city", "杭州")

	// 1) 尝试读 MySQL（当天数据有则直接返回）
	if w, err := models.GetWeather(city); err == nil {
		response.Success("ok", gin.H{
			"city":         w.City,
			"date":         w.Date,
			"week_day":     w.WeekDay,
			"temp_current": w.TempCurrent,
			"temp_high":    w.TempHigh,
			"temp_low":     w.TempLow,
			"aqi":          w.AQI,
			"condition":    w.Condition,
		}, c)
		return
	}

	// 2) 内存缓存
	if cached, ok := weatherCached.get(); ok {
		response.Success("ok", cached, c)
		return
	}

	// 3) 调用 Python 爬取
	script, _ := filepath.Abs("cmd/weather.py")
	cmd := exec.Command("python3", script, "--city", city)
	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("weather: python failed", "error", err, "output", string(output))
		response.Failed("获取天气失败", c)
		return
	}

	var result struct {
		Code  int                    `json:"code"`
		Data  map[string]any         `json:"data,omitempty"`
		Error string                 `json:"error,omitempty"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		response.Failed("解析天气数据失败", c)
		return
	}
	if result.Code != 200 {
		response.Failed(result.Error, c)
		return
	}

	data := result.Data

	// 4) 写入 MySQL
	weekDay, _ := data["week_day"].(string)
	tempHigh, _ := data["temp_high"].(float64)
	tempLow, _ := data["temp_low"].(float64)
	tempCur, _ := data["temp_current"].(float64)
	aqi, _ := data["aqi"].(float64)
	condition, _ := data["condition"].(string)

	_ = models.UpsertWeather(&models.Weather{
		City:        city,
		Date:        time.Now().Format("2006-01-02"),
		WeekDay:     weekDay,
		TempHigh:    tempHigh,
		TempLow:     tempLow,
		TempCurrent: tempCur,
		AQI:         int(aqi),
		Condition:   condition,
	})

	// 5) 内存缓存
	weatherCached.set(data)

	response.Success("ok", data, c)
}
