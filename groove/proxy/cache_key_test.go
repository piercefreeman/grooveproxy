package main

import (
	"net/http"
	"net/url"
	"testing"
)

func TestCacheKeyParameters(t *testing.T) {
	var tests = []struct {
		url1, url2      string
		desiredEquality bool
	}{
		// Identity
		{"http://example.com?test1=a&test2=b", "http://example.com?test1=a&test2=b", true},
		// Switching parameter order
		{"http://example.com?test1=a&test2=b", "http://example.com?test2=b&test1=a", true},
		// Different parameters
		{"http://example.com?test1=a&test2=b", "http://example.com?&test1=a&test3=b", false},
		// Different protocols
		{"https://example.com", "http://example.com", true},
		// Different domain suffix same host name
		{"http://example.com", "http://example.net", false},
	}

	for _, tt := range tests {
		url1, _ := url.Parse(tt.url1)
		url2, _ := url.Parse(tt.url2)
		cacheEquality := getCacheKey(&http.Request{URL: url1}) == getCacheKey(&http.Request{URL: url2})

		if cacheEquality != tt.desiredEquality {
			t.Fatalf("CacheKey - `%s` `%s` (actual: %v, expected: %v)", url1, url2, cacheEquality, tt.desiredEquality)
		}
	}
}
