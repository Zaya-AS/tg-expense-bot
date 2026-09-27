package expense

import (
	"errors"
	"testing"
)

func TestParseAmountMinor(t *testing.T) {
	tests := []struct {
		input string
		want  int64
		valid bool
	}{
		{"250", 25000, true},
		{"250.5", 25050, true},
		{"250,05", 25005, true},
		{" 0.01 ", 1, true},
		{"92233720368547758.07", 9223372036854775807, true},
		{"0", 0, false},
		{"-1", 0, false},
		{"1.234", 0, false},
		{"1.", 0, false},
		{"92233720368547758.08", 0, false},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			got, err := ParseAmountMinor(test.input)
			if test.valid {
				if err != nil || got != test.want {
					t.Fatalf("ParseAmountMinor(%q) = %d, %v; want %d", test.input, got, err, test.want)
				}
			} else if !errors.Is(err, ErrInvalidAmount) {
				t.Fatalf("ParseAmountMinor(%q) error = %v; want ErrInvalidAmount", test.input, err)
			}
		})
	}
}
