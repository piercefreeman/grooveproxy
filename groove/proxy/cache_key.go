package main

import (
	"encoding/base64"
	"net/http"
	"net/url"
	"sort"
)

type FlatQuery struct {
	// By default url.Values() is a map of string to slice of strings
	// This isn't compatible with sorting so we need to flatten them
	key   string
	value string
}

type QueryArray []FlatQuery

func (s QueryArray) Len() int {
	return len(s)
}
func (s QueryArray) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s QueryArray) Less(i, j int) bool {
	queryPairI := s[i]
	queryPairJ := s[j]

	if queryPairI.key < queryPairJ.key {
		return true
	} else if queryPairI.key < queryPairJ.key {
		return false
	}

	return queryPairI.value < queryPairJ.value
}

func newQueryArray(values url.Values) QueryArray {
	/*
	 * Convert a url.Values into a QueryArray
	 */
	var queryArray QueryArray
	for key, keyValues := range values {
		for _, value := range keyValues {
			queryArray = append(queryArray, FlatQuery{key, value})
		}
	}
	return queryArray
}

func getCacheKey(request *http.Request) string {
	/*
	 * Generates a key based upon a request
	 * TODO: Add heuristics for stripping a URL of parameters that are lightly to change
	 * - Host
	 * - Path
	 * - Method
	 */
	urlBase := request.URL.Hostname() + request.URL.Path
	method := request.Method

	// Sort arguments to align them across cache requests with same parameters
	queryArray := newQueryArray(request.URL.Query())
	sort.Sort(queryArray)

	str := method + "-" + urlBase + "-"
	for _, queryPair := range queryArray {
		str += queryPair.key + "=" + queryPair.value + "&"
	}

	return base64.StdEncoding.EncodeToString([]byte(str))
}
