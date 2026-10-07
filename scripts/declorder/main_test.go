package main

import "testing"

// An ordered file passes (a public method on a private type counts as public);
// in a mixed one every declaration after a later group's is reported
func TestCheck(t *testing.T) {
	for file, want := range map[string]int{"testdata/ordered.go": 0, "testdata/mixed.go": 4} {
		got, err := check(file)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("%s: %d misplaced, want %d", file, got, want)
		}
	}
}
