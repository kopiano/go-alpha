package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"go-alpha/response"
)

func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(401, gin.H{"code": 401, "message": "未登录"})
			c.Abort()
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := response.ParseToken(tokenStr)
		if err != nil {
			c.JSON(401, gin.H{"code": 401, "message": "登录已过期"})
			c.Abort()
			return
		}

		c.Set("userId", claims.Id)
		c.Next()
	}
}
