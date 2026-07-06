package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
)

const (
	Private    = 0 // 用户可见
	Public     = 1 // 所有人可见(包括游客)
	OwnerOnly  = 0 // 仅作者可编辑
	EditPublic = 1 // 任何人可编辑
)

type Md struct {
	ID             uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID         uint           `gorm:"index;not null" json:"user_id"`
	Title          string         `gorm:"type:varchar(20);not null" json:"title"`
	Content        string         `gorm:"type:longtext" json:"content"`
	Category       string         `gorm:"type:varchar(20)" json:"category"`
	Visibility     int            `gorm:"type:tinyint(1);not null" json:"visibility"`      // 0=private, 1=public
	EditPermission int            `gorm:"type:tinyint(1);not null" json:"edit_permission"` // 0=owner_only, 1=public
	Contributors   UserIDList     `gorm:"type:json" json:"contributors"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

type UserIDList []uint

func (l *UserIDList) Scan(value any) error {
	if value == nil {
		*l = nil
		return nil
	}
	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return fmt.Errorf("cannot scan %T into UserIDList", value)
	}
	if len(data) == 0 {
		*l = nil
		return nil
	}
	var ids []uint
	if err := json.Unmarshal(data, &ids); err != nil {
		return err
	}
	*l = UserIDList(ids)
	return nil
}

func (l UserIDList) Value() (driver.Value, error) {
	if l == nil {
		return []byte("[]"), nil
	}
	return json.Marshal([]uint(l))
}

func (Md) TableName() string {
	return "md"
}

func ErrRecordNotFound() error {
	return gorm.ErrRecordNotFound
}

func (Md) ListPublic() ([]Md, error) {
	var items []Md
	err := DB.Where("visibility = ?", Public).
		Order("updated_at DESC, id DESC").
		Find(&items).Error
	return items, err
}

func (Md) ListByUser(userID uint) ([]Md, error) {
	var items []Md
	err := DB.Where("user_id = ? AND visibility = ?", userID, Private).
		Order("updated_at DESC, id DESC").
		Find(&items).Error
	return items, err
}

func (Md) ListVisible(userID uint, hasUser bool) ([]Md, error) {
	var items []Md
	query := DB.Model(&Md{})
	if hasUser {
		query = query.Where("user_id = ? OR visibility = ?", userID, Public)
	} else {
		query = query.Where("visibility = ?", Public)
	}
	err := query.Order("updated_at DESC, id DESC").Find(&items).Error
	return items, err
}

func (Md) GetPublicByID(id uint) (Md, error) {
	var item Md
	err := DB.Where("id = ? AND visibility = ?", id, Public).First(&item).Error
	return item, err
}

func (Md) GetVisibleByID(id, userID uint, hasUser bool) (Md, error) {
	var item Md
	query := DB.Where("id = ? AND visibility = ?", id, Public)
	if hasUser {
		query = DB.Where("id = ? AND (user_id = ? OR visibility = ?)", id, userID, Public)
	}
	err := query.First(&item).Error
	return item, err
}

func (Md) GetOwnedByID(id, userID uint) (Md, error) {
	var item Md
	err := DB.Where("id = ? AND user_id = ?", id, userID).First(&item).Error
	return item, err
}

func (Md) GetEditableByID(id, userID uint, hasUser bool) (Md, error) {
	var item Md
	query := DB.Where("id = ? AND visibility = ? AND edit_permission = ?", id, Public, EditPublic)
	if hasUser {
		query = DB.Where("id = ? AND (user_id = ? OR (visibility = ? AND edit_permission = ?))", id, userID, Public, EditPublic)
	}
	err := query.First(&item).Error
	return item, err
}

func (m *Md) Create() error {
	if len(m.Contributors) == 0 {
		m.Contributors = UserIDList{m.UserID}
	}
	return DB.Create(m).Error
}

func (m *Md) Update() error {
	return DB.Save(m).Error
}

func (Md) DeleteByID(id, userID uint) (int64, error) {
	result := DB.Where("id = ? AND user_id = ?", id, userID).Delete(&Md{})
	return result.RowsAffected, result.Error
}
