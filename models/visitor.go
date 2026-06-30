package models

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

type VisitorSummary struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	Date      string    `gorm:"type:date;uniqueIndex;not null" json:"date"` // 统计日期
	UV        int64     `gorm:"default:0" json:"uv"`                        // 真实访客人数cookie + uuid唯一标识
	PV        int64     `gorm:"default:0" json:"pv"`                        // 总访问量
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Visitor struct {
	ID              uint      `gorm:"primarykey" json:"id"`
	VisitorID       string    `gorm:"type:varchar(64);uniqueIndex;not null" json:"visitor_id"`
	UserName        string    `gorm:"type:varchar(100)" json:"user_name"`
	Avatar          string    `gorm:"type:varchar(255)" json:"avatar"`
	IP              string    `gorm:"type:varchar(64)" json:"ip"`
	Country         string    `gorm:"type:varchar(100)" json:"country"`
	City            string    `gorm:"type:varchar(100)" json:"city"`
	Location        string    `gorm:"type:varchar(255)" json:"location"`
	FirstSeen       time.Time `gorm:"type:datetime;not null" json:"first_seen"`
	LastSeen        time.Time `gorm:"type:datetime;not null" json:"last_seen"`
	TotalBrowseTime int64     `gorm:"default:0;not null" json:"total_browse_time"`
	OS              string    `gorm:"type:varchar(50)" json:"os"`
	Browser         string    `gorm:"type:varchar(50)" json:"browser"`
	Device          string    `gorm:"type:varchar(50)" json:"device"`
	Status          string    `gorm:"type:varchar(50);default:offline" json:"status"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (VisitorSummary) TableName() string {
	return "visitor_summary"
}

func (VisitorSummary) Upsert(date string, uv, pv int64) {
	// Insert or update if date already exists
	DB.Where("date = ?", date).Assign(VisitorSummary{UV: uv, PV: pv}).FirstOrCreate(&VisitorSummary{
		Date: date,
		UV:   uv,
		PV:   pv,
	})
}

func (VisitorSummary) GetStats() *VisitorSummary {
	var stats VisitorSummary
	DB.Where("date = ?", time.Now().Format("2006-01-02")).First(&stats)
	return &stats
}

func (visitor *Visitor) UpsertVisit(duration int64) (*Visitor, error) {
	now := time.Now()
	var existing Visitor
	err := DB.Where("visitor_id = ?", visitor.VisitorID).First(&existing).Error
	if err == nil {
		err = DB.Model(&existing).Updates(map[string]any{
			"ip":                visitor.IP,
			"country":           visitor.Country,
			"city":              visitor.City,
			"location":          visitor.Location,
			"last_seen":         now,
			"total_browse_time": existing.TotalBrowseTime + duration,
			"os":                visitor.OS,
			"browser":           visitor.Browser,
			"device":            visitor.Device,
			"status":            visitor.Status,
			"user_name":         visitor.UserName,
			"avatar":            visitor.Avatar,
		}).Error
		if err != nil {
			return nil, err
		}
		existing.IP = visitor.IP
		existing.Country = visitor.Country
		existing.City = visitor.City
		existing.Location = visitor.Location
		existing.LastSeen = now
		existing.TotalBrowseTime += duration
		existing.OS = visitor.OS
		existing.Browser = visitor.Browser
		existing.Device = visitor.Device
		existing.Status = visitor.Status
		existing.UserName = visitor.UserName
		existing.Avatar = visitor.Avatar
		return &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	visitor.FirstSeen = now
	visitor.LastSeen = now
	visitor.TotalBrowseTime = duration
	return visitor, DB.Create(visitor).Error
}

func (Visitor) GetAllVisitors() ([]Visitor, error) {
	var visitors []Visitor
	err := DB.Order("last_seen desc").Find(&visitors).Error
	return visitors, err
}

func (visitor *Visitor) AddDuration(duration int64) error {
	if duration < 0 {
		duration = 0
	}
	return DB.Model(&Visitor{}).
		Where("visitor_id = ?", visitor.VisitorID).
		Updates(map[string]any{
			"last_seen":         time.Now(),
			"total_browse_time": gorm.Expr("total_browse_time + ?", duration),
		}).Error
}
