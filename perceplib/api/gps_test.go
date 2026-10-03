package api

import (
	"slices"
	"testing"
)

type askedExif map[string]bool

func (a askedExif) GetExif(key string) string { a[key] = true; return "" }

// Coordinates reads only CoordinateTags: a perceptor declaring them gets them all
func TestCoordinatesReadsDeclaredTags(t *testing.T) {
	asked := askedExif{}
	Coordinates(asked)
	for tag := range asked {
		if !slices.Contains(CoordinateTags, tag) {
			t.Errorf("Coordinates reads %s, not in CoordinateTags", tag)
		}
	}
}
