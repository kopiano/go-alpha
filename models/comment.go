package models

import (
	"time"

	"gorm.io/gorm"
)

type Comment struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	ParentID    *uint     `gorm:"index" json:"parent_id"`
	Content     string    `gorm:"type:text;not null" json:"content"`
	Username    string    `gorm:"type:varchar(100);not null;index" json:"username"`
	Email       string    `gorm:"type:varchar(255)" json:"email"`
	Website     string    `gorm:"type:varchar(255)" json:"website"`
	CommentTime time.Time `gorm:"type:datetime;not null" json:"comment_time"`
	LikeCount   int64     `gorm:"->;column:like_count;default:0;not null" json:"like_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CommentWithReplies struct {
	Comment
	Replies []Comment `json:"replies"`
}

func (comment *Comment) AddComment() error {
	return DB.Create(comment).Error
}

func (Comment) GetAllComments() ([]Comment, error) {
	var comments []Comment
	err := DB.Order("comment_time desc").Find(&comments).Error
	return comments, err
}

func (Comment) GetCommentsWithReplies() ([]CommentWithReplies, error) {
	comments, err := Comment{}.GetAllComments()
	if err != nil {
		return nil, err
	}

	rootIndex := make(map[uint]int)
	roots := make([]CommentWithReplies, 0)

	for _, comment := range comments {
		if comment.ParentID != nil {
			continue
		}
		roots = append(roots, CommentWithReplies{
			Comment: comment,
			Replies: []Comment{},
		})
		rootIndex[comment.ID] = len(roots) - 1
	}

	for _, comment := range comments {
		if comment.ParentID == nil {
			continue
		}
		index, ok := rootIndex[*comment.ParentID]
		if ok {
			roots[index].Replies = append(roots[index].Replies, comment)
		}
	}

	return roots, nil
}

func (Comment) Exists(id uint) (bool, error) {
	var count int64
	err := DB.Model(&Comment{}).Where("id = ?", id).Count(&count).Error
	return count > 0, err
}

func (Comment) Like(id uint) (*Comment, error) {
	if err := DB.Model(&Comment{}).
		Where("id = ?", id).
		Update("like_count", gorm.Expr("like_count + ?", 1)).Error; err != nil {
		return nil, err
	}

	var comment Comment
	if err := DB.First(&comment, id).Error; err != nil {
		return nil, err
	}
	return &comment, nil
}

func (Comment) Unlike(id uint) (*Comment, error) {
	if err := DB.Model(&Comment{}).
		Where("id = ? AND like_count > 0", id).
		Update("like_count", gorm.Expr("like_count - ?", 1)).Error; err != nil {
		return nil, err
	}

	var comment Comment
	if err := DB.First(&comment, id).Error; err != nil {
		return nil, err
	}
	return &comment, nil
}

func (Comment) React(id uint, action string) (*Comment, error) {
	switch action {
	case "like":
		return Comment{}.Like(id)
	case "unlike":
		return Comment{}.Unlike(id)
	default:
		return nil, gorm.ErrInvalidData
	}
}
