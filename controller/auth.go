package controller

import (
	"fmt"
	"log/slog"
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

const avatarDir = "/app/assets/avatar"

func (c *authController) Me(ctx *gin.Context) {
	userId, _ := ctx.Get("userId")
	user := models.User{}.GetUserById(int(userId.(uint)))
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
	var form struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := ctx.ShouldBindJSON(&form); err != nil {
		response.Failed("登录失败，参数错误", ctx)
		return
	}

	user := models.User{}.GetUserByName(form.Username)
	if user.ID == 0 {
		response.Failed("用户名或密码错误", ctx)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(form.Password)); err != nil {
		response.Failed("用户名或密码错误", ctx)
		return
	}

	token, err := response.GenerateToken(user.ID)
	if err != nil {
		slog.Error("Login: GenerateToken failed", "error", err)
		response.Failed("登录失败", ctx)
		return
	}

	// update status to active on login
	models.DB.Model(user).Updates(map[string]any{
		"status":        "active",
		"last_login_at": time.Now(),
	})

	// 清除聊天联系人缓存，使对方立即看到上线状态
	invalidateChatUserInfoCache(user.ID)

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
}

func (c *authController) Logout(ctx *gin.Context) {
	userId, _ := ctx.Get("userId")
	id := int(userId.(uint))

	result := models.DB.Model(&models.User{}).Where("id = ?", id).Updates(map[string]any{
		"status":        "inactive",
		"last_login_at": time.Now().Add(-10 * time.Minute), // 退出后标记为 10 分钟前，立刻离线
	})
	slog.Info("Logout update", "id", id, "rows_affected", result.RowsAffected)

	// 清除聊天联系人缓存，使对方立即看到离线状态
	invalidateChatUserInfoCache(uint(id))

	response.Success("退出成功", nil, ctx)
}

func (c *authController) SettingUser(ctx *gin.Context) {
	userId, _ := ctx.Get("userId")
	user := models.User{}.GetUserById(int(userId.(uint)))
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
	if username != "" && username != user.Username {
		existing := models.User{}.GetUserByName(username)
		if existing.ID != 0 {
			response.Failed("用户名已存在", ctx)
			return
		}
		updates["username"] = username
	}
	if email != "" {
		updates["email"] = email
	}
	if password != "" {
		hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			response.Failed("密码加密失败", ctx)
			return
		}
		updates["password"] = string(hashed)
	}

	// Handle avatar upload (form-data only)
	file, err := ctx.FormFile("avatar")
	if err == nil {
		if err := os.MkdirAll(avatarDir, 0755); err != nil {
			slog.Warn("SettingUser: MkdirAll failed", "error", err)
		} else {
			ext := filepath.Ext(file.Filename)
			if ext == "" {
				ext = ".jpg"
			}
			filename := fmt.Sprintf("avatar-%d%s", user.ID, ext)
			savePath := filepath.Join(avatarDir, filename)
			if err := ctx.SaveUploadedFile(file, savePath); err != nil {
				slog.Warn("SettingUser: SaveUploadedFile failed", "error", err)
			} else {
				updates["avatar"] = "/api/v1/avatar/" + filename
			}
		}
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

	// 清除聊天联系人缓存，使对方立即看到最新的 username/avatar
	invalidateChatUserInfoCache(user.ID)

	// Return updated user
	updated := models.User{}.GetUserById(int(user.ID))
	response.Success("设置已保存", gin.H{
		"id":       updated.ID,
		"username": updated.Username,
		"email":    updated.Email,
		"avatar":   updated.Avatar,
	}, ctx)
}

func (c *authController) Register(ctx *gin.Context) {
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
			response.Failed("注册失败，参数错误", ctx)
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

	if username == "" || password == "" {
		response.Failed("用户名和密码不能为空", ctx)
		return
	}

	// check duplicate username
	existing := models.User{}.GetUserByName(username)
	if existing.ID != 0 {
		response.Failed("用户名已存在", ctx)
		return
	}

	// hash password
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		slog.Error("Register: bcrypt failed", "error", err)
		response.Failed("注册失败", ctx)
		return
	}

	// create user
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

	// handle optional avatar upload
	file, err := ctx.FormFile("avatar")
	if err == nil {
		// ensure directory exists
		if err := os.MkdirAll(avatarDir, 0755); err != nil {
			slog.Warn("Register: MkdirAll failed", "error", err)
		} else {
			ext := filepath.Ext(file.Filename)
			if ext == "" {
				ext = ".jpg"
			}
			filename := fmt.Sprintf("avatar-%d%s", newUser.ID, ext)
			savePath := filepath.Join(avatarDir, filename)

			if err := ctx.SaveUploadedFile(file, savePath); err != nil {
				slog.Warn("Register: SaveUploadedFile failed", "error", err)
			} else {
				avatarURL := "/api/v1/avatar/" + filename
				newUser.Avatar = avatarURL
				models.DB.Model(&newUser).Update("avatar", avatarURL)
			}
		}
	}

	// generate token
	token, err := response.GenerateToken(newUser.ID)
	if err != nil {
		slog.Error("Register: GenerateToken failed", "error", err)
		response.Failed("注册失败", ctx)
		return
	}

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
}
