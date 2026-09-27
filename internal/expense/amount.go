package expense

import (
	"errors"
	"math"
	"strconv"
	"strings"
)

var (
	ErrInvalidAmount    = errors.New("invalid amount")
	ErrCategoryNotFound = errors.New("category not found")
)

func ParseAmountMinor(raw string) (int64, error) {
	raw = strings.ReplaceAll(strings.TrimSpace(raw), ",", ".")
	parts := strings.Split(raw, ".")

	if len(parts) > 2 {
		return 0, ErrInvalidAmount
	}

	rubles, err := strconv.ParseUint(parts[0], 10, 64)

	if err != nil {
		return 0, ErrInvalidAmount
	}
	var kopecks uint64
	if len(parts) == 2 {
		if len(parts[1]) == 0 || len(parts[1]) > 2 {
			return 0, ErrInvalidAmount
		}

		kopecks, err = strconv.ParseUint(parts[1], 10, 64)
		if err != nil {
			return 0, ErrInvalidAmount
		}
		if len(parts[1]) == 1 {
			kopecks *= 10
		}
	}

	if rubles > (uint64(math.MaxInt64)-kopecks)/100 {
		return 0, ErrInvalidAmount
	}

	amountMinor := int64(rubles*100 + kopecks)
	if amountMinor == 0 {
		return 0, ErrInvalidAmount
	}
	return amountMinor, nil
}
