package models

import (
	"crypto/rand"
	"errors"
	"log/slog"
	"math/big"
	"time"

	"gorm.io/gorm"
)

type VisitorSummary struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	Date       string    `gorm:"type:date;index;not null" json:"date"`
	VisitorID  string    `gorm:"type:varchar(32);index;not null" json:"visitor_id"`
	VisitCount int64     `gorm:"default:0" json:"visit_count"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (VisitorSummary) TableName() string {
	return "visitor_summary"
}

func (VisitorSummary) UpsertDaily(date string, visitorID string) {
	err := DB.Where("date = ? AND visitor_id = ?", date, visitorID).
		Assign(map[string]any{"visit_count": gorm.Expr("visit_count + 1")}).
		FirstOrCreate(&VisitorSummary{
			Date:      date,
			VisitorID: visitorID,
			VisitCount: 1,
		})
	if err.Error != nil {
		slog.Error("VisitorSummary.UpsertDaily failed", "date", date, "visitor_id", visitorID, "error", err.Error)
	}
}

type Visitor struct {
	ID              uint      `gorm:"primarykey" json:"id"`
	VisitorID       string    `gorm:"type:varchar(32);uniqueIndex;not null" json:"visitor_id"`
	UserName        string    `gorm:"type:varchar(100)" json:"user_name"`
	Avatar          string    `gorm:"type:varchar(255)" json:"avatar"`
	IP              string    `gorm:"type:varchar(64)" json:"ip"`
	Country         string    `gorm:"type:varchar(100)" json:"country"`
	City            string    `gorm:"type:varchar(100)" json:"city"`
	Location        string    `gorm:"type:varchar(255)" json:"location"`
	FirstSeen       time.Time `gorm:"type:datetime;not null" json:"first_seen"`
	LastSeen        time.Time `gorm:"type:datetime;not null" json:"last_seen"`
	TotalBrowseTime int64     `gorm:"default:0;not null" json:"total_browse_time"`
	VisitCount      int64     `gorm:"default:0;not null" json:"visit_count"`
	OS              string    `gorm:"type:varchar(50)" json:"os"`
	Browser         string    `gorm:"type:varchar(50)" json:"browser"`
	Device          string    `gorm:"type:varchar(50)" json:"device"`
	Status          string    `gorm:"type:varchar(50);default:inactive" json:"status"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func generateShortID() string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[n.Int64()]
	}
	return string(b)
}

func (visitor *Visitor) UpsertVisit(duration int64) (*Visitor, error) {
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

	// 2) Not found by visitor_id — if username + ip are both present,
	//    check whether the same user+ip already exists under a different
	//    visitor_id (e.g. localStorage cleared, device switch, bucket change).
	if visitor.UserName != "" && visitor.IP != "" {
		err = DB.Where("user_name = ? AND ip = ?", visitor.UserName, visitor.IP).First(&existing).Error
		if err == nil {
			// Merge into the existing record, updating visitor_id.
			existing.VisitorID = visitor.VisitorID
			return applyVisitorUpdates(&existing, visitor, duration, now)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}

	// 3) Truly new visitor — insert.
	visitor.VisitorID = generateShortID()
	visitor.FirstSeen = now
	visitor.LastSeen = now
	visitor.TotalBrowseTime = duration
	visitor.VisitCount = 1
	return visitor, DB.Create(visitor).Error
}

// applyVisitorUpdates persists the merged fields and returns the updated record.
func applyVisitorUpdates(existing *Visitor, incoming *Visitor, duration int64, now time.Time) (*Visitor, error) {
	updates := map[string]any{
		"visitor_id":        existing.VisitorID,
		"ip":                incoming.IP,
		"country":           incoming.Country,
		"city":              incoming.City,
		"location":          incoming.Location,
		"last_seen":         now,
		"total_browse_time": existing.TotalBrowseTime + duration,
		"visit_count":       gorm.Expr("visit_count + 1"),
		"os":                incoming.OS,
		"browser":           incoming.Browser,
		"device":            incoming.Device,
		"status":            incoming.Status,
		"avatar":            incoming.Avatar,
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
