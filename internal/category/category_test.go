package category

import "testing"

func TestNormalize(t *testing.T) {
	if got := Normalize("  ЕдА  "); got != "еда" {
		t.Fatalf("Normalize = %q; want еда", got)
	}
	for _, name := range Defaults {
		if Normalize(name) != name {
			t.Fatalf("default category %q is not normalized", name)
		}
	}
}
