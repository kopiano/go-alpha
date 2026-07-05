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
	data    []map[string]any
	expires time.Time
}

var weatherCached = &weatherCache{}
const weatherTTL = 10 * time.Minute

func (c *weatherCache) get() ([]map[string]any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.data != nil && time.Now().Before(c.expires) {
		return c.data, true
	}
	return nil, false
}

func (c *weatherCache) set(data []map[string]any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = data
	c.expires = time.Now().Add(weatherTTL)
}

// ── handler ──

func GetWeather(c *gin.Context) {
	city := c.DefaultQuery("city", "杭州")
	today := time.Now().Format("2006-01-02")
	weekLater := time.Now().Add(6 * 24 * time.Hour).Format("2006-01-02")

	// 1) 尝试读 MySQL（如果 7 天都有则直接返回数组）
	if list, err := models.GetWeatherRange(city, today, weekLater); err == nil && len(list) == 7 {
		days := make([]map[string]any, 0, 7)
		for _, w := range list {
			days = append(days, map[string]any{
				"city":         w.City,
				"date":         w.Date,
				"week_day":     w.WeekDay,
				"temp_current": w.TempCurrent,
				"temp_high":    w.TempHigh,
				"temp_low":     w.TempLow,
				"aqi":          w.AQI,
				"condition":    w.Condition,
				"humidity":     w.Humidity,
				"wind":         w.Wind,
				"uv":           w.UV,
				"sunrise":      w.Sunrise,
				"sunset":       w.Sunset,
			})
		}
		weatherCached.set(days)
		response.Success("ok", days, c)
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
		Data  []map[string]any       `json:"data,omitempty"`
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

	days := result.Data

	// 4) 逐日写入 MySQL
	for _, data := range days {
		dateStr, _ := data["date"].(string)
		if dateStr == "" {
			continue
		}
		weekDay, _ := data["week_day"].(string)
		tempHigh, _ := data["temp_high"].(float64)
		tempLow, _ := data["temp_low"].(float64)
		tempCur, _ := data["temp_current"].(float64)
		aqi, _ := data["aqi"].(float64)
		condition, _ := data["condition"].(string)
		humidity, _ := data["humidity"].(string)
		wind, _ := data["wind"].(string)
		uv, _ := data["uv"].(string)
		sunrise, _ := data["sunrise"].(string)
		sunset, _ := data["sunset"].(string)

		_ = models.UpsertWeather(&models.Weather{
			City:        city,
			Date:        dateStr,
			WeekDay:     weekDay,
			TempHigh:    tempHigh,
			TempLow:     tempLow,
			TempCurrent: tempCur,
			AQI:         int(aqi),
			Condition:   condition,
			Humidity:    humidity,
			Wind:        wind,
			UV:          uv,
			Sunrise:     sunrise,
			Sunset:      sunset,
		})
	}

	// 5) 内存缓存
	weatherCached.set(days)

	response.Success("ok", days, c)
}
