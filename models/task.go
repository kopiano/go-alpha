package models

import (
	"time"

	"gorm.io/gorm"
)

type Task struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"` // 软删除
	UserID    uint           `gorm:"not null;index" json:"user_id"`
	Title     string         `gorm:"type:varchar(200);not null" json:"title"`
	Priority  string         `gorm:"type:varchar(20);default:medium" json:"priority"`
	Active    *bool          `gorm:"default:true" json:"active"`
}

func (Task) GetTasksByOwner(userID uint) *[]Task {
	var tasks []Task
	DB.Where("user_id = ?", userID).Order("created_at desc").Find(&tasks)
	return &tasks
}

func (Task) GetTaskByIdAndOwner(id, userID uint) *Task {
	var task Task
	DB.Where("id = ? AND user_id = ?", id, userID).First(&task)
	return &task
}

func (task *Task) AddTask() *Task {
	DB.Create(task)
	return task
}

func (task *Task) ToggleActive(active bool) *Task {
	DB.Model(task).Update("active", active)
	task.Active = &active
	return task
}

func (Task) DeleteTask(id uint) {
	DB.Delete(&Task{}, id)
}
