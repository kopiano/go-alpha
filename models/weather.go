package models

import (
	"time"
)

// Weather 天气数据（按城市+日期去重）
type Weather struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	City        string    `gorm:"type:varchar(50);uniqueIndex:idx_weather_city_date;not null" json:"city"`
	Date        string    `gorm:"type:date;uniqueIndex:idx_weather_city_date;not null" json:"date"`
	WeekDay     string    `gorm:"type:varchar(10)" json:"week_day"`
	TempHigh    float64   `json:"temp_high"`
	TempLow     float64   `json:"temp_low"`
	TempCurrent float64   `json:"temp_current"`
	AQI         int       `json:"aqi"`
	Condition   string    `gorm:"type:varchar(100)" json:"condition"`
	Humidity    string    `gorm:"type:varchar(10)" json:"humidity"`
	Wind        string    `gorm:"type:varchar(10)" json:"wind"`
	UV          string    `gorm:"type:varchar(5)" json:"uv"`
	Sunrise     string    `gorm:"type:varchar(20)" json:"sunrise"`
	Sunset      string    `gorm:"type:varchar(20)" json:"sunset"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// UpsertWeather 写入或更新天气数据，按 city+date 去重
func UpsertWeather(w *Weather) error {
	return DB.Where("city = ? AND date = ?", w.City, w.Date).
		Assign(map[string]any{
			"week_day":     w.WeekDay,
			"temp_high":    w.TempHigh,
			"temp_low":     w.TempLow,
			"temp_current": w.TempCurrent,
			"aqi":          w.AQI,
			"condition":    w.Condition,
			"humidity":     w.Humidity,
			"wind":         w.Wind,
			"uv":           w.UV,
			"sunrise":      w.Sunrise,
			"sunset":       w.Sunset,
		}).
		FirstOrCreate(w).Error
}

// GetWeather 查询指定城市最新天气（默认当天）
func GetWeather(city string, date ...string) (*Weather, error) {
	d := time.Now().Format("2006-01-02")
	if len(date) > 0 && date[0] != "" {
		d = date[0]
	}
	var w Weather
	err := DB.Where("city = ? AND date = ?", city, d).First(&w).Error
	if err != nil {
		return nil, err
	}
	return &w, nil
}

// GetWeatherRange 查询指定城市日期范围内的天气（按日期升序）
func GetWeatherRange(city, startDate, endDate string) ([]Weather, error) {
	var list []Weather
	err := DB.Where("city = ? AND date BETWEEN ? AND ?", city, startDate, endDate).
		Order("date ASC").Find(&list).Error
	return list, err
}
