package controller

import (
	"bytes"
	"crypto/md5"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"math"
	"regexp"
	"strconv"
	"context"
	"encoding/json"
	"time"

	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"go-alpha/models"
	"go-alpha/response"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

type TransactionController struct{}

func NewTransactionController() *TransactionController {
	return &TransactionController{}
}

// ─── CSV Import ──────────────────────────────────────────────────────────────

// ImportCSV  POST /api/v1/transactions/import  — 上传 CSV 并导入交易记录
func (tc *TransactionController) ImportCSV(c *gin.Context) {
	userID, _ := c.Get("userId")

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		response.Failed("缺少 CSV 文件", c)
		return
	}
	defer file.Close()

	filename := ""
	if header != nil {
		filename = header.Filename
	}

	data, err := io.ReadAll(file)
	if err != nil {
		response.Failed("读取 CSV 文件失败", c)
		return
	}

	// 自动检测编码并转为 UTF-8
	data = detectAndDecode(data)

	slog.Info("ImportCSV: CSV preview", "preview", previewFirstBytes(data, 600))

	source := detectCSVSource(data, filename)
	slog.Info("ImportCSV: source detected", "source", source, "filename", filename)

	var txns []models.Transaction
	if source == "Alipay" {
		txns, err = parseAlipayCSV(bytes.NewReader(data), filename)
	} else {
		txns, err = parseWeChatCSV(bytes.NewReader(data), filename)
	}
	if err != nil {
		slog.Error("Transaction.ImportCSV: parse failed", "error", err)
		response.Failed("CSV 解析失败: "+err.Error(), c)
		return
	}

	// 绑定 userID
	for i := range txns {
		txns[i].UserID = userID.(uint)
	}

	// 分批写入（支付宝用专用方法，微信用通用方法）
	var inserted int64
	if source == "Alipay" {
		inserted, err = (&models.Transaction{}).BatchCreateAlipay(txns)
	} else {
		inserted, err = (&models.Transaction{}).BatchCreateWeChat(txns)
	}
	if err != nil {
		slog.Error("Transaction.ImportCSV: batch insert failed", "error", err)
		response.Failed("数据保存失败", c)
		return
	}

	duplicates := len(txns) - int(inserted)

	summary := calcSummary(txns)
	slog.Info("Transaction.ImportCSV: success",
		"user_id", userID,
		"total", len(txns),
		"inserted", inserted,
		"duplicates", duplicates,
		"income", summary.IncomeCount,
		"expense", summary.ExpenseCount,
		"neutral", summary.NeutralCount,
	)

	response.Success("导入成功", gin.H{
		"imported":   inserted,
		"duplicates": duplicates,
		"summary":    summary,
	}, c)
}

// ─── List ────────────────────────────────────────────────────────────────────

func (tc *TransactionController) List(c *gin.Context) {
	userID, _ := c.Get("userId")
	year := c.Query("year")
	month := c.Query("month")
	category := c.Query("category")
	txType := c.Query("type")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))

	filter := models.TransactionFilter{
		UserID:   userID.(uint),
		Year:     year,
		Month:    month,
		Category: category,
		Type:     txType,
		Page:     page,
		PageSize: pageSize,
	}

	txns, total, err := (models.Transaction{}).List(filter)
	if err != nil {
		slog.Error("Transaction.List: query failed", "error", err)
		response.Failed("查询失败", c)
		return
	}
	if txns == nil {
		txns = []models.Transaction{}
	}

	summary, _ := (models.Transaction{}).GetSummary(userID.(uint), year, month)
	response.Success("ok", gin.H{"list": txns, "total": total, "summary": summary}, c)
}

func (tc *TransactionController) FilterByMonth(c *gin.Context) {
	userID, _ := c.Get("userId")
	var body struct {
		Year  string `json:"year"`
		Month string `json:"month"`
		Type  string `json:"type"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		body.Year = c.Query("year")
		body.Month = c.Query("month")
		body.Type = c.Query("type")
	}
	filter := models.TransactionFilter{
		UserID:   userID.(uint),
		Year:     body.Year,
		Month:    body.Month,
		Type:     body.Type,
		Page:     1,
		PageSize: 200,
	}
	txns, total, err := (models.Transaction{}).List(filter)
	if err != nil {
		slog.Error("Transaction.FilterByMonth: query failed", "error", err)
		response.Failed("查询失败", c)
		return
	}
	if txns == nil {
		txns = []models.Transaction{}
	}
	summary, _ := (models.Transaction{}).GetSummary(userID.(uint), body.Year, body.Month)
	response.Success("ok", gin.H{"list": txns, "total": total, "summary": summary}, c)
}

func (tc *TransactionController) Summary(c *gin.Context) {
	userID, _ := c.Get("userId")
	year := c.Query("year")
	month := c.Query("month")
	summary, err := (models.Transaction{}).GetSummary(userID.(uint), year, month)
	if err != nil {
		slog.Error("Transaction.Summary: query failed", "error", err)
		response.Failed("查询失败", c)
		return
	}
	response.Success("ok", summary, c)
}

func (tc *TransactionController) Months(c *gin.Context) {
	userID, _ := c.Get("userId")
	months, err := (models.Transaction{}).GetAvailableMonths(userID.(uint))
	if err != nil {
		slog.Error("Transaction.Months: query failed", "error", err)
		response.Failed("查询失败", c)
		return
	}
	if months == nil {
		months = []string{}
	}
	response.Success("ok", gin.H{"months": months}, c)
}

func (tc *TransactionController) Delete(c *gin.Context) {
	userID, _ := c.Get("userId")
	year := c.Query("year")
	month := c.Query("month")
	err := (models.Transaction{}).DeleteByUserID(userID.(uint), year, month)
	if err != nil {
		slog.Error("Transaction.Delete: failed", "error", err)
		response.Failed("清空失败", c)
		return
	}
	response.Success("已清空交易数据", nil, c)
}

func (tc *TransactionController) CategoryBreakdown(c *gin.Context) {
	userID, _ := c.Get("userId")
	year := c.Query("year")
	month := c.Query("month")
	entries, err := (models.Transaction{}).GetCategorySummary(userID.(uint), year, month)
	if err != nil {
		slog.Error("Transaction.CategoryBreakdown: query failed", "error", err)
		response.Failed("查询失败", c)
		return
	}
	if entries == nil {
		entries = []models.CategoryEntry{}
	}
	response.Success("ok", entries, c)
}

// TopMerchants  GET /api/v1/transactions/top-merchants — 商家消费排行
func (tc *TransactionController) TopMerchants(c *gin.Context) {
	userID, _ := c.Get("userId")
	year := c.Query("year")
	month := c.Query("month")
	paymentApp := c.DefaultQuery("payment_app", "WeChat")
	results, err := (models.Transaction{}).GetTopMerchants(userID.(uint), year, month, paymentApp)
	if err != nil {
		slog.Error("Transaction.TopMerchants: query failed", "error", err)
		response.Failed("查询失败", c)
		return
	}
	if results == nil {
		results = []models.MerchantRanking{}
	}
	response.Success("ok", results, c)
}

// HotMerchants  GET /api/v1/transactions/hot-merchants — 热门商家排行（Redis 缓存）
func (tc *TransactionController) HotMerchants(c *gin.Context) {
	userID, _ := c.Get("userId")
	ctx := context.Background()
	cacheKey := fmt.Sprintf("ranking:hot_merchants:%d", userID.(uint))
	if cached, err := models.RDB.Get(ctx, cacheKey).Result(); err == nil {
		var results []models.MerchantRanking
		if json.Unmarshal([]byte(cached), &results) == nil {
			response.Success("ok", results, c)
			return
		}
	}
	results, err := (models.Transaction{}).GetHotMerchants(userID.(uint))
	if err != nil {
		slog.Error("Transaction.HotMerchants: query failed", "error", err)
		response.Failed("查询失败", c)
		return
	}
	if results == nil {
		results = []models.MerchantRanking{}
	}
	if data, err := json.Marshal(results); err == nil {
		models.RDB.Set(ctx, cacheKey, string(data), 30*time.Minute)
	}
	response.Success("ok", results, c)
}

func (tc *TransactionController) MonthlyBreakdown(c *gin.Context) {
	userID, _ := c.Get("userId")
	year := c.Query("year")
	entries, err := (models.Transaction{}).GetMonthlySummary(userID.(uint), year)
	if err != nil {
		slog.Error("Transaction.MonthlyBreakdown: query failed", "error", err)
		response.Failed("查询失败", c)
		return
	}
	if entries == nil {
		entries = []models.MonthlyEntry{}
	}
	response.Success("ok", entries, c)
}

// ═════════════════════════════════════════════════════════════════════════════
// CSV 来源检测
// ═════════════════════════════════════════════════════════════════════════════

func detectCSVSource(data []byte, filename string) string {
	if strings.HasPrefix(filename, "cashbook_record") || strings.Contains(filename, "支付宝") {
		return "Alipay"
	}
	if strings.Contains(filename, "微信") || strings.Contains(filename, "WeChat") {
		return "WeChat"
	}
	reader := csv.NewReader(bytes.NewReader(data))
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1
	for i := 0; i < 10; i++ {
		row, err := reader.Read()
		if err != nil {
			break
		}
		for _, col := range row {
			t := strings.TrimSpace(col)
			if strings.Contains(t, "支付宝") || strings.Contains(t, "Alipay") ||
				strings.Contains(t, "余额宝") || strings.Contains(t, "花呗") {
				return "Alipay"
			}
			if strings.Contains(t, "微信") || strings.Contains(t, "WeChat") {
				return "WeChat"
			}
		}
	}
	return ""
}

// ═════════════════════════════════════════════════════════════════════════════
// 支付宝 CSV 解析
// ═════════════════════════════════════════════════════════════════════════════

func parseAlipayCSV(r io.Reader, filename string) ([]models.Transaction, error) {
	reader := csv.NewReader(r)
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1
	var txns []models.Transaction
	headerFound := false
	var idx []int

	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil || len(row) < 3 {
			continue
		}
		first := strings.TrimSpace(row[0])
		if first == "" || strings.HasPrefix(first, "---") || strings.HasPrefix(first, "==") ||
			strings.Contains(first, "汇总") || strings.Contains(first, "账单明细") ||
			strings.Contains(first, "支付宝") || strings.Contains(first, "说明") {
			continue
		}

		if !headerFound {
			hasTime, hasAmount, hasType := false, false, false
			for _, col := range row {
				t := strings.TrimSpace(col)
				if strings.Contains(t, "交易时间") || strings.Contains(t, "记录时间") || t == "时间" {
					hasTime = true
				}
				if strings.Contains(t, "金额") || t == "¥" {
					hasAmount = true
				}
				if t == "收/支" || strings.Contains(t, "收入") || strings.Contains(t, "支出") || strings.Contains(t, "收支") {
					hasType = true
				}
			}
			if hasTime && hasAmount && hasType {
				headerFound = true
				idx = mapAlipayHeader(row)
				slog.Info("Alipay CSV header found", "row", row)
			}
			continue
		}

		txn, ok := parseAlipayRow(row, idx)
		if !ok {
			continue
		}
		txn.PaymentApp = "Alipay"
		txns = append(txns, txn)
	}

	if !headerFound {
		return nil, parseErr("未找到支付宝CSV表头，请确认是支付宝账单格式")
	}
	if len(txns) == 0 {
		return nil, parseErr("未能从CSV中提取到有效交易记录")
	}
	slog.Info("Alipay CSV parsed", "rows", len(txns))
	return txns, nil
}

func mapAlipayHeader(row []string) []int {
	idx := make([]int, 9)
	for i := range idx {
		idx[i] = -1
	}
	for i, col := range row {
		t := strings.TrimSpace(col)
		switch {
		case strings.Contains(t, "交易时间") || strings.Contains(t, "记录时间") || t == "时间":
			idx[0] = i // Time
		case strings.Contains(t, "分类"):
			idx[1] = i // Category
		case strings.Contains(t, "交易对方") || t == "对方":
			idx[2] = i // Merchant
		case (strings.Contains(t, "商品说明") || strings.Contains(t, "商品")) && !strings.Contains(t, "备注"):
			idx[3] = i // Product
		case t == "收/支" || strings.Contains(t, "收入") || strings.Contains(t, "支出") || strings.Contains(t, "收支类型"):
			idx[4] = i // inOut
		case strings.Contains(t, "金额") || t == "¥":
			idx[5] = i // Amount
		case (strings.Contains(t, "支付") || strings.Contains(t, "账户") || strings.Contains(t, "交易方式")) && !strings.Contains(t, "软件"):
			idx[6] = i // PaymentMethod
		case strings.Contains(t, "交易单号") || strings.Contains(t, "订单号"):
			idx[7] = i // TransactionID
		case strings.Contains(t, "备注"):
			idx[8] = i // Note
		}
	}
	return idx
}

func parseAlipayRow(row []string, idx []int) (models.Transaction, bool) {
	get := func(i int) string {
		if i < 0 || i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}

	timeStr := get(idx[0])
	category := get(idx[1])
	merchant := get(idx[2])
	product := get(idx[3])
	inOut := get(idx[4])
	amountStr := get(idx[5])
	payMethod := get(idx[6])
	transactionID := get(idx[7])
	note := get(idx[8])

	if timeStr == "" || amountStr == "" {
		return models.Transaction{}, false
	}

	amount := parseAmount(amountStr)
	if amount <= 0 {
		return models.Transaction{}, false
	}

	// 从备注中提取商家和商品信息
	if merchant == "" {
		if orderIdx := strings.Index(note, "订单号"); orderIdx >= 0 {
			part := note[orderIdx+9:]
			if semi := strings.Index(part, "；"); semi >= 0 {
				merchant = part[:semi]
			} else {
				merchant = strings.TrimSpace(part)
			}
		}
		if merchant == "" {
			merchant = category
		}
	}
	if product == "" {
		product = category
	}

	var txnType string
	switch {
	case inOut == "收入" || inOut == "收益":
		txnType = "income"
	case inOut == "支出" || inOut == "消费":
		txnType = "expense"
	default:
		text := merchant + " " + product + " " + note
		isNeutral := false
		for _, kw := range neutralKeywords {
			if strings.Contains(text, kw) {
				isNeutral = true
				break
			}
		}
		if isNeutral {
			txnType = "neutral"
		} else {
			txnType = "expense"
		}
	}

	if category == "" {
		category = autoCategory(merchant, product, "")
	}

	// 生成去重ID：按时间+类型+金额+备注 的 MD5
	if transactionID == "" {
		hash := md5.Sum([]byte(fmt.Sprintf("%s|%s|%.2f|%s|%s", timeStr, txnType, amount, merchant, note)))
		transactionID = "alipay_" + hex.EncodeToString(hash[:])
	}

	return models.Transaction{
		Time:          normalizeTime(timeStr),
		Type:          txnType,
		Merchant:      merchant,
		Product:       product,
		Amount:        amount,
		PaymentMethod: payMethod,
		PaymentApp:    "Alipay",
		Category:      category,
		Note:          note,
		TransactionID: transactionID,
	}, true
}

// ═════════════════════════════════════════════════════════════════════════════
// 微信 CSV 解析
// ═════════════════════════════════════════════════════════════════════════════

func parseWeChatCSV(r io.Reader, filename string) ([]models.Transaction, error) {
	reader := csv.NewReader(r)
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1

	var txns []models.Transaction
	headerFound := false
	var headerIdx []int

	// 优先从文件名判断
	csvSource := ""
	if strings.HasPrefix(filename, "cashbook_record") || strings.Contains(filename, "支付宝") {
		csvSource = "Alipay"
	} else if strings.Contains(filename, "微信") || strings.Contains(filename, "WeChat") {
		csvSource = "WeChat"
	}

	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil || len(row) < 3 {
			continue
		}

		first := strings.TrimSpace(row[0])
		if csvSource == "" && first != "" {
			if strings.Contains(first, "微信") {
				csvSource = "WeChat"
			} else if strings.Contains(first, "支付宝") {
				csvSource = "Alipay"
			}
		}

		if first == "" || strings.HasPrefix(first, "--") || strings.HasPrefix(first, "本笔账单") ||
			strings.Contains(first, "汇总") || strings.Contains(first, "微信支付账单") ||
			strings.Contains(first, "微信昵称") || strings.Contains(first, "起始时间") ||
			strings.Contains(first, "导出类型") {
			continue
		}

		if !headerFound {
			hasTime, hasMerchant := false, false
			for _, col := range row {
				trimmed := strings.TrimSpace(col)
				if strings.Contains(trimmed, "交易时间") {
					hasTime = true
				}
				if strings.Contains(trimmed, "交易对方") {
					hasMerchant = true
				}
			}
			if hasTime && hasMerchant {
				headerFound = true
				headerIdx = mapWeChatHeader(row)
			}
			continue
		}

		txn, ok := parseWeChatRow(row, headerIdx)
		if !ok {
			continue
		}
		txns = append(txns, txn)
	}

	if !headerFound {
		return nil, parseErr("未找到CSV表头，请确认是微信账单格式")
	}
	if len(txns) == 0 {
		return nil, parseErr("未能从CSV中提取到有效交易记录")
	}

	if csvSource != "" {
		for i := range txns {
			txns[i].PaymentApp = csvSource
		}
	}
	return txns, nil
}

func mapWeChatHeader(row []string) []int {
	idx := make([]int, 11)
	for i := range idx {
		idx[i] = -1
	}
	for i, col := range row {
		col = strings.TrimSpace(col)
		switch {
		case strings.Contains(col, "交易时间"):
			idx[0] = i
		case strings.Contains(col, "交易类型"):
			idx[1] = i
		case strings.Contains(col, "交易对方"):
			idx[2] = i
		case strings.Contains(col, "商品说明") || strings.Contains(col, "商品"):
			idx[3] = i
		case col == "收/支" || col == "收入/支出":
			idx[4] = i
		case strings.Contains(col, "金额") || col == "¥":
			idx[5] = i
		case strings.Contains(col, "支付方式") || strings.Contains(col, "支付"):
			if !strings.Contains(col, "支付软件") {
				idx[6] = i
			}
		case strings.Contains(col, "当前状态") || strings.Contains(col, "状态"):
			idx[7] = i
		case strings.Contains(col, "交易单号"):
			idx[8] = i
		case strings.Contains(col, "商户单号"):
			idx[9] = i
		case strings.Contains(col, "备注"):
			idx[10] = i
		}
	}
	return idx
}

var neutralKeywords = []string{
	"充值", "提现", "理财通", "零钱通", "信用卡还款",
	"转入", "转出", "基金", "零钱理财",
}

func parseWeChatRow(row []string, idx []int) (models.Transaction, bool) {
	get := func(i int) string {
		if i < 0 || i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}

	timeStr := get(idx[0])
	txType := get(idx[1])
	merchant := get(idx[2])
	product := get(idx[3])
	inOut := get(idx[4])
	amountStr := get(idx[5])
	payMethod := get(idx[6])
	transactionID := get(idx[8])
	note := get(idx[10])

	if timeStr == "" || amountStr == "" {
		return models.Transaction{}, false
	}

	amount := parseAmount(amountStr)
	if amount <= 0 {
		return models.Transaction{}, false
	}

	var txnType string
	switch {
	case inOut == "收入":
		txnType = "income"
	case inOut == "支出":
		txnType = "expense"
	case isNeutral(txType, product, note):
		txnType = "neutral"
	case inOut == "" || inOut == "/":
		if isNeutral(txType, product, note) {
			txnType = "neutral"
		} else {
			txnType = "expense"
		}
	default:
		txnType = "expense"
	}

	category := autoCategory(merchant, product, txType)

	return models.Transaction{
		Time:          normalizeTime(timeStr),
		Type:          txnType,
		Merchant:      merchant,
		Product:       product,
		Amount:        amount,
		PaymentMethod: payMethod,
		PaymentApp:    normalizePaymentApp(payMethod),
		Category:      category,
		Note:          note,
		TransactionID: transactionID,
	}, true
}

func isNeutral(txType, product, note string) bool {
	text := txType + " " + product + " " + note
	textLower := strings.ToLower(text)
	for _, kw := range neutralKeywords {
		if strings.Contains(text, kw) || strings.Contains(textLower, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

// ═════════════════════════════════════════════════════════════════════════════
// 自动分类
// ═════════════════════════════════════════════════════════════════════════════

func autoCategory(merchant, product, txType string) string {
	text := merchant + " " + product + " " + txType
	textLower := strings.ToLower(text)

	rules := []struct {
		keywords []string
		category string
	}{
		{[]string{"餐饮", "美食", "咖啡", "奶茶", "外卖", "饿了么", "美团外卖", "麦当劳", "肯德基", "星巴克", "瑞幸", "海底捞", "饺子", "餐厅", "食堂", "面包", "蛋糕", "生鲜", "买菜", "叮咚", "超市", "便利店"}, "餐饮"},
		{[]string{"交通", "出行", "滴滴", "地铁", "公交", "出租车", "网约车", "加油", "中石化", "中石油", "停车", "高铁", "火车票", "机票", "共享单车"}, "交通"},
		{[]string{"购物", "淘宝", "天猫", "京东", "拼多多", "唯品会", "商城", "百货", "服装", "数码", "电器", "超市", "便利店", "文具", "玩具", "家居"}, "购物"},
		{[]string{"娱乐", "电影", "影院", "万达", "游戏", "视频", "音乐", "KTV", "旅游", "酒店", "门票", "健身", "运动"}, "娱乐"},
		{[]string{"居住", "房租", "物业", "水电", "燃气", "暖气", "链家", "贝壳", "自如", "维修", "装修"}, "居住"},
		{[]string{"医疗", "医院", "诊所", "药店", "体检", "医保", "挂号", "牙科", "中医", "健康"}, "医疗"},
		{[]string{"教育", "课程", "培训", "得到", "极客时间", "知", "书", "学堂", "网易云课堂", "腾讯课堂", "付费"}, "教育"},
		{[]string{"通讯", "话费", "流量", "移动", "联通", "电信", "iCloud", "云存储", "宽带"}, "通讯"},
		{[]string{"金融", "转账", "还款", "理财", "基金", "保险", "借贷", "信用"}, "金融"},
		{[]string{"社交", "红包", "转账"}, "社交"},
	}

	for _, rule := range rules {
		for _, kw := range rule.keywords {
			if strings.Contains(text, kw) || strings.Contains(textLower, strings.ToLower(kw)) {
				return rule.category
			}
		}
	}

	return "其他"
}

// ═════════════════════════════════════════════════════════════════════════════
// 工具函数
// ═════════════════════════════════════════════════════════════════════════════

func parseAmount(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, "¥", "")
	s = strings.ReplaceAll(s, "￥", "")
	s = strings.TrimSpace(s)
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return math.Abs(f)
}

func normalizeTime(s string) string {
	re := regexp.MustCompile(`(\d{4}[-/]\d{1,2}[-/]\d{1,2})\s+(\d{1,2}:\d{2}(:\d{2})?)`)
	match := re.FindStringSubmatch(s)
	if len(match) > 0 {
		date := strings.ReplaceAll(match[1], "/", "-")
		timePart := match[2]
		if len(timePart) == 5 {
			timePart += ":00"
		}
		return date + " " + timePart
	}
	return s
}

func calcSummary(txns []models.Transaction) models.TransactionSummary {
	var s models.TransactionSummary
	for _, t := range txns {
		switch t.Type {
		case "income":
			s.IncomeCount++
			s.IncomeAmount += t.Amount
		case "expense":
			s.ExpenseCount++
			s.ExpenseAmount += t.Amount
		case "neutral":
			s.NeutralCount++
			s.NeutralAmount += t.Amount
		}
	}
	s.TotalIncome = s.IncomeAmount
	s.TotalExpense = s.ExpenseAmount + s.NeutralAmount
	return s
}

func normalizePaymentApp(s string) string {
	if strings.Contains(s, "微信") || strings.Contains(s, "WeChat") {
		return "WeChat"
	}
	if strings.Contains(s, "支付宝") || strings.Contains(s, "Alipay") ||
		strings.Contains(s, "余额宝") || strings.Contains(s, "花呗") ||
		strings.Contains(s, "余利宝") || strings.Contains(s, "借呗") {
		return "Alipay"
	}
	return s
}

func decodeGBK(data []byte) ([]byte, error) {
	reader := transform.NewReader(bytes.NewReader(data), simplifiedchinese.GBK.NewDecoder())
	return io.ReadAll(reader)
}

func parseErr(msg string) error {
	return &csvParseError{msg: msg}
}

type csvParseError struct{ msg string }

func (e *csvParseError) Error() string { return e.msg }

func previewFirstBytes(data []byte, n int) string {
	s := string(data)
	runes := []rune(s)
	if len(runes) > n {
		runes = runes[:n]
	}
	return string(runes)
}

func isValidUTF8(data []byte) bool {
	return utf8.Valid(data)
}

// detectAndDecode 自动检测 CSV 编码并转为 UTF-8
func detectAndDecode(data []byte) []byte {
	// UTF-8 BOM: \xEF\xBB\xBF
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		slog.Info("detectAndDecode: UTF-8 BOM detected, stripping BOM")
		return data[3:]
	}

	// UTF-16 LE BOM: \xFF\xFE
	if len(data) >= 2 && data[0] == 0xFF && data[1] == 0xFE {
		slog.Info("detectAndDecode: UTF-16 LE BOM detected, decoding")
		// 简单处理：尝试 GBK 或直接返回（实际场景较少）
	}

	// 已经是合法 UTF-8，不需要转码
	if utf8.Valid(data) {
		return data
	}

	// 尝试 GBK 解码
	slog.Info("detectAndDecode: not valid UTF-8, trying GBK decode")
	decoded, err := decodeGBK(data)
	if err != nil {
		slog.Warn("detectAndDecode: GBK decode failed, returning raw bytes", "error", err)
		return data
	}

	// GBK 解码后若仍不是合法 UTF-8，说明可能不是 GBK
	if !utf8.Valid(decoded) {
		slog.Warn("detectAndDecode: decoded is still not valid UTF-8, returning raw bytes")
		return data
	}

	return decoded
}
