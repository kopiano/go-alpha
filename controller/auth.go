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
	models.DB.Model(user).Update("status", "active")

	response.Success("登录成功", gin.H{
		"token": token,
		"user": gin.H{
			"id":            user.ID,
			"username":      user.Username,
			"email":         user.Email,
			"avatar":        user.Avatar,
			"status":        "active",
			"last_login_at": user.LastLoginAt,
		},
	}, ctx)
}

func (c *authController) Logout(ctx *gin.Context) {
	userId, _ := ctx.Get("userId")
	user := models.User{}.GetUserById(int(userId.(uint)))
	if user.ID != 0 {
		now := time.Now()
		models.DB.Model(user).Updates(map[string]any{
			"status":        "inactive",
			"last_login_at": now,
		})
	}
	response.Success("退出成功", nil, ctx)
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
