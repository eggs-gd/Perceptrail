package duration

import "testing"

func TestParse(t *testing.T) {
	for in, want := range map[string]float64{
		"24.4":                    24.4,
		"9.80833333333333":        9.80833333333333,
		" 83 ":                    83,
		"":                        0,
		"-1":                      0,
		"24.40 s (not -n output)": 0,
	} {
		if got := parse(in); got != want {
			t.Errorf("parse(%q) = %v, want %v", in, got, want)
		}
	}
}
