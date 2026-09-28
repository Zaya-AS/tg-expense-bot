package report

import (
	"bytes"
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/Zaya-AS/tg-expense-bot/internal/expense"
	"github.com/xuri/excelize/v2"
)

func TestBuildExpensesWorkbookGroupsMonthsInUserTimezone(t *testing.T) {
	location, err := time.LoadLocation("Asia/Yekaterinburg")
	if err != nil {
		t.Fatal(err)
	}
	records := []expense.Record{
		{Category: "еда", AmountMinor: 12345, Currency: "RUB", Description: "Обед", SpentAt: time.Date(2026, 8, 31, 18, 30, 0, 0, time.UTC)},
		{Category: "продукты", AmountMinor: 5010, Currency: "RUB", Description: "Молоко", SpentAt: time.Date(2026, 8, 31, 20, 0, 0, 0, time.UTC)},
		{Category: "дом", AmountMinor: 2500, Currency: "RUB", Description: "=HYPERLINK(\"https://example.com\")", SpentAt: time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)},
	}
	data, err := BuildExpensesWorkbook(records, location)
	if err != nil {
		t.Fatal(err)
	}
	file, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	sheets := file.GetSheetList()
	if len(sheets) != 3 || sheets[0] != "Итоги" || sheets[1] != "2026-09" || sheets[2] != "2026-08" {
		t.Fatalf("sheets = %v", sheets)
	}
	for _, tc := range []struct {
		sheet, cell, want string
	}{
		{"2026-09", "A1", "Цена, ₽"},
		{"2026-09", "B1", "Категория"},
		{"2026-09", "C1", "Что куплено"},
		{"2026-09", "C2", "Молоко"},
		{"2026-09", "C3", "=HYPERLINK(\"https://example.com\")"},
		{"2026-09", "B4", "Итого за месяц"},
		{"2026-08", "C2", "Обед"},
		{"Итоги", "A2", "2026-09"},
		{"Итоги", "A3", "2026-08"},
		{"Итоги", "A4", "Всего"},
	} {
		got, err := file.GetCellValue(tc.sheet, tc.cell)
		if err != nil || got != tc.want {
			t.Fatalf("%s!%s = %q, %v; want %q", tc.sheet, tc.cell, got, err, tc.want)
		}
	}
	for _, tc := range []struct {
		sheet, cell string
		want        float64
	}{
		{"2026-09", "A2", 50.10},
		{"2026-09", "A4", 75.10},
		{"2026-08", "A3", 123.45},
		{"Итоги", "B2", 75.10},
		{"Итоги", "B3", 123.45},
		{"Итоги", "B4", 198.55},
	} {
		value, err := file.GetCellValue(tc.sheet, tc.cell, excelize.Options{RawCellValue: true})
		if err != nil {
			t.Fatal(err)
		}
		got, err := strconv.ParseFloat(value, 64)
		if err != nil || math.Abs(got-tc.want) > 0.000001 {
			t.Fatalf("%s!%s = %q, %v; want %.2f", tc.sheet, tc.cell, value, err, tc.want)
		}
	}
	formula, err := file.GetCellFormula("2026-09", "C3")
	if err != nil || formula != "" {
		t.Fatalf("description was interpreted as formula: %q, %v", formula, err)
	}
}
