package api

import (
	"slices"
)

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func GetRatio(size Size) Size {
	gcd := gcd(size.W, size.H)
	return Size{
		W: size.W / gcd,
		H: size.H / gcd,
	}
}

func AppendUniq[T comparable](slice []T, elem T) []T {
	if !slices.Contains(slice, elem) {
		slice = append(slice, elem)
	}

	return slice
}
