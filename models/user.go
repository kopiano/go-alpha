package models

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Username    string     `gorm:"type:varchar(100);not null" json:"username"`
	Email       string     `gorm:"type:varchar(100);uniqueIndex;not null" json:"email"`
	Password    string     `gorm:"type:varchar(100);not null" json:"-"` // 无需显示
	Website     string     `gorm:"type:varchar(100)" json:"website"`
	Status      string     `gorm:"type:varchar(100);default:active" json:"status"`
	Content     string     `gorm:"type:text" json:"content"`                        // comment
	Avatar      string     `gorm:"type:varchar(255);default:null" json:"avatar"`    // 允许默认头像
	LastLoginAt *time.Time `gorm:"type:datetime;default:null" json:"last_login_at"` // 最后登录日期，注册日期为Create_at
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
