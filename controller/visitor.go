package controller

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"go-alpha/models"
	"go-alpha/response"
)

const visitorCookieKey = "_user_uuid"

func generateUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// RecordVisit records a visit: Redis real-time counting, MySQL persistence.
func RecordVisit(c *gin.Context) {
	ctx := context.Background()
	today := time.Now().Format("2006-01-02")

	cookieUUID, err := c.Cookie(visitorCookieKey)
	isNewVisitor := (err != nil || cookieUUID == "")

	if isNewVisitor {
		cookieUUID = generateUUID()
		c.SetCookie(visitorCookieKey, cookieUUID, 365*24*3600, "/", "", false, true)
	}

	// Redis real-time counting
	models.RDB.PFAdd(ctx, fmt.Sprintf("visit:uv:%s", today), cookieUUID)
	models.RDB.PFAdd(ctx, "visit:uv:total", cookieUUID)
	models.RDB.Incr(ctx, fmt.Sprintf("visit:pv:%s", today))
	models.RDB.Incr(ctx, "visit:pv:total")

	// MySQL persistence (every visit syncs today's stats)
	todayUV, _ := models.RDB.PFCount(ctx, fmt.Sprintf("visit:uv:%s", today)).Result()
	todayPV, _ := models.RDB.Get(ctx, fmt.Sprintf("visit:pv:%s", today)).Int64()
	models.VisitorDaily{}.Upsert(today, todayUV, todayPV)

	// Refresh cache
	totalUV, _ := models.RDB.PFCount(ctx, "visit:uv:total").Result()
	totalPV, _ := models.RDB.Get(ctx, "visit:pv:total").Int64()
	models.RDB.Set(ctx, "visit:cache:total_uv", totalUV, 10*time.Minute)
	models.RDB.Set(ctx, "visit:cache:total_pv", totalPV, 10*time.Minute)

	response.Success("记录成功", gin.H{
		"is_new_visitor": isNewVisitor,
		"total_visitors": totalUV,
	}, c)
}

// GetVisitor reads from Redis cache, falls back to MySQL.
func GetVisitor(c *gin.Context) {
	ctx := context.Background()
	today := time.Now().Format("2006-01-02")

	totalUV, err := models.RDB.Get(ctx, "visit:cache:total_uv").Int64()
	if err == nil {
		// Cache hit — read everything from Redis
		totalPV, _ := models.RDB.Get(ctx, "visit:cache:total_pv").Int64()
		todayUV, _ := models.RDB.PFCount(ctx, fmt.Sprintf("visit:uv:%s", today)).Result()
		todayPV, _ := models.RDB.Get(ctx, fmt.Sprintf("visit:pv:%s", today)).Int64()

		weekKeys := make([]string, 7)
		for i := 0; i < 7; i++ {
			d := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
			weekKeys[i] = fmt.Sprintf("visit:uv:%s", d)
		}
		weekUV, _ := models.RDB.PFCount(ctx, weekKeys...).Result()

		response.Success("获取统计成功", gin.H{
			"today_uv":  todayUV,
			"today_pv":  todayPV,
			"weekly_uv": weekUV,
			"total_uv":  totalUV,
			"total_pv":  totalPV,
		}, c)
		return
	}

	// Cache miss — read today from MySQL, total from Redis HLL
	var daily models.VisitorDaily
	models.DB.Where("date = ?", today).First(&daily)

	weekStart := time.Now().AddDate(0, 0, -6).Format("2006-01-02")
	var weekStats []models.VisitorDaily
	models.DB.Where("date >= ?", weekStart).Find(&weekStats)

	var weekUV int64
	for _, s := range weekStats {
		weekUV += s.UV
	}

	totalUV, _ = models.RDB.PFCount(ctx, "visit:uv:total").Result()
	totalPV, _ := models.RDB.Get(ctx, "visit:pv:total").Int64()

	response.Success("获取统计成功", gin.H{
		"today_uv":  daily.UV,
		"today_pv":  daily.PV,
		"weekly_uv": weekUV,
		"total_uv":  totalUV,
		"total_pv":  totalPV,
	}, c)
}
