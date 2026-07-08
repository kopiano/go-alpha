package controller

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go-alpha/models"
	"go-alpha/response"
	"golang.org/x/crypto/bcrypt"
)

type AuthController interface {
	Login(ctx *gin.Context)
	Register(ctx *gin.Context)
	Me(ctx *gin.Context)
	Logout(ctx *gin.Context)
	SettingUser(ctx *gin.Context)
}

type authController struct{}

func NewAuthController() *authController {
	return &authController{}
}

const maxAvatarSize = 10 * 1024 * 1024
const authBcryptCost = 10

func avatarDir() string {
	if v := strings.TrimSpace(os.Getenv("AVATAR_DIR")); v != "" {
		return v
	}
	return filepath.Join(".", "assets", "avatar")
}

func AvatarDir() string {
	return avatarDir()
}

func runAsync(taskName string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("async task panicked", "task", taskName, "panic", r)
			}
		}()
		fn()
	}()
}

func validateAvatarSize(file *multipart.FileHeader) bool {
	return file != nil && file.Size <= maxAvatarSize
}

func (c *authController) Me(ctx *gin.Context) {
	userId, ok := ctx.Get("userId")
	if !ok {
		response.Failed("未登录", ctx)
		return
	}
	id, ok := userId.(uint)
	if !ok || id == 0 {
		response.Failed("登录信息无效", ctx)
		return
	}
	user := models.User{}.GetUserById(int(id))
	if user.ID == 0 {
		response.Failed("用户不存在", ctx)
		return
	}

	response.Success("ok", gin.H{
		"id":            user.ID,
		"username":      user.Username,
		"email":         user.Email,
		"avatar":        user.Avatar,
		"status":        user.Status,
		"last_login_at": user.LastLoginAt,
	}, ctx)
}

func (c *authController) Login(ctx *gin.Context) {
	start := time.Now()
	var form struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := ctx.ShouldBindJSON(&form); err != nil {
		response.Failed("参数错误", ctx)
		return
	}
	form.Username = strings.TrimSpace(form.Username)

	queryStart := time.Now()
	user := models.User{}.GetUserByName(form.Username)
	slog.Info("auth.login timing", "step", "get_user_by_name", "cost_ms", time.Since(queryStart).Milliseconds(), "username", form.Username)
	if user.ID == 0 {
		response.Failed("用户名或密码错误", ctx)
		return
	}

	bcryptStart := time.Now()
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(form.Password)); err != nil {
		slog.Info("auth.login timing", "step", "bcrypt_compare", "cost_ms", time.Since(bcryptStart).Milliseconds(), "username", form.Username)
		response.Failed("用户名或密码错误", ctx)
		return
	}
	slog.Info("auth.login timing", "step", "bcrypt_compare", "cost_ms", time.Since(bcryptStart).Milliseconds(), "username", form.Username)

	tokenStart := time.Now()
	token, err := response.GenerateToken(user.ID)
	if err != nil {
		slog.Error("Login: GenerateToken failed", "error", err)
		response.Failed("登录失败", ctx)
		return
	}
	slog.Info("auth.login timing", "step", "generate_token", "cost_ms", time.Since(tokenStart).Milliseconds(), "user_id", user.ID)

	// update status to active on login
	updateStart := time.Now()
	if err := models.DB.Model(user).Updates(map[string]any{
		"status":        "active",
		"last_login_at": time.Now(),
	}).Error; err != nil {
		slog.Error("Login: update user status failed", "error", err)
		response.Failed("登录失败", ctx)
		return
	}
	slog.Info("auth.login timing", "step", "db_update", "cost_ms", time.Since(updateStart).Milliseconds(), "user_id", user.ID)

	// 清除聊天联系人缓存，不阻塞登录响应
	runAsync("invalidate_chat_user_info_cache_login", func() {
		invalidateChatUserInfoCache(user.ID)
	})

	response.Success("登录成功", gin.H{
		"token": token,
		"user": gin.H{
			"id":            user.ID,
			"username":      user.Username,
			"email":         user.Email,
			"avatar":        user.Avatar,
			"status":        "active",
			"last_login_at": time.Now(),
		},
	}, ctx)
	slog.Info("auth.login timing", "step", "total", "cost_ms", time.Since(start).Milliseconds(), "user_id", user.ID)
}

func (c *authController) Logout(ctx *gin.Context) {
	userId, ok := ctx.Get("userId")
	if !ok {
		response.Failed("未登录", ctx)
		return
	}
	id, ok := userId.(uint)
	if !ok || id == 0 {
		response.Failed("登录信息无效", ctx)
		return
	}

	result := models.DB.Model(&models.User{}).Where("id = ?", id).Updates(map[string]any{
		"status":        "inactive",
		"last_login_at": time.Now().Add(-10 * time.Minute), // 退出后标记为 10 分钟前，立刻离线
	})
	if result.Error != nil {
		slog.Error("Logout update failed", "error", result.Error, "id", id)
		response.Failed("退出失败", ctx)
		return
	}
	slog.Info("Logout update", "id", id, "rows_affected", result.RowsAffected)

	// 清除聊天联系人缓存，不阻塞退出响应
	runAsync("invalidate_chat_user_info_cache_logout", func() {
		invalidateChatUserInfoCache(uint(id))
	})

	response.Success("退出成功", nil, ctx)
}

func (c *authController) SettingUser(ctx *gin.Context) {
	start := time.Now()
	userId, ok := ctx.Get("userId")
	if !ok {
		response.Failed("未登录", ctx)
		return
	}
	id, ok := userId.(uint)
	if !ok || id == 0 {
		response.Failed("登录信息无效", ctx)
		return
	}
	user := models.User{}.GetUserById(int(id))
	if user.ID == 0 {
		response.Failed("用户不存在", ctx)
		return
	}

	// Support both JSON body and multipart/form-data
	var username, email, password string
	contentType := ctx.ContentType()

	if strings.HasPrefix(contentType, "application/json") {
		var form struct {
			Username string `json:"username"`
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := ctx.ShouldBindJSON(&form); err == nil {
			username = strings.TrimSpace(form.Username)
			email = strings.TrimSpace(form.Email)
			password = strings.TrimSpace(form.Password)
		}
	} else {
		username = strings.TrimSpace(ctx.PostForm("username"))
		email = strings.TrimSpace(ctx.PostForm("email"))
		password = strings.TrimSpace(ctx.PostForm("password"))
	}

	updates := map[string]any{}
	queryStart := time.Now()
	if username != "" && username != user.Username {
		existing := models.User{}.GetUserByName(username)
		slog.Info("auth.setting_user timing", "step", "check_username", "cost_ms", time.Since(queryStart).Milliseconds(), "user_id", user.ID)
		if existing.ID != 0 {
			response.Failed("用户名已存在", ctx)
			return
		}
		updates["username"] = username
	}
	if email != "" {
		if !strings.Contains(email, "@") {
			response.Failed("邮箱格式不正确", ctx)
			return
		}
		updates["email"] = email
	}
	if password != "" {
		bcryptStart := time.Now()
		hashed, err := bcrypt.GenerateFromPassword([]byte(password), authBcryptCost)
		if err != nil {
			response.Failed("密码加密失败", ctx)
			return
		}
		slog.Info("auth.setting_user timing", "step", "bcrypt_hash", "cost_ms", time.Since(bcryptStart).Milliseconds(), "user_id", user.ID)
		updates["password"] = string(hashed)
	}

	// Handle avatar upload (form-data only)
	file, err := ctx.FormFile("avatar")
	if err == nil {
		if !validateAvatarSize(file) {
			response.Failed("头像文件过大，最大支持 5MB", ctx)
			return
		}
		avatarStart := time.Now()
		ext := filepath.Ext(file.Filename)
		if ext == "" {
			ext = ".jpg"
		}
		filename := fmt.Sprintf("avatar-%d%s", user.ID, ext)
		avatarURL := "/api/v1/avatar/" + filename
		updates["avatar"] = avatarURL

		runAsync("setting_user_save_avatar", func() {
			dir := avatarDir()
			if err := os.MkdirAll(dir, 0755); err != nil {
				slog.Warn("SettingUser: MkdirAll failed", "error", err, "user_id", user.ID)
				return
			}
			savePath := filepath.Join(dir, filename)
			src, err := file.Open()
			if err != nil {
				slog.Warn("SettingUser: Open avatar file failed", "error", err, "user_id", user.ID)
				return
			}
			defer src.Close()

			dst, err := os.Create(savePath)
			if err != nil {
				slog.Warn("SettingUser: Create avatar file failed", "error", err, "user_id", user.ID)
				return
			}
			defer dst.Close()

			if _, err := io.Copy(dst, src); err != nil {
				slog.Warn("SettingUser: Copy avatar file failed", "error", err, "user_id", user.ID)
				return
			}
			slog.Info("auth.setting_user timing", "step", "save_avatar_async_done", "cost_ms", time.Since(avatarStart).Milliseconds(), "user_id", user.ID)
		})
		slog.Info("auth.setting_user timing", "step", "queue_avatar_async", "cost_ms", time.Since(avatarStart).Milliseconds(), "user_id", user.ID)
	}

	if len(updates) == 0 {
		response.Success("没有需要修改的字段", gin.H{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
			"avatar":   user.Avatar,
		}, ctx)
		return
	}

	if err := models.DB.Model(&models.User{}).Where("id = ?", user.ID).Updates(updates).Error; err != nil {
		slog.Error("SettingUser: Update failed", "error", err)
		response.Failed("更新失败", ctx)
		return
	}
	slog.Info("auth.setting_user timing", "step", "db_update", "cost_ms", time.Since(start).Milliseconds(), "user_id", user.ID, "fields", len(updates))

	// 清除聊天联系人缓存，不阻塞设置响应
	runAsync("invalidate_chat_user_info_cache_setting_user", func() {
		invalidateChatUserInfoCache(user.ID)
	})

	// Return updated user
	updated := models.User{}.GetUserById(int(user.ID))
	response.Success("设置已保存", gin.H{
		"id":       updated.ID,
		"username": updated.Username,
		"email":    updated.Email,
		"avatar":   updated.Avatar,
	}, ctx)
	slog.Info("auth.setting_user timing", "step", "total", "cost_ms", time.Since(start).Milliseconds(), "user_id", user.ID)
}

func (c *authController) Register(ctx *gin.Context) {
	start := time.Now()
	// Support both JSON body and multipart/form-data (for avatar upload)
	contentType := ctx.ContentType()

	var username, password, email string

	if strings.HasPrefix(contentType, "application/json") {
		var form struct {
			Username string `json:"username"`
			Password string `json:"password"`
			Email    string `json:"email"`
		}
		if err := ctx.ShouldBindJSON(&form); err != nil {
			response.Failed("参数错误", ctx)
			return
		}
		username = form.Username
		password = form.Password
		email = form.Email
	} else {
		username = ctx.PostForm("username")
		password = ctx.PostForm("password")
		email = ctx.PostForm("email")
	}

	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	email = strings.TrimSpace(email)
	if username == "" || password == "" {
		response.Failed("用户名和密码不能为空", ctx)
		return
	}
	if email != "" && !strings.Contains(email, "@") {
		response.Failed("邮箱格式不正确", ctx)
		return
	}

	// check duplicate username
	queryStart := time.Now()
	existing := models.User{}.GetUserByName(username)
	slog.Info("auth.register timing", "step", "check_username", "cost_ms", time.Since(queryStart).Milliseconds(), "username", username)
	if existing.ID != 0 {
		response.Failed("用户名已存在", ctx)
		return
	}

	// hash password
	bcryptStart := time.Now()
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), authBcryptCost)
	if err != nil {
		slog.Error("Register: bcrypt failed", "error", err)
		response.Failed("注册失败", ctx)
		return
	}
	slog.Info("auth.register timing", "step", "bcrypt_hash", "cost_ms", time.Since(bcryptStart).Milliseconds(), "username", username)

	// create user
	createStart := time.Now()
	newUser := models.User{
		Username: username,
		Email:    email,
		Password: string(hashed),
		Status:   "inactive",
	}
	if err := models.DB.Create(&newUser).Error; err != nil {
		slog.Error("Register: Create failed", "error", err)
		response.Failed("注册失败", ctx)
		return
	}
	slog.Info("auth.register timing", "step", "db_create", "cost_ms", time.Since(createStart).Milliseconds(), "user_id", newUser.ID)

	// handle optional avatar upload
	file, err := ctx.FormFile("avatar")
	if err == nil {
		if !validateAvatarSize(file) {
			response.Failed("头像文件过大，最大支持 5MB", ctx)
			return
		}
		avatarStart := time.Now()
		ext := filepath.Ext(file.Filename)
		if ext == "" {
			ext = ".jpg"
		}
		filename := fmt.Sprintf("avatar-%d%s", newUser.ID, ext)
		avatarURL := "/api/v1/avatar/" + filename
		newUser.Avatar = avatarURL

		runAsync("register_save_avatar", func() {
			// ensure directory exists
			dir := avatarDir()
			if err := os.MkdirAll(dir, 0755); err != nil {
				slog.Warn("Register: MkdirAll failed", "error", err, "user_id", newUser.ID)
				return
			}
			savePath := filepath.Join(dir, filename)
			src, err := file.Open()
			if err != nil {
				slog.Warn("Register: Open avatar file failed", "error", err, "user_id", newUser.ID)
				return
			}
			defer src.Close()

			dst, err := os.Create(savePath)
			if err != nil {
				slog.Warn("Register: Create avatar file failed", "error", err, "user_id", newUser.ID)
				return
			}
			defer dst.Close()

			if _, err := io.Copy(dst, src); err != nil {
				slog.Warn("Register: SaveUploadedFile failed", "error", err, "user_id", newUser.ID)
				return
			}
			if err := models.DB.Model(&models.User{}).Where("id = ?", newUser.ID).Update("avatar", avatarURL).Error; err != nil {
				slog.Warn("Register: Update avatar failed", "error", err, "user_id", newUser.ID)
			}
			slog.Info("auth.register timing", "step", "save_avatar_async_done", "cost_ms", time.Since(avatarStart).Milliseconds(), "user_id", newUser.ID)
		})
		slog.Info("auth.register timing", "step", "queue_avatar_async", "cost_ms", time.Since(avatarStart).Milliseconds(), "user_id", newUser.ID)
	}

	// generate token
	tokenStart := time.Now()
	token, err := response.GenerateToken(newUser.ID)
	if err != nil {
		slog.Error("Register: GenerateToken failed", "error", err)
		response.Failed("注册失败", ctx)
		return
	}
	slog.Info("auth.register timing", "step", "generate_token", "cost_ms", time.Since(tokenStart).Milliseconds(), "user_id", newUser.ID)

	// 广播新用户注册事件（WebSocket），不阻塞注册响应
	wsMsg := map[string]interface{}{
		"type":     "user_registered",
		"user_id":  newUser.ID,
		"username": newUser.Username,
		"avatar":   newUser.Avatar,
	}
	runAsync("broadcast_user_registered", func() {
		if data, err := json.Marshal(wsMsg); err == nil {
			ChatHub.broadcast <- data
		}
	})
	// 将新用户加入 Team 群组，不阻塞注册响应
	runAsync("add_user_to_team", func() {
		models.AddUserToTeam(models.DB, newUser.ID)
	})

	response.Success("注册成功", gin.H{
		"token": token,
		"user": gin.H{
			"id":            newUser.ID,
			"username":      newUser.Username,
			"email":         newUser.Email,
			"avatar":        newUser.Avatar,
			"status":        newUser.Status,
			"last_login_at": newUser.LastLoginAt,
		},
	}, ctx)
	slog.Info("auth.register timing", "step", "total", "cost_ms", time.Since(start).Milliseconds(), "user_id", newUser.ID)
}
