package api

import (
	"slices"
)

func GetRatio(size Size) Size {
	if size.W <= 0 || size.H <= 0 {
		return Size{}
	}
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

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}
