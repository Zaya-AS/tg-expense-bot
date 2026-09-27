package report

import "time"

type CategoryTotal struct {
	Name        string
	AmountMinor int64
}

func MonthBounds(now time.Time, location *time.Location) (time.Time, time.Time) {
	local := now.In(location)
	start := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, location)
	return start, start.AddDate(0, 1, 0)
}
