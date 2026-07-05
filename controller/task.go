package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"go-alpha/models"
	"go-alpha/response"
)

type TaskController struct{}

func getTaskOwnerID(c *gin.Context) (uint, bool) {
	authHeader := c.GetHeader("Authorization")
	if strings.TrimSpace(authHeader) == "" {
		return 0, true
	}

	tokenStr := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	if tokenStr == "" {
		return 0, true
	}

	claims, err := response.ParseToken(tokenStr)
	if err != nil {
		// Stale token should not block guest access to public tasks.
		return 0, true
	}
	return claims.Id, true
}

func (TaskController) ListTasks(c *gin.Context) {
	userID, ok := getTaskOwnerID(c)
	if !ok {
		return
	}
	tasks := models.Task{}.GetTasksByOwner(userID)
	response.Success("查询所有任务成功", tasks, c)
}

func (TaskController) AddTask(c *gin.Context) {
	var task models.Task
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID, ok := getTaskOwnerID(c)
	if !ok {
		return
	}
	task.UserID = userID
	task.AddTask()
	response.Success("添加任务成功", &task, c)
}

func (TaskController) ToggleActive(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	userID, ok := getTaskOwnerID(c)
	if !ok {
		return
	}
	task := models.Task{}.GetTaskByIdAndOwner(uint(id), userID)
	if task.ID == 0 {
		response.Failed("任务不存在", c)
		return
	}

	var body struct {
		Active bool `json:"active"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	task.ToggleActive(body.Active)
	response.Success("更新任务状态成功", task, c)
}

func (TaskController) DeleteTask(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	userID, ok := getTaskOwnerID(c)
	if !ok {
		return
	}
	task := models.Task{}.GetTaskByIdAndOwner(uint(id), userID)
	if task.ID == 0 {
		response.Failed("任务不存在", c)
		return
	}
	models.Task{}.DeleteTask(uint(id))
	response.Success("删除任务成功", task, c)
}
