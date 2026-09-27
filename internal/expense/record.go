package expense

import "time"

type Record struct {
	ID          int64
	Category    string
	AmountMinor int64
	Currency    string
	Description string
	SpentAt     time.Time
}
