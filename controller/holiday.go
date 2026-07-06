package controller

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"go-alpha/response"
)

type HolidayItem struct {
	Date  string `json:"date"`
	Label string `json:"label"`
}

// 目前先内置 2026 年节假日。后续如需扩展，可直接补充年份表。
var holidayByYear = map[int][]HolidayItem{
	2026: {
		{Date: "2026-01-01", Label: "元旦"},
		{Date: "2026-01-02", Label: "元旦"},
		{Date: "2026-01-03", Label: "元旦"},
		{Date: "2026-02-15", Label: "春节"},
		{Date: "2026-02-16", Label: "春节"},
		{Date: "2026-02-17", Label: "春节"},
		{Date: "2026-02-18", Label: "春节"},
		{Date: "2026-02-19", Label: "春节"},
		{Date: "2026-02-20", Label: "春节"},
		{Date: "2026-02-21", Label: "春节"},
		{Date: "2026-02-22", Label: "春节"},
		{Date: "2026-02-23", Label: "春节"},
		{Date: "2026-04-04", Label: "清明节"},
		{Date: "2026-04-05", Label: "清明节"},
		{Date: "2026-04-06", Label: "清明节"},
		{Date: "2026-05-01", Label: "劳动节"},
		{Date: "2026-05-02", Label: "劳动节"},
		{Date: "2026-05-03", Label: "劳动节"},
		{Date: "2026-05-04", Label: "劳动节"},
		{Date: "2026-05-05", Label: "劳动节"},
		{Date: "2026-06-19", Label: "端午节"},
		{Date: "2026-06-20", Label: "端午节"},
		{Date: "2026-06-21", Label: "端午节"},
		{Date: "2026-09-25", Label: "中秋节"},
		{Date: "2026-09-26", Label: "中秋节"},
		{Date: "2026-09-27", Label: "中秋节"},
		{Date: "2026-10-01", Label: "国庆节"},
		{Date: "2026-10-02", Label: "国庆节"},
		{Date: "2026-10-03", Label: "国庆节"},
		{Date: "2026-10-04", Label: "国庆节"},
		{Date: "2026-10-05", Label: "国庆节"},
		{Date: "2026-10-06", Label: "国庆节"},
		{Date: "2026-10-07", Label: "国庆节"},
	},
}

func GetHoliday(c *gin.Context) {
	year := c.DefaultQuery("year", "2026")
	y, err := strconv.Atoi(year)
	if err != nil || y < 2000 || y > 2100 {
		response.Failed("year 参数不合法", c)
		return
	}

	items, ok := holidayByYear[y]
	if !ok {
		response.Success("ok", []HolidayItem{}, c)
		return
	}

	response.Success("ok", gin.H{
		"year":  y,
		"items": items,
	}, c)
}
