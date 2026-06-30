package controller

import (
	"errors"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"go-alpha/models"
	"go-alpha/response"
	"gorm.io/gorm"
)

type CommentController struct{}

type commentForm struct {
	ParentID    *uint  `json:"parent_id"`
	Content     string `json:"content" binding:"required"`
	Username    string `json:"username" binding:"required"`
	Email       string `json:"email"`
	Website     string `json:"website"`
	CommentTime string `json:"comment_time"`
}

func (CommentController) AddComment(c *gin.Context) {
	var form commentForm
	if err := c.ShouldBindJSON(&form); err != nil {
		response.Failed("参数错误，username、content 为必填", c)
		return
	}

	username := strings.TrimSpace(form.Username)
	content := strings.TrimSpace(form.Content)
	if username == "" || content == "" {
		response.Failed("参数错误，username、content 不能为空", c)
		return
	}

	if form.ParentID != nil {
		exists, err := models.Comment{}.Exists(*form.ParentID)
		if err != nil {
			log.Printf("[error] Comment.Exists: %s", err)
			response.Failed("评论失败", c)
			return
		}
		if !exists {
			response.Failed("父评论不存在", c)
			return
		}
	}

	commentTime := time.Now()
	if strings.TrimSpace(form.CommentTime) != "" {
		parsedTime, err := time.Parse(time.RFC3339, strings.TrimSpace(form.CommentTime))
		if err != nil {
			response.Failed("comment_time 格式错误，请使用 RFC3339 格式", c)
			return
		}
		commentTime = parsedTime
	}

	comment := models.Comment{
		ParentID:    form.ParentID,
		Content:     content,
		Username:    username,
		Email:       strings.TrimSpace(form.Email),
		Website:     strings.TrimSpace(form.Website),
		CommentTime: commentTime,
		LikeCount:   0,
	}
	if err := comment.AddComment(); err != nil {
		log.Printf("[error] Comment.AddComment: %s", err)
		response.Failed("评论失败", c)
		return
	}

	response.Success("评论成功", comment, c)
}

func (CommentController) ListComments(c *gin.Context) {
	comments, err := models.Comment{}.GetCommentsWithReplies()
	if err != nil {
		log.Printf("[error] Comment.ListComments: %s", err)
		response.Failed("获取评论失败", c)
		return
	}
	response.Success("获取评论成功", comments, c)
}

type reactionForm struct {
	Action string `json:"action" binding:"required"`
}

func (CommentController) ReactionComment(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Failed("评论 ID 不正确", c)
		return
	}

	var form reactionForm
	if err := c.ShouldBindJSON(&form); err != nil {
		response.Failed("参数错误，action 为必填", c)
		return
	}

	action := strings.TrimSpace(form.Action)
	switch action {
	case "likes":
		action = "like"
	case "unlikes":
		action = "unlike"
	}

	comment, err := models.Comment{}.React(uint(id), action)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Failed("评论不存在", c)
			return
		}
		if errors.Is(err, gorm.ErrInvalidData) {
			response.Failed("action 仅支持 like 或 unlike", c)
			return
		}
		log.Printf("[error] Comment.Like: %s", err)
		response.Failed("操作失败", c)
		return
	}

	message := "点赞成功"
	if action == "unlike" {
		message = "取消点赞成功"
	}

	response.Success(message, gin.H{
		"id":         comment.ID,
		"likes":      comment.LikeCount,
		"like_count": comment.LikeCount,
	}, c)
}

func (CommentController) ReactionCommentLegacy(c *gin.Context) {
	CommentController{}.ReactionComment(c)
}
