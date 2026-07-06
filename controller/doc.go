package controller

import (
	"errors"
	"log/slog"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"go-alpha/models"
	"go-alpha/response"
)

var docCategories = map[string]struct{}{
	"frontend":  {},
	"backend":   {},
	"database":  {},
	"devops":    {},
	"api":       {},
	"resources": {},
}

type DocController struct{}

type docListItem struct {
	ID             uint   `json:"id"`
	UserID         uint   `json:"user_id"`
	Author         string `json:"author"`
	AvatarURL      string `json:"avatar_url"`
	Contributors   []uint `json:"contributors"`
	Title          string `json:"title"`
	Category       string `json:"category"`
	Visibility     int    `json:"visibility"`
	EditPermission int    `json:"edit_permission"`
	Excerpt        string `json:"excerpt"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

type docDetail struct {
	ID             uint   `json:"id"`
	UserID         uint   `json:"user_id"`
	Author         string `json:"author"`
	AvatarURL      string `json:"avatar_url"`
	Contributors   []uint `json:"contributors"`
	Title          string `json:"title"`
	Content        string `json:"content"`
	Category       string `json:"category"`
	Visibility     int    `json:"visibility"`
	EditPermission int    `json:"edit_permission"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

func getDocUserID(ctx *gin.Context) (uint, bool) {
	userID, ok := ctx.Get("userId")
	if !ok {
		return 0, false
	}
	uid, ok := userID.(uint)
	if !ok || uid == 0 {
		return 0, false
	}
	return uid, true
}

func getOptionalDocUserID(ctx *gin.Context) (uint, bool, bool) {
	if userID, ok := getDocUserID(ctx); ok {
		return userID, true, true
	}

	authHeader := strings.TrimSpace(ctx.GetHeader("Authorization"))
	if authHeader == "" {
		return 0, false, true
	}

	tokenStr := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	if tokenStr == "" {
		response.Failed("登录已过期", ctx)
		return 0, false, false
	}

	claims, err := response.ParseToken(tokenStr)
	if err != nil || claims == nil || claims.Id == 0 {
		response.Failed("登录已过期", ctx)
		return 0, false, false
	}
	return claims.Id, true, true
}

func getDocViewerID(ctx *gin.Context) (uint, bool, bool) {
	if userID, ok := getDocUserID(ctx); ok {
		return userID, true, true
	}

	authHeader := strings.TrimSpace(ctx.GetHeader("Authorization"))
	if authHeader == "" {
		return 0, false, true
	}

	tokenStr := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	if tokenStr == "" {
		return 0, false, true
	}

	claims, err := response.ParseToken(tokenStr)
	if err != nil || claims == nil || claims.Id == 0 {
		return 0, false, true
	}
	return claims.Id, true, true
}

func docExcerpt(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return "Documentation file."
	}
	trimmed = strings.ReplaceAll(trimmed, "\n", " ")
	trimmed = strings.ReplaceAll(trimmed, "\t", " ")
	trimmed = strings.Join(strings.Fields(trimmed), " ")
	if len(trimmed) > 140 {
		trimmed = trimmed[:140]
	}
	return trimmed
}

func toDocListItem(md models.Md) docListItem {
	return docListItem{
		ID:             md.ID,
		UserID:         md.UserID,
		Author:         docAuthorName(md.UserID),
		AvatarURL:      docAvatarURL(md.UserID),
		Contributors:   normalizeDocContributors(md.UserID, md.Contributors),
		Title:          md.Title,
		Category:       md.Category,
		Visibility:     md.Visibility,
		EditPermission: md.EditPermission,
		Excerpt:        docExcerpt(md.Content),
		CreatedAt:      md.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:      md.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func toDocDetail(md models.Md) docDetail {
	return docDetail{
		ID:             md.ID,
		UserID:         md.UserID,
		Author:         docAuthorName(md.UserID),
		AvatarURL:      docAvatarURL(md.UserID),
		Contributors:   normalizeDocContributors(md.UserID, md.Contributors),
		Title:          md.Title,
		Content:        md.Content,
		Category:       md.Category,
		Visibility:     md.Visibility,
		EditPermission: md.EditPermission,
		CreatedAt:      md.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:      md.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func docAuthorName(userID uint) string {
	if userID == 0 {
		return "Guest"
	}
	user := models.User{}.GetUserById(int(userID))
	if user == nil || user.ID == 0 || strings.TrimSpace(user.Username) == "" {
		return "Unknown"
	}
	return user.Username
}

func docAvatarURL(userID uint) string {
	if userID == 0 {
		return ""
	}
	user := models.User{}.GetUserById(int(userID))
	if user == nil || user.ID == 0 {
		return ""
	}
	return strings.TrimSpace(user.Avatar)
}

func normalizeDocContributors(authorID uint, contributors models.UserIDList) []uint {
	seen := map[uint]struct{}{}
	out := make([]uint, 0, len(contributors)+1)
	add := func(id uint) {
		if id == 0 {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}

	add(authorID)
	for _, id := range contributors {
		add(id)
	}
	if len(out) == 0 && authorID == 0 {
		return []uint{0}
	}
	return out
}

func validateVisibility(visibility int) bool {
	return visibility == models.Private || visibility == models.Public
}

func validateEditPermission(editPermission int) bool {
	return editPermission == models.OwnerOnly || editPermission == models.EditPublic
}

func validateDocPermissionCombo(visibility, editPermission int) bool {
	if visibility == models.Private {
		return editPermission == models.OwnerOnly
	}
	return editPermission == models.OwnerOnly || editPermission == models.EditPublic
}

func validateCategory(category string) bool {
	_, ok := docCategories[strings.ToLower(strings.TrimSpace(category))]
	return ok
}

func (DocController) List(ctx *gin.Context) {
	userID, ok, allowed := getDocViewerID(ctx)
	if !allowed {
		return
	}

	items, err := (models.Md{}).ListVisible(userID, ok)
	if err != nil {
		slog.Error("Doc.List failed", "error", err, "user_id", userID)
		response.Failed("获取文档列表失败", ctx)
		return
	}

	list := make([]docListItem, 0, len(items))
	for _, item := range items {
		list = append(list, toDocListItem(item))
	}

	response.Success("获取文档列表成功", gin.H{"list": list, "total": len(list)}, ctx)
}

func (DocController) Detail(ctx *gin.Context) {
	userID, ok, allowed := getDocViewerID(ctx)
	if !allowed {
		return
	}

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Failed("文档 ID 不正确", ctx)
		return
	}

	item, err := (models.Md{}).GetVisibleByID(uint(id), userID, ok)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Failed("文档不存在", ctx)
			return
		}
		slog.Error("Doc.Detail failed", "error", err, "id", id, "user_id", userID)
		response.Failed("获取文档失败", ctx)
		return
	}

	response.Success("获取文档成功", toDocDetail(item), ctx)
}

func (DocController) Save(ctx *gin.Context) {
	userID, hasUser, allowed := getOptionalDocUserID(ctx)
	if !allowed {
		return
	}

	var form struct {
		Title          string `json:"title" binding:"required"`
		Content        string `json:"content" binding:"required"`
		Category       string `json:"category"`
		Visibility     *int   `json:"visibility" binding:"required"`
		EditPermission *int   `json:"edit_permission" binding:"required"`
		EditorUserID   *uint  `json:"editor_user_id"`
	}
	if err := ctx.ShouldBindJSON(&form); err != nil {
		response.Failed("参数错误，title、content、visibility、edit_permission 为必填", ctx)
		return
	}

	if form.Visibility == nil || !validateVisibility(*form.Visibility) {
		response.Failed("visibility 取值不合法", ctx)
		return
	}
	if form.EditPermission == nil || !validateEditPermission(*form.EditPermission) {
		response.Failed("edit_permission 取值不合法", ctx)
		return
	}
	if !validateCategory(form.Category) {
		response.Failed("category 取值不合法", ctx)
		return
	}
	if !validateDocPermissionCombo(*form.Visibility, *form.EditPermission) {
		response.Failed("visibility 与 edit_permission 组合不合法", ctx)
		return
	}
	if form.EditorUserID != nil && hasUser && *form.EditorUserID != 0 && *form.EditorUserID != userID {
		response.Failed("editor_user_id 与登录用户不一致", ctx)
		return
	}
	if !hasUser && form.EditorUserID != nil && *form.EditorUserID != 0 {
		response.Failed("游客不能指定 editor_user_id", ctx)
		return
	}

	if !hasUser && *form.Visibility != models.Public {
		response.Failed("游客只能创建公开文档", ctx)
		return
	}
	if !hasUser && *form.EditPermission != models.EditPublic {
		response.Failed("游客只能创建可公开编辑的文档", ctx)
		return
	}

	md := models.Md{
		UserID:         userID,
		Title:          strings.TrimSpace(form.Title),
		Content:        form.Content,
		Category:       strings.TrimSpace(form.Category),
		Visibility:     *form.Visibility,
		EditPermission: *form.EditPermission,
	}
	if !hasUser {
		md.UserID = 0
	}
	if hasUser {
		md.Contributors = models.UserIDList{userID}
	} else {
		md.Contributors = models.UserIDList{0}
	}
	if md.Title == "" {
		response.Failed("标题不能为空", ctx)
		return
	}

	if err := md.Create(); err != nil {
		slog.Error("Doc.Save failed", "error", err, "user_id", userID)
		response.Failed("保存文档失败", ctx)
		return
	}

	response.Success("保存成功", toDocDetail(md), ctx)
}

func (DocController) Update(ctx *gin.Context) {
	userID, hasUser, allowed := getOptionalDocUserID(ctx)
	if !allowed {
		return
	}

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Failed("文档 ID 不正确", ctx)
		return
	}

	var form struct {
		Title          string `json:"title" binding:"required"`
		Content        string `json:"content" binding:"required"`
		Category       string `json:"category"`
		Visibility     *int   `json:"visibility" binding:"required"`
		EditPermission *int   `json:"edit_permission" binding:"required"`
		EditorUserID   *uint  `json:"editor_user_id"`
	}
	if err := ctx.ShouldBindJSON(&form); err != nil {
		response.Failed("参数错误，title、content、visibility、edit_permission 为必填", ctx)
		return
	}

	if form.Visibility == nil || !validateVisibility(*form.Visibility) {
		response.Failed("visibility 取值不合法", ctx)
		return
	}
	if form.EditPermission == nil || !validateEditPermission(*form.EditPermission) {
		response.Failed("edit_permission 取值不合法", ctx)
		return
	}
	if !validateCategory(form.Category) {
		response.Failed("category 取值不合法", ctx)
		return
	}
	if !validateDocPermissionCombo(*form.Visibility, *form.EditPermission) {
		response.Failed("visibility 与 edit_permission 组合不合法", ctx)
		return
	}
	if form.EditorUserID != nil && hasUser && *form.EditorUserID != 0 && *form.EditorUserID != userID {
		response.Failed("editor_user_id 与登录用户不一致", ctx)
		return
	}
	if !hasUser && form.EditorUserID != nil && *form.EditorUserID != 0 {
		response.Failed("游客不能指定 editor_user_id", ctx)
		return
	}

	md, err := (models.Md{}).GetEditableByID(uint(id), userID, hasUser)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Failed("文档不存在或无权限", ctx)
			return
		}
		slog.Error("Doc.Update load failed", "error", err, "id", id, "user_id", userID)
		response.Failed("获取文档失败", ctx)
		return
	}

	md.Title = strings.TrimSpace(form.Title)
	md.Content = form.Content
	md.Category = strings.TrimSpace(form.Category)
	md.Visibility = *form.Visibility
	md.EditPermission = *form.EditPermission
	md.Contributors = mergeContributors(md.UserID, md.Contributors, userID, hasUser)
	if md.Title == "" {
		response.Failed("标题不能为空", ctx)
		return
	}
	if len(md.Contributors) == 0 {
		md.Contributors = models.UserIDList{md.UserID}
	}

	if err := md.Update(); err != nil {
		slog.Error("Doc.Update failed", "error", err, "id", id, "user_id", userID)
		response.Failed("更新文档失败", ctx)
		return
	}

	response.Success("更新成功", toDocDetail(md), ctx)
}

func mergeContributors(authorID uint, contributors models.UserIDList, userID uint, hasUser bool) models.UserIDList {
	seen := map[uint]struct{}{}
	out := make([]uint, 0, len(contributors)+2)
	add := func(id uint) {
		if id == 0 {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}

	add(authorID)
	for _, id := range contributors {
		add(id)
	}
	if hasUser {
		add(userID)
	}
	return models.UserIDList(out)
}

func (DocController) Delete(ctx *gin.Context) {
	userID, ok, allowed := getOptionalDocUserID(ctx)
	if !allowed {
		return
	}
	if !ok {
		response.Failed("未登录", ctx)
		return
	}

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Failed("文档 ID 不正确", ctx)
		return
	}

	md, err := (models.Md{}).GetOwnedByID(uint(id), userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Failed("仅文档作者可以删除该文档", ctx)
			return
		}
		slog.Error("Doc.Delete ownership check failed", "error", err, "id", id, "user_id", userID)
		response.Failed("删除文档失败", ctx)
		return
	}

	rowsAffected, err := (models.Md{}).DeleteByID(md.ID, userID)
	if err != nil {
		slog.Error("Doc.Delete failed", "error", err, "id", id, "user_id", userID)
		response.Failed("删除文档失败", ctx)
		return
	}
	if rowsAffected == 0 {
		response.Failed("文档不存在或无权限", ctx)
		return
	}

	response.Success("删除成功", gin.H{"id": uint(id)}, ctx)
}
