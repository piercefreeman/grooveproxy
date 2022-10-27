package main

import (
	"bytes"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"sync"
	"testing"
)

func TestBlockConcurrent(t *testing.T) {
	/*
	 * Test if we run X concurrent requests for the same URL and then satisfy the first
	 * we will store the same value for the secondary ones.
	 */
	cache := NewCache(100)
	cache.mode = CacheModeAggressive

	requestWaiter := &sync.WaitGroup{}

	// If the page actually fetches "outbound" there should be multiple values
	pageResponses := []string{"1", "2", "3", "4", "5"}
	sharedURL := "http://example.com"

	// Response payloads
	foundValuesLock := &sync.RWMutex{}
	foundValues := make([]*http.Response, 0)

	for _, value := range pageResponses {
		requestWaiter.Add(1)

		go func(value string) {
			defer requestWaiter.Done()

			request := &http.Request{
				Method: "GET",
				URL:    &url.URL{Path: sharedURL},
			}

			_, cachedResponse := cache.handleRequest(request)

			if cachedResponse != nil {
				// Cache hit, nothing to do besides log the overall value
				foundValuesLock.Lock()
				defer foundValuesLock.Unlock()
				foundValues = append(foundValues, cachedResponse)
				return
			}

			// If we got here we should execute the outward "fetch", ie. create
			// a response payload with our mocked value
			response := &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString(value)),
				Request:    request,
			}

			finalResponse := cache.handleResponse(response, request, nil)
			foundValuesLock.Lock()
			defer foundValuesLock.Unlock()
			foundValues = append(foundValues, finalResponse)
		}(value)
	}

	requestWaiter.Wait()

	uniqueResponses := make([]string, 0)
	for _, response := range foundValues {
		body, _ := ioutil.ReadAll(response.Body)
		if !contains(uniqueResponses, string(body)) {
			uniqueResponses = append(uniqueResponses, string(body))
		}
	}

	// Ensure we only have one value and blocking worked successfully
	if len(uniqueResponses) != 1 {
		log.Fatalf("Expected only one value, got %d", len(uniqueResponses))
	}
}
