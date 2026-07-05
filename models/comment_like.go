package models

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CommentLike struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CommentID uint           `gorm:"index;not null;uniqueIndex:idx_comment_user" json:"comment_id"`
	UserID    uint           `gorm:"index;not null;uniqueIndex:idx_comment_user" json:"user_id"`
	CreatedAt  time.Time      `json:"created_at"`
}

func (CommentLike) Exists(commentID, userID uint) (bool, error) {
	var count int64
	err := DB.Model(&CommentLike{}).
		Where("comment_id = ? AND user_id = ?", commentID, userID).
		Count(&count).Error
	return count > 0, err
}

func (CommentLike) Add(commentID, userID uint) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "comment_id"}, {Name: "user_id"}},
			DoNothing: true,
		}).Create(&CommentLike{CommentID: commentID, UserID: userID})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		if err := tx.Model(&Comment{}).
			Where("id = ?", commentID).
			Update("like_count", gorm.Expr("like_count + ?", 1)).Error; err != nil {
			return err
		}
		return nil
	})
}

func (CommentLike) Remove(commentID, userID uint) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Where("comment_id = ? AND user_id = ?", commentID, userID).Delete(&CommentLike{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		return tx.Model(&Comment{}).
			Where("id = ? AND like_count > 0", commentID).
			Update("like_count", gorm.Expr("like_count - ?", 1)).Error
	})
}
