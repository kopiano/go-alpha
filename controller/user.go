package controller

import (
	"net/http"
	"strconv"

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
	id, _ := strconv.Atoi(c.Param("id"))
	user := models.User{}.GetUserById(id)
	if user.ID == 0 {
		response.Failed("查询失败", c)
		return
	}
	response.Success("查询单个用户成功", user, c)
}

func (User) GetUserByName(c *gin.Context) {
	name := c.Param("name")
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	hashPassword, _ := response.HashPassword(user.Password)
	user.Password = hashPassword
	user.AddUser()
	response.Success("添加单个用户成功", &user, c)
}

func (User) UpdateUser(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	user := models.User{}.GetUserById(id)
	if user.ID == 0 {
		c.JSON(http.StatusNotFound, gin.H{})
		return
	}
	if err := c.ShouldBindJSON(user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	user.UpdateUser()
	response.Success("更新成功", user, c)
}

func (User) DeleteUser(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	user := models.User{}.GetUserById(id)
	if user.ID == 0 {
		c.JSON(http.StatusNotFound, gin.H{})
		return
	}
	models.User{}.DeleteUser(uint(id))
	response.Success("删除成功", user, c)
}
