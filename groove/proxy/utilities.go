package main

import (
	"net/http"
)

func reverseSlice[T any](s []T) {
	// https://github.com/golang/go/wiki/SliceTricks#reversing
	// https://eli.thegreenplace.net/2021/generic-functions-on-slices-with-go-type-parameters/
	for left, right := 0, len(s)-1; left < right; left, right = left+1, right-1 {
		s[left], s[right] = s[right], s[left]
	}
}

func filterSlice[T any](s []T, f func(T) bool) []T {
	filtered := make([]T, 0)

	for _, value := range s {
		if f(value) {
			filtered = append(filtered, value)
		}
	}

	return filtered
}

func contains[T comparable](s []T, e T) bool {
	for _, v := range s {
		if v == e {
			return true
		}
	}
	return false
}

func getRedirectHistory(response *http.Response) ([]*http.Request, []*http.Response) {
	// The eventually resolved response payload carries alongside all of the request
	// history - this function reassembles it
	requestHistory := make([]*http.Request, 0)
	responseHistory := make([]*http.Response, 0)

	for response != nil {
		request := response.Request
		requestHistory = append(requestHistory, request)
		responseHistory = append(responseHistory, response)
		response = request.Response
	}

	// The response order is actually reversed from what we expect
	// The last request that eventually made the response comes first in the slice
	reverseSlice(requestHistory)
	reverseSlice(responseHistory)

	return requestHistory, responseHistory
}
