package report

import (
	"testing"
	"time"
)

func TestMonthBoundsInUserTimezone(t *testing.T) {
	tests := []struct {
		zone      string
		now       time.Time
		wantStart time.Time
		wantEnd   time.Time
	}{
		{
			zone:      "Asia/Yekaterinburg",
			now:       time.Date(2026, 8, 31, 20, 0, 0, 0, time.UTC),
			wantStart: time.Date(2026, 8, 31, 19, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2026, 9, 30, 19, 0, 0, 0, time.UTC),
		},
		{
			zone:      "America/New_York",
			now:       time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC),
			wantStart: time.Date(2026, 3, 1, 5, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2026, 4, 1, 4, 0, 0, 0, time.UTC),
		},
	}
	for _, test := range tests {
		t.Run(test.zone, func(t *testing.T) {
			location, err := time.LoadLocation(test.zone)
			if err != nil {
				t.Fatal(err)
			}
			start, end := MonthBounds(test.now, location)
			if !start.Equal(test.wantStart) || !end.Equal(test.wantEnd) {
				t.Fatalf("bounds = %s to %s; want %s to %s", start, end, test.wantStart, test.wantEnd)
			}
		})
	}
}
