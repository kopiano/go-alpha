package controller

import (
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"go-alpha/models"
	"go-alpha/response"
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
			slog.Error("Comment.Exists failed", "error", err)
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
		slog.Error("Comment.AddComment failed", "error", err)
		response.Failed("评论失败", c)
		return
	}

	response.Success("评论成功", comment, c)
}

func (CommentController) ListComments(c *gin.Context) {
	comments, err := models.Comment{}.GetCommentsWithReplies()
	if err != nil {
		slog.Error("Comment.ListComments failed", "error", err)
		response.Failed("获取评论失败", c)
		return
	}
	response.Success("获取评论成功", comments, c)
}

type reactionForm struct {
	Action string `json:"action" binding:"required"`
}

func respondCommentReaction(c *gin.Context, comment *models.Comment, action string) {
	message := "点赞成功"
	if action == "unlikes" || action == "delete" {
		message = "取消点赞成功"
	}

	response.Success(message, gin.H{
		"id":         comment.ID,
		"likes":      comment.LikeCount,
		"like_count": comment.LikeCount,
	}, c)
}

func getAuthUserID(c *gin.Context) (uint, bool) {
	userID, ok := c.Get("userId")
	if !ok {
		response.Failed("未登录", c)
		return 0, false
	}
	id, ok := userID.(uint)
	if !ok || id == 0 {
		response.Failed("未登录", c)
		return 0, false
	}
	return id, true
}

func handleCommentReaction(c *gin.Context, action string) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Failed("评论 ID 不正确", c)
		return
	}

	userID, ok := getAuthUserID(c)
	if !ok {
		return
	}

	exists, err := models.Comment{}.Exists(uint(id))
	if err != nil {
		slog.Error("Comment.Exists failed", "error", err)
		response.Failed("操作失败", c)
		return
	}
	if !exists {
		response.Failed("评论不存在", c)
		return
	}

	switch action {
	case "likes":
		likeModel := models.CommentLike{}
		if err := likeModel.Add(uint(id), userID); err != nil {
			slog.Error("CommentLike.Add failed", "error", err)
			response.Failed("点赞失败", c)
			return
		}
	case "unlikes":
		likeModel := models.CommentLike{}
		if err := likeModel.Remove(uint(id), userID); err != nil {
			slog.Error("CommentLike.Remove failed", "error", err)
			response.Failed("取消点赞失败", c)
			return
		}
	default:
		response.Failed("action 仅支持 likes 或 unlikes", c)
		return
	}

	var comment models.Comment
	if err := models.DB.First(&comment, uint(id)).Error; err != nil {
		slog.Error("Comment fetch failed", "error", err)
		response.Failed("操作失败", c)
		return
	}
	respondCommentReaction(c, &comment, action)
}

func (CommentController) LikesComment(c *gin.Context) {
	var form reactionForm
	if err := c.ShouldBindJSON(&form); err != nil {
		response.Failed("参数错误，action 为必填", c)
		return
	}

	action := strings.TrimSpace(form.Action)
	handleCommentReaction(c, action)
}

func (CommentController) LikeComment(c *gin.Context) {
	handleCommentReaction(c, "likes")
}

func (CommentController) UnlikeComment(c *gin.Context) {
	handleCommentReaction(c, "unlikes")
}
