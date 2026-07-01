package models

import (
	"errors"
	"fmt"
	"hash/fnv"
	"log/slog"
	"time"

	"gorm.io/gorm"
)

type VisitorSummary struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	Date      string    `gorm:"type:date;uniqueIndex;not null" json:"date"`
	UV        int64     `gorm:"default:0" json:"uv"`
	PV        int64     `gorm:"default:0" json:"pv"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (VisitorSummary) TableName() string {
	return "visitor_summary"
}

func (VisitorSummary) Upsert(date string, uv, pv int64) {
	err := DB.Where("date = ?", date).Assign(VisitorSummary{UV: uv, PV: pv}).FirstOrCreate(&VisitorSummary{
		Date: date,
		UV:   uv,
		PV:   pv,
	}).Error
	if err != nil {
		slog.Error("VisitorSummary.Upsert failed", "date", date, "uv", uv, "pv", pv, "error", err)
	}
}

type Visitor struct {
	ID                uint      `gorm:"primarykey" json:"id"`
	VisitorID         string    `gorm:"type:varchar(64);uniqueIndex;not null" json:"visitor_id"`
	UserName          string    `gorm:"type:varchar(100)" json:"user_name"`
	Avatar            string    `gorm:"type:varchar(255)" json:"avatar"`
	IP                string    `gorm:"type:varchar(64)" json:"ip"`
	Country           string    `gorm:"type:varchar(100)" json:"country"`
	City              string    `gorm:"type:varchar(100)" json:"city"`
	Location          string    `gorm:"type:varchar(255)" json:"location"`
	FirstSeen         time.Time `gorm:"type:datetime;not null" json:"first_seen"`
	LastSeen          time.Time `gorm:"type:datetime;not null" json:"last_seen"`
	TotalBrowseTime   int64     `gorm:"default:0;not null" json:"total_browse_time"`
	VisitCount        int64     `gorm:"default:0;not null" json:"visit_count"`
	OS                string    `gorm:"type:varchar(50)" json:"os"`
	Browser           string    `gorm:"type:varchar(50)" json:"browser"`
	Device            string    `gorm:"type:varchar(50)" json:"device"`
	Status            string    `gorm:"type:varchar(50);default:inactive" json:"status"`
	DeviceFingerprint string    `gorm:"type:varchar(512)" json:"device_fingerprint"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// stableShortID 从任意 visitor_id 生成 8 位稳定短 ID（相同输入 → 相同输出）
func stableShortID(id string) string {
	h := fnv.New32a()
	h.Write([]byte(id))
	return fmt.Sprintf("%x", h.Sum32())
}

func (visitor *Visitor) UpsertVisit(duration int64) (*Visitor, error) {
	// Convert to stable short ID
	visitor.VisitorID = stableShortID(visitor.VisitorID)

	now := time.Now()
	var existing Visitor

	// 1) Look up by visitor_id first.
	err := DB.Where("visitor_id = ?", visitor.VisitorID).First(&existing).Error
	if err == nil {
		return applyVisitorUpdates(&existing, visitor, duration, now)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// 2) Not found by visitor_id — try matching by device_fingerprint + IP

	//    (handles localStorage cleared, incognito, cookie expiry).
	if visitor.DeviceFingerprint != "" {
		err = DB.Where("device_fingerprint = ? AND ip = ?", visitor.DeviceFingerprint, visitor.IP).
			First(&existing).Error
		if err == nil {
			existing.VisitorID = visitor.VisitorID
			return applyVisitorUpdates(&existing, visitor, duration, now)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}

	// 3) Fallback: match by IP + username
	if visitor.IP != "" && visitor.UserName != "" {
		err = DB.Where("ip = ? AND user_name = ?", visitor.IP, visitor.UserName).
			First(&existing).Error
		if err == nil {
			existing.VisitorID = visitor.VisitorID
			return applyVisitorUpdates(&existing, visitor, duration, now)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}

	// 5) Truly new visitor — insert.
	visitor.FirstSeen = now
	visitor.LastSeen = now
	visitor.TotalBrowseTime = duration
	visitor.VisitCount = 1
	return visitor, DB.Create(visitor).Error
}

// applyVisitorUpdates persists the merged fields and returns the updated record.
func applyVisitorUpdates(existing *Visitor, incoming *Visitor, duration int64, now time.Time) (*Visitor, error) {
	updates := map[string]any{
		"visitor_id":         existing.VisitorID,
		"device_fingerprint": incoming.DeviceFingerprint,
		"ip":                 incoming.IP,
		"country":            incoming.Country,
		"city":               incoming.City,
		"location":           incoming.Location,
		"last_seen":          now,
		"total_browse_time":  existing.TotalBrowseTime + duration,
		"visit_count":        gorm.Expr("visit_count + 1"),
		"os":                 incoming.OS,
		"browser":            incoming.Browser,
		"device":             incoming.Device,
		"status":             incoming.Status,
		"avatar":             incoming.Avatar,
	}
	// Only overwrite user_name when the incoming value is non-empty,
	// otherwise a logout (guest heartbeat) would wipe the linked username.
	if incoming.UserName != "" {
		updates["user_name"] = incoming.UserName
	}
	err := DB.Model(existing).Updates(updates).Error
	if err != nil {
		return nil, err
	}
	existing.IP = incoming.IP
	existing.Country = incoming.Country
	existing.City = incoming.City
	existing.Location = incoming.Location
	existing.LastSeen = now
	existing.TotalBrowseTime += duration
	existing.OS = incoming.OS
	existing.Browser = incoming.Browser
	existing.Device = incoming.Device
	existing.Status = incoming.Status
	if incoming.UserName != "" {
		existing.UserName = incoming.UserName
	}
	existing.Avatar = incoming.Avatar
	return existing, nil
}

func (visitor *Visitor) UpdateUserName(userName string) error {
	return DB.Model(&Visitor{}).
		Where("visitor_id = ?", visitor.VisitorID).
		Update("user_name", userName).Error
}

func (Visitor) GetAllVisitors() ([]Visitor, error) {
	var visitors []Visitor
	err := DB.Order("last_seen desc").Find(&visitors).Error
	return visitors, err
}

func (VisitorSummary) GetAllSummaries() ([]VisitorSummary, error) {
	var summaries []VisitorSummary
	err := DB.Order("date desc").Find(&summaries).Error
	return summaries, err
}

// FixDailyUV recalculates UV for each day in visitor_summary using the visitor table.
// UV = number of visitors whose last_seen falls on that date.
func (VisitorSummary) FixDailyUV() error {
	return DB.Exec(`
		UPDATE visitor_summary vs
		SET uv = (
			SELECT COUNT(*) FROM visitor v
			WHERE DATE(v.last_seen) = vs.date
		),
		updated_at = NOW()
		WHERE vs.uv != (
			SELECT COUNT(*) FROM visitor v
			WHERE DATE(v.last_seen) = vs.date
		)
	`).Error
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
