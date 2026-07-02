package models

import (
	"time"

	"gorm.io/gorm"
)

// Transaction 交易记录表
type Transaction struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	UserID        uint           `gorm:"index;not null" json:"user_id"`
	Time          string         `gorm:"type:varchar(50);index" json:"time"`             // 交易时间
	Type          string         `gorm:"type:varchar(20);index" json:"type"`             // income / expense / neutral
	Merchant      string         `gorm:"type:varchar(255)" json:"merchant"`              // 商家（交易对方）
	Product       string         `gorm:"type:varchar(255)" json:"product"`               // 商品说明
	Amount        float64        `gorm:"type:decimal(12,2);not null" json:"amount"`      // 金额
	PaymentMethod string         `gorm:"type:varchar(100)" json:"payment_method"`         // 支付方式
	PaymentApp    string         `gorm:"type:varchar(100)" json:"payment_app"`           // 支付软件
	Category      string         `gorm:"type:varchar(100);index" json:"category"`        // 分类
	Note          string         `gorm:"type:text" json:"note"`                          // 备注
	TransactionID  string         `gorm:"type:varchar(100);index" json:"transaction_id"`  // 交易单号（应用层去重）
}

// ─── Query params ───

type TransactionFilter struct {
	UserID   uint
	Year     string // "2026"
	Month    string // "07"
	Category string
	Page     int
	PageSize int
}

type TransactionSummary struct {
	IncomeCount  int     `json:"income_count"`
	IncomeAmount float64 `json:"income_amount"`
	ExpenseCount int     `json:"expense_count"`
	ExpenseAmount float64 `json:"expense_amount"`
	NeutralCount int     `json:"neutral_count"`
	NeutralAmount float64 `json:"neutral_amount"`
	TotalIncome  float64 `json:"total_income"`
	TotalExpense float64 `json:"total_expense"`
}

// ─── Model methods ───

func (Transaction) TableName() string {
	return "transaction"
}

// BatchCreate 批量插入交易记录（根据交易单号去重）
func (t *Transaction) BatchCreate(txns []Transaction) (int64, error) {
	if len(txns) == 0 {
		return 0, nil
	}

	// 查出该用户已有交易单号
	var existingIDs []string
	DB.Model(&Transaction{}).
		Where("user_id = ? AND transaction_id IS NOT NULL AND transaction_id != ''", txns[0].UserID).
		Pluck("transaction_id", &existingIDs)

	idSet := make(map[string]bool, len(existingIDs))
	for _, id := range existingIDs {
		idSet[id] = true
	}

	// 过滤重复
	var newTxns []Transaction
	for _, txn := range txns {
		if txn.TransactionID == "" || !idSet[txn.TransactionID] {
			newTxns = append(newTxns, txn)
		}
	}

	if len(newTxns) == 0 {
		return 0, nil
	}

	result := DB.CreateInBatches(newTxns, 100)
	return result.RowsAffected, result.Error
}

// List 查询交易记录（带分页和筛选）
func (Transaction) List(filter TransactionFilter) ([]Transaction, int64, error) {
	var txns []Transaction
	var total int64

	query := DB.Model(&Transaction{}).Where("user_id = ?", filter.UserID)

	if filter.Year != "" {
		query = query.Where("LEFT(time, 4) = ?", filter.Year)
	}
	if filter.Month != "" {
		// time format: "2026-07-01 12:30" -> LEFT(time, 7) = "2026-07"
		query = query.Where("LEFT(time, 7) = ?", filter.Year+"-"+filter.Month)
	}
	if filter.Category != "" {
		query = query.Where("category = ?", filter.Category)
	}

	query.Count(&total)

	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 50
	}
	offset := (filter.Page - 1) * filter.PageSize

	err := query.Order("time DESC, id DESC").Offset(offset).Limit(filter.PageSize).Find(&txns).Error
	return txns, total, err
}

// GetSummary 获取收支汇总
func (Transaction) GetSummary(userID uint, year, month string) (TransactionSummary, error) {
	var summary TransactionSummary

	query := DB.Model(&Transaction{}).Where("user_id = ?", userID)
	if year != "" {
		query = query.Where("LEFT(time, 4) = ?", year)
	}
	if month != "" {
		query = query.Where("LEFT(time, 7) = ?", year+"-"+month)
	}

	type Row struct {
		Type   string
		Count  int
		Amount float64
	}
	var rows []Row

	err := query.Select("type, COUNT(*) as count, SUM(amount) as amount").
		Group("type").
		Find(&rows).Error
	if err != nil {
		return summary, err
	}

	for _, r := range rows {
		switch r.Type {
		case "income":
			summary.IncomeCount = r.Count
			summary.IncomeAmount = r.Amount
		case "expense":
			summary.ExpenseCount = r.Count
			summary.ExpenseAmount = r.Amount
		case "neutral":
			summary.NeutralCount = r.Count
			summary.NeutralAmount = r.Amount
		}
	}

	summary.TotalIncome = summary.IncomeAmount
	summary.TotalExpense = summary.ExpenseAmount + summary.NeutralAmount

	return summary, nil
}

// GetAvailableMonths 获取用户有交易记录的月份列表（年-月）
func (Transaction) GetAvailableMonths(userID uint) ([]string, error) {
	type MonthRow struct {
		Month string `gorm:"column:month"`
	}
	var rows []MonthRow
	err := DB.Model(&Transaction{}).
		Where("user_id = ? AND time IS NOT NULL AND time != ''", userID).
		Select("DISTINCT LEFT(time, 7) as month").
		Order("LEFT(time, 7) DESC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	months := make([]string, 0, len(rows))
	for _, r := range rows {
		if r.Month != "" {
			months = append(months, r.Month)
		}
	}
	return months, nil
}

// MonthlyEntry 月度汇总条目
type MonthlyEntry struct {
	Month  string  `json:"month"`
	Amount float64 `json:"amount"`
}

// CategoryEntry 分类汇总条目
type CategoryEntry struct {
	Category   string  `json:"category"`
	Amount     float64 `json:"amount"`
	Percentage float64 `json:"percentage"`
	Count      int     `json:"count"`
}

// GetMonthlySummary 获取每月支出汇总
func (Transaction) GetMonthlySummary(userID uint, year string) ([]MonthlyEntry, error) {
	var entries []MonthlyEntry

	query := DB.Model(&Transaction{}).
		Where("user_id = ? AND type = 'expense'", userID)
	if year != "" {
		query = query.Where("LEFT(time, 4) = ?", year)
	}

	err := query.Select("LEFT(time, 7) as month, SUM(amount) as amount").
		Group("LEFT(time, 7)").
		Order("month ASC").
		Find(&entries).Error
	return entries, err
}

// Truncate 清空交易表（TRUNCATE TABLE）
func (Transaction) Truncate() error {
	return DB.Exec("TRUNCATE TABLE `transaction`").Error
}

// GetCategorySummary 获取分类支出汇总
func (Transaction) GetCategorySummary(userID uint, year, month string) ([]CategoryEntry, error) {
	var entries []CategoryEntry

	query := DB.Model(&Transaction{}).
		Where("user_id = ? AND type = 'expense'", userID)
	if year != "" {
		query = query.Where("LEFT(time, 4) = ?", year)
	}
	if month != "" {
		query = query.Where("LEFT(time, 7) = ?", year+"-"+month)
	}

	err := query.Select("category, SUM(amount) as amount, COUNT(*) as count").
		Group("category").
		Order("amount DESC").
		Find(&entries).Error

	if len(entries) > 0 {
		var total float64
		for i := range entries {
			total += entries[i].Amount
		}
		for i := range entries {
			entries[i].Percentage = entries[i].Amount / total * 100
		}
	}

	return entries, err
}

// DeleteByUserID 删除用户某月或所有交易记录
func (Transaction) DeleteByUserID(userID uint, year, month string) error {
	query := DB.Where("user_id = ?", userID)
	if year != "" {
		query = query.Where("LEFT(time, 4) = ?", year)
	}
	if month != "" {
		query = query.Where("LEFT(time, 7) = ?", year+"-"+month)
	}
	return query.Delete(&Transaction{}).Error
}
