package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
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

func containsInt(s []int, search int) bool {
	for _, value := range s {
		if value == search {
			return true
		}
	}

	return false
}

func containsString(s []string, search string) bool {
	for _, value := range s {
		if value == search {
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

func objectToBytes(obj any) ([]byte, error) {
	var writeBuffer bytes.Buffer
	encoder := gob.NewEncoder(&writeBuffer)
	err := encoder.Encode(obj)
	if err != nil {
		log.Println(fmt.Errorf("Failed to encode cache entry %w", err))
		return nil, err
	}

	return writeBuffer.Bytes(), nil
}

func objectFromBytes(readBuffer []byte, obj any) error {
	decoder := gob.NewDecoder(bytes.NewReader(readBuffer))
	err := decoder.Decode(obj)
	if err != nil {
		log.Println(fmt.Errorf("Failed to decode cache entry %w", err))
		return err
	}
	return nil
}
