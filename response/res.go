package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
	Error   any    `json:"error"`
}

// Success 请求成功
func Success(message string, data any, c *gin.Context) {
	c.JSON(http.StatusOK, Response{
		Code:    200,
		Message: message,
		Data:    data,
		Error:   nil,
	})
}

// Failed 请求失败
func Failed(message string, c *gin.Context) {
	c.JSON(http.StatusOK, Response{
		Code:    400,
		Message: message,
		Data:    0,
		Error:   nil,
	})
}
