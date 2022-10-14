package main

func reverseSlice[T any](s []T) {
	// https://github.com/golang/go/wiki/SliceTricks#reversing
	// https://eli.thegreenplace.net/2021/generic-functions-on-slices-with-go-type-parameters/
	for left, right := 0, len(s)-1; left < right; left, right = left+1, right-1 {
		s[left], s[right] = s[right], s[left]
	}
}
