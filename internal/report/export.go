package report

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/Zaya-AS/tg-expense-bot/internal/expense"
	"github.com/xuri/excelize/v2"
)

const maxExcelRows = 1048576

// BuildExpensesWorkbook creates a summary and one sheet per calendar month in
// the user's timezone. Monetary totals are accumulated in kopecks to avoid
// floating point errors; only the finished cell values are converted to rubles.
func BuildExpensesWorkbook(records []expense.Record, location *time.Location) ([]byte, error) {
	if location == nil {
		return nil, fmt.Errorf("expense export timezone is missing")
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("expense export has no records")
	}
	byMonth := make(map[string][]expense.Record)
	totals := make(map[string]int64)
	var overall int64
	for _, record := range records {
		if record.Currency != "RUB" || record.AmountMinor <= 0 {
			return nil, fmt.Errorf("unsupported expense currency or amount")
		}
		month := record.SpentAt.In(location).Format("2006-01")
		if len(byMonth[month]) >= maxExcelRows-2 {
			return nil, fmt.Errorf("too many expenses in month %s for Excel", month)
		}
		if totals[month] > math.MaxInt64-record.AmountMinor || overall > math.MaxInt64-record.AmountMinor {
			return nil, fmt.Errorf("expense export total overflow")
		}
		byMonth[month] = append(byMonth[month], record)
		totals[month] += record.AmountMinor
		overall += record.AmountMinor
	}
	months := make([]string, 0, len(byMonth))
	for month := range byMonth {
		months = append(months, month)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(months)))

	file := excelize.NewFile()
	defer file.Close()
	if err := file.SetSheetName("Sheet1", "Итоги"); err != nil {
		return nil, fmt.Errorf("name summary sheet: %w", err)
	}
	headerStyle, err := file.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"24527A"}, Pattern: 1},
	})
	if err != nil {
		return nil, fmt.Errorf("create export header style: %w", err)
	}
	totalStyle, err := file.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"E7F0F7"}, Pattern: 1},
	})
	if err != nil {
		return nil, fmt.Errorf("create export total style: %w", err)
	}
	amountStyle, err := file.NewStyle(&excelize.Style{NumFmt: 2})
	if err != nil {
		return nil, fmt.Errorf("create export amount style: %w", err)
	}
	amountTotalStyle, err := file.NewStyle(&excelize.Style{
		NumFmt: 2,
		Font:   &excelize.Font{Bold: true},
		Fill:   excelize.Fill{Type: "pattern", Color: []string{"E7F0F7"}, Pattern: 1},
	})
	if err != nil {
		return nil, fmt.Errorf("create export total amount style: %w", err)
	}
	if err := file.SetCellStr("Итоги", "A1", "Месяц"); err != nil {
		return nil, err
	}
	if err := file.SetCellStr("Итоги", "B1", "Сумма, ₽"); err != nil {
		return nil, err
	}
	if err := file.SetCellStyle("Итоги", "A1", "B1", headerStyle); err != nil {
		return nil, err
	}
	if err := file.SetColWidth("Итоги", "A", "A", 18); err != nil {
		return nil, err
	}
	if err := file.SetColWidth("Итоги", "B", "B", 20); err != nil {
		return nil, err
	}

	for index, month := range months {
		if _, err := file.NewSheet(month); err != nil {
			return nil, fmt.Errorf("create month sheet %s: %w", month, err)
		}
		if err := writeMonthSheet(file, month, byMonth[month], totals[month], headerStyle, totalStyle, amountStyle, amountTotalStyle); err != nil {
			return nil, err
		}
		row := index + 2
		if err := file.SetCellStr("Итоги", fmt.Sprintf("A%d", row), month); err != nil {
			return nil, err
		}
		if err := file.SetCellValue("Итоги", fmt.Sprintf("B%d", row), float64(totals[month])/100); err != nil {
			return nil, err
		}
	}
	lastRow := len(months) + 2
	if err := file.SetCellStr("Итоги", fmt.Sprintf("A%d", lastRow), "Всего"); err != nil {
		return nil, err
	}
	if err := file.SetCellValue("Итоги", fmt.Sprintf("B%d", lastRow), float64(overall)/100); err != nil {
		return nil, err
	}
	if err := file.SetCellStyle("Итоги", fmt.Sprintf("A%d", lastRow), fmt.Sprintf("B%d", lastRow), totalStyle); err != nil {
		return nil, err
	}
	if err := file.SetCellStyle("Итоги", "B2", fmt.Sprintf("B%d", lastRow-1), amountStyle); err != nil {
		return nil, err
	}
	if err := file.SetCellStyle("Итоги", fmt.Sprintf("B%d", lastRow), fmt.Sprintf("B%d", lastRow), amountTotalStyle); err != nil {
		return nil, err
	}
	file.SetActiveSheet(0)
	buffer, err := file.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("write expense workbook: %w", err)
	}
	return buffer.Bytes(), nil
}

func writeMonthSheet(file *excelize.File, month string, records []expense.Record, totalMinor int64, headerStyle, totalStyle, amountStyle, amountTotalStyle int) error {
	for col, title := range []string{"Цена, ₽", "Категория", "Что куплено"} {
		cell := string(rune('A'+col)) + "1"
		if err := file.SetCellStr(month, cell, title); err != nil {
			return err
		}
	}
	if err := file.SetCellStyle(month, "A1", "C1", headerStyle); err != nil {
		return err
	}
	for _, width := range []struct {
		column string
		value  float64
	}{{"A", 16}, {"B", 24}, {"C", 50}} {
		if err := file.SetColWidth(month, width.column, width.column, width.value); err != nil {
			return err
		}
	}
	for index, record := range records {
		row := index + 2
		if err := file.SetCellValue(month, fmt.Sprintf("A%d", row), float64(record.AmountMinor)/100); err != nil {
			return err
		}
		if err := file.SetCellStr(month, fmt.Sprintf("B%d", row), record.Category); err != nil {
			return err
		}
		if err := file.SetCellStr(month, fmt.Sprintf("C%d", row), record.Description); err != nil {
			return err
		}
	}
	totalRow := len(records) + 2
	if err := file.SetCellValue(month, fmt.Sprintf("A%d", totalRow), float64(totalMinor)/100); err != nil {
		return err
	}
	if err := file.SetCellStr(month, fmt.Sprintf("B%d", totalRow), "Итого за месяц"); err != nil {
		return err
	}
	if err := file.SetCellStyle(month, fmt.Sprintf("A%d", totalRow), fmt.Sprintf("C%d", totalRow), totalStyle); err != nil {
		return err
	}
	if err := file.SetCellStyle(month, "A2", fmt.Sprintf("A%d", totalRow-1), amountStyle); err != nil {
		return err
	}
	return file.SetCellStyle(month, fmt.Sprintf("A%d", totalRow), fmt.Sprintf("A%d", totalRow), amountTotalStyle)
}
