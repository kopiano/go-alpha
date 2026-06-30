package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"go-alpha/models"
	"go-alpha/response"
)

type ipLocationResponse struct {
	CountryName string `json:"country_name"`
	City        string `json:"city"`
}

type ipLocation struct {
	Country  string `json:"country"`
	City     string `json:"city"`
	Location string `json:"location"`
}

type visitForm struct {
	VisitorID         string `json:"visitor_id" binding:"required"`
	DeviceFingerprint string `json:"device_fingerprint"`
	Duration          int64  `json:"duration"`
	OS                string `json:"os"`
	Browser           string `json:"browser"`
	Device            string `json:"device"`
	UserAgent         string `json:"user_agent"`
	PageURL           string `json:"page_url"`
	Referrer          string `json:"referrer"`
	Country           string `json:"country"`
	City              string `json:"city"`
	Location          string `json:"location"`
	Status            string `json:"status"`
	UserName          string `json:"user_name"`
	Avatar            string `json:"avatar"`
}

type visitorItem struct {
	VisitorID       string    `json:"visitor_id"`
	IP              string    `json:"ip"`
	Country         string    `json:"country"`
	City            string    `json:"city"`
	Region          string    `json:"region"`
	FirstSeen       time.Time `json:"first_seen"`
	LastSeen        time.Time `json:"last_seen"`
	Duration        int64     `json:"duration"`
	TotalBrowseTime int64     `json:"total_browse_time"`
	OS              string    `json:"os"`
	Browser         string    `json:"browser"`
	Brower          string    `json:"brower"`
	Device          string    `json:"device"`
	Status          string    `json:"status"`
	UserName        string    `json:"user_name"`
	Avatar          string    `json:"avatar"`
}

func isPrivateIP(ip string) bool {
	parsed := net.ParseIP(strings.TrimSpace(ip))
	if parsed == nil {
		return true
	}
	return parsed.IsLoopback() || parsed.IsPrivate() || parsed.IsLinkLocalUnicast()
}

func getClientIP(c *gin.Context) string {
	for _, header := range []string{"CF-Connecting-IP", "X-Real-IP", "X-Forwarded-For"} {
		value := strings.TrimSpace(c.GetHeader(header))
		if value == "" {
			continue
		}
		ip := strings.TrimSpace(strings.Split(value, ",")[0])
		if ip != "" && !isPrivateIP(ip) {
			return ip
		}
	}
	return c.ClientIP()
}

func getIPLocation(ip string) ipLocation {
	client := http.Client{Timeout: 2 * time.Second}
	url := "https://ipapi.co/json/"
	if strings.TrimSpace(ip) != "" && !isPrivateIP(ip) {
		url = fmt.Sprintf("https://ipapi.co/%s/json/", ip)
	}

	resp, err := client.Get(url)
	if err != nil {
		return ipLocation{}
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return ipLocation{}
	}

	var location ipLocationResponse
	if err := json.NewDecoder(resp.Body).Decode(&location); err != nil {
		return ipLocation{}
	}

	country := strings.TrimSpace(location.CountryName)
	city := strings.TrimSpace(location.City)
	result := ipLocation{
		Country: country,
		City:    city,
	}
	if country == "" {
		result.Location = city
		return result
	}
	if city == "" || country == city {
		result.Location = country
		return result
	}
	result.Location = country + " " + city
	return result
}

func visitorLocationData(c *gin.Context) gin.H {
	ip := getClientIP(c)
	location := getIPLocation(ip)
	return gin.H{
		"ip":       ip,
		"country":  location.Country,
		"city":     location.City,
		"location": location.Location,
	}
}

func detectOS(userAgent string) string {
	ua := strings.ToLower(userAgent)
	switch {
	case strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad") || strings.Contains(ua, "ios"):
		return "iOS"
	case strings.Contains(ua, "mac os") || strings.Contains(ua, "macintosh"):
		return "macOS"
	case strings.Contains(ua, "windows"):
		return "Windows"
	default:
		return "Unknown"
	}
}

func detectBrowser(userAgent string) string {
	ua := strings.ToLower(userAgent)
	switch {
	case strings.Contains(ua, "edg/"):
		return "Edge"
	case strings.Contains(ua, "chrome/") || strings.Contains(ua, "crios/"):
		return "Chrome"
	case strings.Contains(ua, "safari/"):
		return "Safari"
	default:
		return "Unknown"
	}
}

func detectDevice(userAgent string) string {
	ua := strings.ToLower(userAgent)
	switch {
	case strings.Contains(ua, "ipad") || strings.Contains(ua, "tablet"):
		return "Tablet"
	case strings.Contains(ua, "mobile") || strings.Contains(ua, "iphone") || strings.Contains(ua, "android"):
		return "Mobile"
	default:
		return "Desktop"
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func visitorRegion(visitor models.Visitor) string {
	return firstNonEmpty(visitor.Location, strings.TrimSpace(visitor.Country+" "+visitor.City))
}

func currentUser(c *gin.Context) *models.User {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return nil
	}
	tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
	claims, err := response.ParseToken(tokenStr)
	if err != nil {
		return nil
	}
	user := models.User{}.GetUserById(int(claims.Id))
	if user.ID == 0 {
		return nil
	}
	return user
}

func buildVisitorStats(visitors []models.Visitor) (int64, int64, []visitorItem) {
	activeAfter := time.Now().Add(-5 * time.Minute)
	var activeVisitors int64
	var totalBrowseTime int64
	items := make([]visitorItem, 0, len(visitors))

	for _, visitor := range visitors {
		totalBrowseTime += visitor.TotalBrowseTime
		status := visitor.Status
		if visitor.LastSeen.After(activeAfter) {
			activeVisitors++
			status = "active"
		} else {
			status = "inactive"
		}
		items = append(items, visitorItem{
			VisitorID:       visitor.VisitorID,
			IP:              visitor.IP,
			Country:         visitor.Country,
			City:            visitor.City,
			Region:          visitorRegion(visitor),
			FirstSeen:       visitor.FirstSeen,
			LastSeen:        visitor.LastSeen,
			Duration:        visitor.TotalBrowseTime,
			TotalBrowseTime: visitor.TotalBrowseTime,
			OS:              visitor.OS,
			Browser:         visitor.Browser,
			Brower:          visitor.Browser,
			Device:          visitor.Device,
			Status:          status,
			UserName:        visitor.UserName,
			Avatar:          visitor.Avatar,
		})
	}

	return activeVisitors, totalBrowseTime, items
}

func visitorStatsData(totalPV, totalUV, todayPV, todayUV, weeklyUV int64, visitors []models.Visitor) gin.H {
	activeVisitors, totalBrowseTime, visitorItems := buildVisitorStats(visitors)
	return gin.H{
		"pv":                totalPV,
		"uv":                totalUV,
		"total_pv":          totalPV,
		"total_uv":          totalUV,
		"today_pv":          todayPV,
		"today_uv":          todayUV,
		"weekly_uv":         weeklyUV,
		"active_visitors":   activeVisitors,
		"total_browse_time": totalBrowseTime,
		"visitors":          visitorItems,
	}
}

// RecordVisit records a visit: Redis real-time counting, MySQL persistence.
func RecordVisit(c *gin.Context) {
	var form visitForm
	if err := c.ShouldBindJSON(&form); err != nil {
		response.Failed("参数错误，visitor_id 为必填", c)
		return
	}

	visitorID := strings.TrimSpace(form.VisitorID)
	if visitorID == "" {
		response.Failed("参数错误，visitor_id 不能为空", c)
		return
	}

	if form.Duration < 0 {
		form.Duration = 0
	}

	ctx := context.Background()
	today := time.Now().Format("2006-01-02")

	// Redis real-time counting
	slog.Debug("RecordVisit called", "visitor_id", visitorID, "duration", form.Duration)

	models.RDB.PFAdd(ctx, fmt.Sprintf("visit:uv:%s", today), visitorID)
	models.RDB.PFAdd(ctx, "visit:uv:total", visitorID)
	models.RDB.Incr(ctx, fmt.Sprintf("visit:pv:%s", today))
	models.RDB.Incr(ctx, "visit:pv:total")

	// Refresh cache
	totalUV, _ := models.RDB.PFCount(ctx, "visit:uv:total").Result()
	totalPV, _ := models.RDB.Get(ctx, "visit:pv:total").Int64()
	models.RDB.Set(ctx, "visit:cache:total_uv", totalUV, 10*time.Minute)
	models.RDB.Set(ctx, "visit:cache:total_pv", totalPV, 10*time.Minute)

	locationData := visitorLocationData(c)
	country := firstNonEmpty(form.Country, fmt.Sprint(locationData["country"]))
	city := firstNonEmpty(form.City, fmt.Sprint(locationData["city"]))
	location := firstNonEmpty(form.Location, fmt.Sprint(locationData["location"]))
	if location == "" {
		location = strings.TrimSpace(country + " " + city)
	}

	userAgent := firstNonEmpty(form.UserAgent, c.GetHeader("User-Agent"))
	osName := firstNonEmpty(form.OS, detectOS(userAgent))
	browser := firstNonEmpty(form.Browser, detectBrowser(userAgent))
	device := firstNonEmpty(form.Device, detectDevice(userAgent))
	status := firstNonEmpty(form.Status, "active")
	userName := strings.TrimSpace(form.UserName)
	avatar := strings.TrimSpace(form.Avatar)
	if user := currentUser(c); user != nil {
		userName = user.Username
		avatar = user.Avatar
		status = firstNonEmpty(user.Status, status)
	}

	visitor := models.Visitor{
		VisitorID: visitorID,
			DeviceFingerprint: strings.TrimSpace(form.DeviceFingerprint),

		IP:        fmt.Sprint(locationData["ip"]),
		Country:   country,
		City:      city,
		Location:  location,
		OS:        osName,
		Browser:   browser,
		Device:    device,
		Status:    status,
		UserName:  userName,
		Avatar:    avatar,
	}
	savedVisitor, err := visitor.UpsertVisit(form.Duration)
	if err != nil {
		slog.Error("Visitor.UpsertVisit failed", "error", err)
		response.Failed("记录访问失败", c)
		return
	}

	// Persist today's UV/PV
	var todayUV int64
	models.DB.Model(&models.Visitor{}).
		Where("DATE(last_seen) = ?", today).
		Count(&todayUV)
	todayPV, _ := models.RDB.Get(ctx, fmt.Sprintf("visit:pv:%s", today)).Int64()
	models.VisitorSummary{}.Upsert(today, todayUV, todayPV)


	data := gin.H{
		"visitor_id":     visitorID,
		"total_visitors": totalUV,
		"duration":       form.Duration,
		"ip":             fmt.Sprint(locationData["ip"]),
		"country":        country,
		"city":           city,
		"location":       location,
		"os":             osName,
		"browser":        browser,
		"brower":         browser,
		"device":         device,
		"status":         status,
		"user_name":      userName,
		"avatar":         avatar,
		"visitor":        savedVisitor,
	}
	response.Success("记录成功", data, c)
}

type heartbeatForm struct {
	VisitorID string `json:"visitor_id" binding:"required"`
	Duration  int64  `json:"duration"`
	UserName  string `json:"user_name"`
}

func VisitorHeartbeat(c *gin.Context) {
	var form heartbeatForm
	if err := c.ShouldBindJSON(&form); err != nil {
		response.Failed("参数错误，visitor_id 为必填", c)
		return
	}

	visitorID := strings.TrimSpace(form.VisitorID)
	if visitorID == "" {
		response.Failed("参数错误，visitor_id 不能为空", c)
		return
	}

	visitor := models.Visitor{VisitorID: visitorID}
	if err := visitor.AddDuration(form.Duration); err != nil {
		slog.Error("Visitor.AddDuration failed", "error", err)
		response.Failed("更新访客失败", c)
		return
	}

	// If username is provided, also update it (handles guest → login transition
	// where the heartbeat arrives after the user logged in).
	if userName := strings.TrimSpace(form.UserName); userName != "" {
		if err := visitor.UpdateUserName(userName); err != nil {
			slog.Warn("Visitor.UpdateUserName failed", "error", err)
		}
	}

	response.Success("更新成功", gin.H{
		"visitor_id": visitorID,
		"duration":   form.Duration,
	}, c)
}

// GetVisitor 统一从 MySQL 读，Redis 只做缓存加速
// Query params: visitor_id, fingerprint, ip — used to identify current visitor
func GetVisitor(c *gin.Context) {
	ctx := context.Background()
	today := time.Now().Format("2006-01-02")

	// —— Identify current visitor via query params ——
	visitorID := c.Query("visitor_id")
	fingerprint := c.Query("fingerprint")
	clientIP := c.Query("ip")

	var currentVisitor *models.Visitor
	if visitorID != "" || fingerprint != "" {
		// 1) Try visitor_id
		if visitorID != "" {
			var v models.Visitor
			if err := models.DB.Where("visitor_id = ?", visitorID).First(&v).Error; err == nil {
				currentVisitor = &v
			}
		}
		// 2) Fallback: fingerprint + IP
		if currentVisitor == nil && fingerprint != "" && clientIP != "" {
			var v models.Visitor
			if err := models.DB.Where("device_fingerprint = ? AND ip = ?", fingerprint, clientIP).First(&v).Error; err == nil {
				currentVisitor = &v
			}
		}
		// 3) Fallback: fingerprint alone
		if currentVisitor == nil && fingerprint != "" {
			var v models.Visitor
			if err := models.DB.Where("device_fingerprint = ?", fingerprint).First(&v).Error; err == nil {
				currentVisitor = &v
			}
		}
	}

	visitors, visitorsErr := models.Visitor{}.GetAllVisitors()
	if visitorsErr != nil {
		slog.Error("Visitor.GetAllVisitors failed", "error", visitorsErr)
		response.Failed("获取访客失败", c)
		return
	}

	// Today's UV/PV
	var daily models.VisitorSummary
	models.DB.Where("date = ?", today).First(&daily)

	// Week UV = sum of UV per day
	weekStart := time.Now().AddDate(0, 0, -6).Format("2006-01-02")
	var weekStats []models.VisitorSummary
	models.DB.Where("date >= ?", weekStart).Find(&weekStats)
	var weekUV int64
	for _, s := range weekStats {
		weekUV += s.UV
	}

	// Total PV/UV — try Redis cache, fall back to MySQL
	totalUV, err := models.RDB.Get(ctx, "visit:cache:total_uv").Int64()
	var totalPV int64
	if err == nil {
		totalPV, _ = models.RDB.Get(ctx, "visit:cache:total_pv").Int64()
	} else {
		models.DB.Model(&models.VisitorSummary{}).Select("COALESCE(SUM(pv), 0)").Scan(&totalPV)
		models.DB.Model(&models.Visitor{}).Select("COUNT(*)").Scan(&totalUV)
		models.RDB.Set(ctx, "visit:cache:total_uv", totalUV, 10*time.Minute)
		models.RDB.Set(ctx, "visit:cache:total_pv", totalPV, 10*time.Minute)
	}

	data := visitorStatsData(totalPV, totalUV, daily.PV, daily.UV, weekUV, visitors)
	if currentVisitor != nil {
		data["current_visitor"] = gin.H{
			"visitor_id": currentVisitor.VisitorID,
			"ip":         currentVisitor.IP,
			"country":    currentVisitor.Country,
			"city":       currentVisitor.City,
			"location":   currentVisitor.Location,
			"user_name":  currentVisitor.UserName,
		}
	}
	response.Success("获取统计成功", data, c)
}

// VisitorDaily 返回 visitor_summary 表所有记录
func VisitorDaily(c *gin.Context) {
	var list []models.VisitorSummary
	if err := models.DB.Order("date desc").Find(&list).Error; err != nil {
		slog.Error("VisitorDaily failed", "error", err)
		response.Failed("获取失败", c)
		return
	}
	response.Success("ok", list, c)
}

// VisitorPvUv 统计所有日期的总 PV 和总 UV
func VisitorPvUv(c *gin.Context) {
	var result struct {
		TotalPV int64 `gorm:"column:total_pv" json:"total_pv"`
		TotalUV int64 `gorm:"column:total_uv" json:"total_uv"`
	}
	if err := models.DB.Model(&models.VisitorSummary{}).
		Select("COALESCE(SUM(pv), 0) as total_pv, COALESCE(SUM(uv), 0) as total_uv").
		Scan(&result).Error; err != nil {
		slog.Error("VisitorPvUv failed", "error", err)
		response.Failed("获取失败", c)
		return
	}
	response.Success("ok", result, c)
}
