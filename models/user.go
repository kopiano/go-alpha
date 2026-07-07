package models

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	Username    string         `gorm:"type:varchar(100);not null" json:"username"`
	Email       string         `gorm:"type:varchar(100)" json:"email"`
	Password    string         `gorm:"type:varchar(100);not null" json:"-"`
	Website     string         `gorm:"type:varchar(100)" json:"website"`
	Status      string         `gorm:"type:varchar(100);default:inactive" json:"status"`
	Avatar      string         `gorm:"type:varchar(255);default:null" json:"avatar"`
	LastLoginAt *time.Time     `gorm:"type:datetime;default:null;index" json:"last_login_at"`
}

func (User) GetAllUsers() *[]User {
	var users []User
	DB.Find(&users)
	return &users
}

func (User) GetUserById(id int) *User {
	var user User
	DB.First(&user, id)
	return &user
}

func (User) GetUserByName(name string) *User {
	var user User
	DB.Where("username = ?", name).First(&user)
	return &user
}

func (user *User) AddUser() *User {
	if user.Status == "" {
		user.Status = "inactive"
	}
	DB.Create(user)
	return user
}

func (user *User) UpdateUser() *User {
	DB.Save(user)
	return user
}

func (User) DeleteUser(id uint) {
	DB.Delete(&User{}, id)
}
