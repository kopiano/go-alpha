package controller

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go-alpha/models"
	"go-alpha/response"
)

type User struct{}

func (User) GetAllUsers(c *gin.Context) {
	users := models.User{}.GetAllUsers()
	response.Success("查询所有用户成功", users, c)
}

func (User) GetUserById(c *gin.Context) {
	id, err := strconv.Atoi(strings.TrimSpace(c.Param("id")))
	if err != nil || id <= 0 {
		response.Failed("参数错误，id 必须是正整数", c)
		return
	}
	user := models.User{}.GetUserById(id)
	if user.ID == 0 {
		response.Failed("查询失败", c)
		return
	}
	response.Success("查询单个用户成功", user, c)
}

func (User) GetUserByName(c *gin.Context) {
	name := strings.TrimSpace(c.Param("name"))
	if name == "" {
		response.Failed("参数错误，name 不能为空", c)
		return
	}
	user := models.User{}.GetUserByName(name)
	if user.ID == 0 {
		response.Failed("查询失败", c)
		return
	}
	response.Success("查询单个用户名称成功", user, c)
}

func (User) AddUser(c *gin.Context) {
	var user models.User
	if err := c.ShouldBindJSON(&user); err != nil {
		response.Failed("参数错误", c)
		return
	}
	user.Username = strings.TrimSpace(user.Username)
	user.Email = strings.TrimSpace(user.Email)
	if user.Username == "" || user.Password == "" {
		response.Failed("用户名和密码不能为空", c)
		return
	}
	if user.Email != "" && !strings.Contains(user.Email, "@") {
		response.Failed("邮箱格式不正确", c)
		return
	}
	hashPassword, err := response.HashPassword(user.Password)
	if err != nil {
		response.Failed("密码加密失败", c)
		return
	}
	user.Password = hashPassword
		if existing := (models.User{}).GetUserByName(user.Username); existing.ID != 0 {
		response.Failed("用户名已存在", c)
		return
	}
	if err := models.DB.Create(&user).Error; err != nil {
		response.Failed("添加用户失败", c)
		return
	}
	response.Success("添加单个用户成功", &user, c)
}

func (User) UpdateUser(c *gin.Context) {
	id, err := strconv.Atoi(strings.TrimSpace(c.Param("id")))
	if err != nil || id <= 0 {
		response.Failed("参数错误，id 必须是正整数", c)
		return
	}
	user := models.User{}.GetUserById(id)
	if user.ID == 0 {
		response.Failed("用户不存在", c)
		return
	}
	var form struct {
		Username *string `json:"username"`
		Email    *string `json:"email"`
		Password *string `json:"password"`
		Website  *string `json:"website"`
		Status   *string `json:"status"`
		Avatar   *string `json:"avatar"`
	}
	if err := c.ShouldBindJSON(&form); err != nil {
		response.Failed("参数错误", c)
		return
	}

	updates := map[string]any{}
	if form.Username != nil {
		username := strings.TrimSpace(*form.Username)
		if username == "" {
			response.Failed("用户名不能为空", c)
			return
		}
		if username != user.Username {
			if existing := (models.User{}).GetUserByName(username); existing.ID != 0 && existing.ID != user.ID {
				response.Failed("用户名已存在", c)
				return
			}
		}
		updates["username"] = username
	}
	if form.Email != nil {
		email := strings.TrimSpace(*form.Email)
		if email != "" && !strings.Contains(email, "@") {
			response.Failed("邮箱格式不正确", c)
			return
		}
		updates["email"] = email
	}
	if form.Password != nil {
		password := strings.TrimSpace(*form.Password)
		if password != "" {
			hashed, err := response.HashPassword(password)
			if err != nil {
				response.Failed("密码加密失败", c)
				return
			}
			updates["password"] = hashed
		}
	}
	if form.Website != nil {
		updates["website"] = strings.TrimSpace(*form.Website)
	}
	if form.Status != nil {
		updates["status"] = strings.TrimSpace(*form.Status)
	}
	if form.Avatar != nil {
		updates["avatar"] = strings.TrimSpace(*form.Avatar)
	}
	if len(updates) == 0 {
		response.Success("没有需要修改的字段", user, c)
		return
	}
	if err := models.DB.Model(&models.User{}).Where("id = ?", user.ID).Updates(updates).Error; err != nil {
		response.Failed("更新失败", c)
		return
	}
	updated := models.User{}.GetUserById(id)
	response.Success("更新成功", updated, c)
}

func (User) DeleteUser(c *gin.Context) {
	id, err := strconv.Atoi(strings.TrimSpace(c.Param("id")))
	if err != nil || id <= 0 {
		response.Failed("参数错误，id 必须是正整数", c)
		return
	}
	user := models.User{}.GetUserById(id)
	if user.ID == 0 {
		response.Failed("用户不存在", c)
		return
	}
	if err := models.DB.Delete(&models.User{}, id).Error; err != nil {
		response.Failed("删除失败", c)
		return
	}
	response.Success("删除成功", user, c)
}
