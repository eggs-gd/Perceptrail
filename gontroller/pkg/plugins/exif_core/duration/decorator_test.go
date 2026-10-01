package duration

import "testing"

func TestParse(t *testing.T) {
	for in, want := range map[string]float64{
		"24.40 s":          24.4,
		"0:01:23":          83,
		"1:02:03.5":        3723.5,
		"0.50 s (approx)":  0.5,
		"24.4":             24.4,
		"":                 0,
		"0 s":              0,
		"Unknown duration": 0,
	} {
		if got := parse(in); got != want {
			t.Errorf("parse(%q) = %v, want %v", in, got, want)
		}
	}
}
