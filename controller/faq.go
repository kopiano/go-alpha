package controller

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"go-alpha/models"
	"go-alpha/response"
)

type FaqController struct{}

// ListFAQ handles GET /api/v1/faq — reads from MySQL + syncs to JSON file
func (f *FaqController) ListFAQ(ctx *gin.Context) {
	faqs, err := models.Faq{}.GetAll()
	if err != nil {
		slog.Error("Faq.List: MySQL query failed", "error", err)
		response.Failed("读取FAQ失败", ctx)
		return
	}

	if faqs == nil {
		faqs = []models.Faq{}
	}

	// 同步到 JSON 文件
	if err := models.SyncFaqToFile(); err != nil {
		slog.Error("Faq.List: JSON sync failed", "error", err)
	}

	response.Success("ok", faqs, ctx)
}

// AddFAQ handles POST /api/v1/faq — writes to MySQL + syncs to assets/faq/faq.json
func (f *FaqController) AddFAQ(ctx *gin.Context) {
	var form struct {
		Title      string `json:"title" binding:"required"`
		Answer     string `json:"answer" binding:"required"`
		Difficulty string `json:"difficulty"`
		Category   string `json:"category"`
	}
	if err := ctx.ShouldBindJSON(&form); err != nil {
		response.Failed("参数错误，title、answer 为必填", ctx)
		return
	}

	// Validate difficulty
	switch form.Difficulty {
	case "":
		form.Difficulty = "easy"
	case "easy", "medium", "hard":
		// valid
	default:
		response.Failed("difficulty 只能为 easy、medium 或 hard", ctx)
		return
	}

	faq := models.Faq{
		Title:      form.Title,
		Answer:     form.Answer,
		Difficulty: form.Difficulty,
		Category:   form.Category,
	}

	// 1. Write to MySQL
	if err := faq.Add(); err != nil {
		slog.Error("Faq.Add: MySQL insert failed", "error", err)
		response.Failed("添加FAQ失败", ctx)
		return
	}

	// 2. Sync to assets/faq/faq.json
	if err := models.SyncFaqToFile(); err != nil {
		slog.Error("Faq.Add: JSON sync failed", "error", err)
		// Non-fatal — MySQL write already succeeded
	}

	slog.Info("Faq.Add: success", "id", faq.ID, "title", faq.Title)
	response.Success("添加FAQ成功", faq, ctx)
}
