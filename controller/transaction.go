package controller

import (
	"encoding/csv"
	"io"
	"log/slog"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go-alpha/models"
	"go-alpha/response"
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

	txns, err := parseWeChatCSV(file, filename)
	if err != nil {
		slog.Error("Transaction.ImportCSV: parse failed", "error", err)
		response.Failed("CSV 解析失败: "+err.Error(), c)
		return
	}

	// 为每条记录绑定 userID
	for i := range txns {
		txns[i].UserID = userID.(uint)
	}

	// 批量写入（跳过重复交易单号）
	inserted, err := (&models.Transaction{}).BatchCreate(txns)
	if err != nil {
		slog.Error("Transaction.ImportCSV: batch insert failed", "error", err)
		response.Failed("数据保存失败", c)
		return
	}
	duplicates := len(txns) - int(inserted)

	// 统计
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

// List  GET /api/v1/transactions — 获取交易记录（分页 + 筛选）
func (tc *TransactionController) List(c *gin.Context) {
	userID, _ := c.Get("userId")

	year := c.Query("year")
	month := c.Query("month")
	category := c.Query("category")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))

	filter := models.TransactionFilter{
		UserID:   userID.(uint),
		Year:     year,
		Month:    month,
		Category: category,
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

	// 同时返回汇总数据
	summary, _ := (models.Transaction{}).GetSummary(userID.(uint), year, month)

	response.Success("ok", gin.H{
		"list":    txns,
		"total":   total,
		"summary": summary,
	}, c)
}

// ─── Filter (POST) ──────────────────────────────────────────────────────────

// FilterByMonth  POST /api/v1/transactions/filter — 按年月筛选交易记录
func (tc *TransactionController) FilterByMonth(c *gin.Context) {
	userID, _ := c.Get("userId")

	var body struct {
		Year  string `json:"year"`
		Month string `json:"month"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		// 如果 body 为空或非 JSON，回退到查询参数
		body.Year = c.Query("year")
		body.Month = c.Query("month")
	}

	filter := models.TransactionFilter{
		UserID:   userID.(uint),
		Year:     body.Year,
		Month:    body.Month,
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

	response.Success("ok", gin.H{
		"list":    txns,
		"total":   total,
		"summary": summary,
	}, c)
}

// ─── Summary ────────────────────────────────────────────────────────────────

// Summary  GET /api/v1/transactions/summary — 收支汇总
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

// ─── Months ─────────────────────────────────────────────────────────────────

// Months  GET /api/v1/transactions/months — 用户有记录的所有月份
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

// ─── Delete ─────────────────────────────────────────────────────────────────

// Delete  DELETE /api/v1/transactions — 清空当前用户交易记录
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

	slog.Info("Transaction.Delete: done", "user_id", userID)
	response.Success("已清空交易数据", nil, c)
}

// ─── Category Breakdown ─────────────────────────────────────────────────────

// CategoryBreakdown  GET /api/v1/transactions/categories — 分类支出
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

// ─── Monthly Breakdown ──────────────────────────────────────────────────────

// MonthlyBreakdown  GET /api/v1/transactions/monthly — 月度支出趋势
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
// CSV 解析
// ═════════════════════════════════════════════════════════════════════════════

// parseWeChatCSV 解析微信账单 CSV 文件
// 微信导出格式：
//   第1行: "微信支付账单明细列表"
//   第2行: 微信昵称
//   第3行: 起始/终止时间
//   第4行: 导出类型
//   第5行: "----------------------------------"
//   第6行: 表头 (交易时间,交易类型,交易对方,商品说明,收/支,金额,支付方式,当前状态,交易单号,商户单号,备注)
//   第7+行: 数据行
//   最后几行: 汇总信息
func parseWeChatCSV(r io.Reader, filename string) ([]models.Transaction, error) {
	reader := csv.NewReader(r)
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1 // 允许可变列数

	var txns []models.Transaction
	var csvSource string // 从 CSV 元数据行检测来源（WeChat / Alipay）

	// 优先从文件名判断
	if strings.HasPrefix(filename, "cashbook_record") || strings.Contains(filename, "支付宝") {
		csvSource = "Alipay"
	} else if strings.Contains(filename, "微信") || strings.Contains(filename, "WeChat") {
		csvSource = "WeChat"
	}
	headerFound := false
	var headerIdx []int // 各字段在 CSV 列中的索引

	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue // 跳过解析错误的行
		}
		if len(row) < 3 {
			continue
		}

		// 跳过空行、分隔线、汇总行
		first := strings.TrimSpace(row[0])

		// 从文件内容检测来源（微信/支付宝）
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

		// 查找表头行 — 检查所有列，"交易时间"和"交易对方"在不同列中
		if !headerFound {
			hasTime := false
			hasMerchant := false
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
				slog.Info("CSV header found", "row", row)
			}
			continue
		}

		// 解析数据行
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

	// 统一设置支付软件来源
	if csvSource != "" {
		for i := range txns {
			txns[i].PaymentApp = csvSource
		}
		slog.Info("CSV source detected", "source", csvSource, "rows", len(txns))
	}

	return txns, nil
}

// mapWeChatHeader 映射微信 CSV 表头列
// 返回各字段在 row 中的索引位置
func mapWeChatHeader(row []string) []int {
	// 索引顺序: [交易时间, 交易类型, 交易对方, 商品说明, 收/支, 金额, 支付方式, 状态, 交易单号, 商户单号, 备注]
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

// 中性交易关键词
var neutralKeywords = []string{
	"充值", "提现", "理财通", "零钱通", "信用卡还款",
	"转入", "转出", "基金", "零钱理财",
}

// parseWeChatRow 解析一行微信交易记录
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

	// 解析金额
	amount := parseAmount(amountStr)
	if amount <= 0 {
		return models.Transaction{}, false
	}

	// 判断交易类型
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
			txnType = "expense" // 默认
		}
	default:
		txnType = "expense"
	}

	// 自动分类
	category := autoCategory(merchant, product, txType)

	return models.Transaction{
		Time:           normalizeTime(timeStr),
		Type:           txnType,
		Merchant:       merchant,
		Product:        product,
		Amount:         amount,
		PaymentMethod:  payMethod,
		PaymentApp:     normalizePaymentApp(payMethod),
		Category:       category,
		Note:           note,
		TransactionID:  transactionID,
	}, true
}

// isNeutral 判断是否是中性交易
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

// autoCategory 根据商家/商品名自动分类
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

// parseAmount 解析金额字符串 "¥38.00"、"38.00"、"-¥38.00"、"-38.00"
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

// normalizeTime 规范时间格式
func normalizeTime(s string) string {
	// 微信格式: "2026-07-01 12:30:00"
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

// calcSummary 计算一批交易的汇总数据
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
	// 微信支付
	if strings.Contains(s, "微信") || strings.Contains(s, "WeChat") {
		return "WeChat"
	}
	// 支付宝及阿里系
	if strings.Contains(s, "支付宝") || strings.Contains(s, "Alipay") ||
		strings.Contains(s, "余额宝") || strings.Contains(s, "花呗") ||
		strings.Contains(s, "余利宝") || strings.Contains(s, "借呗") {
		return "Alipay"
	}
	return s
}

func parseErr(msg string) error {
	return &csvParseError{msg: msg}
}

type csvParseError struct{ msg string }

func (e *csvParseError) Error() string { return e.msg }
