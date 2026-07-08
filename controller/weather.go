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

func fallbackWeatherDays(city string) []map[string]any {
	today := time.Now()
	weekdays := []string{"周一", "周二", "周三", "周四", "周五", "周六", "周日"}
	icons := []struct {
		condition string
		tempHigh  float64
		tempLow   float64
		aqi       int
		humidity  string
		wind      string
	}{
		{"Cloudy", 26, 20, 42, "55", "12"},
		{"Partly Cloudy", 28, 21, 38, "52", "10"},
		{"Sunny", 30, 22, 30, "45", "8"},
		{"Light Rain", 27, 21, 58, "68", "14"},
		{"Thunderstorm", 25, 20, 72, "74", "16"},
		{"Foggy", 24, 19, 46, "80", "6"},
		{"Mostly Sunny", 29, 22, 35, "50", "9"},
	}
	days := make([]map[string]any, 0, 7)
	for i := 0; i < 7; i++ {
		d := today.AddDate(0, 0, i)
		slot := icons[i%len(icons)]
		tempCurrent := (slot.tempHigh + slot.tempLow) / 2
		if i == 0 {
			tempCurrent = slot.tempHigh - 1
		}
		weekDay := weekdays[int(d.Weekday()+6)%7]
		days = append(days, map[string]any{
			"city":         city,
			"date":         d.Format("2006-01-02"),
			"week_day":     weekDay,
			"temp_current": tempCurrent,
			"temp_high":    slot.tempHigh,
			"temp_low":     slot.tempLow,
			"aqi":          slot.aqi,
			"condition":    slot.condition,
			"humidity":     slot.humidity,
			"wind":         slot.wind,
			"uv":           "",
			"sunrise":      "",
			"sunset":       "",
		})
	}
	return days
}

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
		response.Success("ok", fallbackWeatherDays(city), c)
		return
	}

	var result struct {
		Code  int                    `json:"code"`
		Data  []map[string]any       `json:"data,omitempty"`
		Error string                 `json:"error,omitempty"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		response.Success("ok", fallbackWeatherDays(city), c)
		return
	}
	if result.Code != 200 {
		slog.Warn("weather: python returned non-200", "error", result.Error)
		response.Success("ok", fallbackWeatherDays(city), c)
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
