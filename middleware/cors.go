package middleware

import (
	"net"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// 判断 origin 是否为本地/内网/ngrok/正式域名
func allowOrigin(origin string) bool {
	host := origin
	for _, prefix := range []string{"http://", "https://"} {
		if strings.HasPrefix(host, prefix) {
			host = strings.TrimPrefix(host, prefix)
			break
		}
	}
	if idx := strings.LastIndex(host, ":"); idx >= 0 {
		host = host[:idx]
	}

	// 本地开发
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}

	// 局域网 IP
	if ip := net.ParseIP(host); ip != nil && (ip.IsPrivate() || ip.IsLoopback()) {
		return true
	}

	// 正式域名
	if origin == "http://alpha.coulsonzero.shop" || origin == "https://alpha.coulsonzero.shop" {
		return true
	}

	// ngrok 隧道
	if strings.HasSuffix(host, ".ngrok-free.dev") ||
		strings.HasSuffix(host, ".ngrok.io") ||
		strings.HasSuffix(host, ".ngrok.app") {
		return true
	}

	return false
}

func CORS() gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowOriginFunc:  allowOrigin,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "Accept", "X-Requested-With"},
		ExposeHeaders:    []string{"Content-Length", "Content-Type"},
		AllowCredentials: true,
	})
}
