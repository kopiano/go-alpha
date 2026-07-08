package middleware

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func CORS() gin.HandlerFunc {
	return cors.New(cors.Config{
		// AllowOriginFunc:  allowOrigin,
		AllowOrigins: []string{
			"https://alpha.coulsonzero.shop",
			"http://localhost:5000",
			"http://127.0.0.1:5000",
			// "http://192.168.31.194:5000",
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "Accept", "X-Requested-With"},
		ExposeHeaders:    []string{"Content-Length", "Content-Type"},
		AllowCredentials: true,
	})
}
