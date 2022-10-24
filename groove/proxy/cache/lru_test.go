package cache

import (
	"io/ioutil"
	"log"
	"path/filepath"
	"testing"
)

func TestCacheStorage(t *testing.T) {
	invalidator := &CacheInvalidator{}

	cacheDirectory, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("Error creating temp dir: %s", err)
	}

	var tests = []struct {
		lruCache *LRUCache
		label    string
	}{
		{invalidator.buildMemoryCache(1), "memory"},
		{invalidator.buildDiskCache(1, cacheDirectory), "disk"},
	}

	testKey := "testKey"
	testObject := []byte{97}

	for _, test := range tests {
		log.Printf("TestCacheStorage: Testing cache: %s", test.label)

		test.lruCache.Set(testKey, &testObject)
		objectRecovered, err := test.lruCache.Get(testKey)

		if err != nil {
			t.Fatalf("Error getting object: %s", err)
		}

		if (*objectRecovered)[0] != 97 {
			t.Fatalf("Recovered object does not match original")
		}
	}
}

func TestInvalidateCache(t *testing.T) {
	invalidator := &CacheInvalidator{}

	cacheDirectory, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("Error creating temp dir: %s", err)
	}

	var tests = []struct {
		lruCache *LRUCache
		label    string
	}{
		// Set max size equal to 0, this should mean nothing is cached
		{invalidator.buildMemoryCache(0), "memory"},
		{invalidator.buildDiskCache(0, cacheDirectory), "disk"},
	}

	testKey := "testKey"
	testObject := []byte{97}

	for _, test := range tests {
		log.Printf("TestInvalidateCache: Testing cache: %s", test.label)

		test.lruCache.Set(testKey, &testObject)
		_, err := test.lruCache.Get(testKey)

		if err == nil {
			t.Fatalf("Object should not have been saved: %s", err)
		}
	}
}

func TestLimitedCacheSize(t *testing.T) {
	invalidator := &CacheInvalidator{}

	cacheDirectory, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("Error creating temp dir: %s", err)
	}

	var tests = []struct {
		lruCache *LRUCache
		label    string
	}{
		// Allow one object
		{invalidator.buildMemoryCache(1), "memory"},
		{invalidator.buildDiskCache(1, cacheDirectory), "disk"},
	}

	testKey1 := "testKey-1"
	testKey2 := "testKey-2"
	testObject1 := []byte{97}
	testObject2 := []byte{98}

	for _, test := range tests {
		log.Printf("TestLimitedCacheSize: Testing cache: %s", test.label)

		test.lruCache.Set(testKey1, &testObject1)
		test.lruCache.Set(testKey2, &testObject2)

		_, err := test.lruCache.Get(testKey1)
		if err == nil {
			t.Fatalf("Key should have been invalidated: %s", err)
		}

		_, err = test.lruCache.Get(testKey2)
		if err != nil {
			t.Fatalf("Key should have saved: %s", err)
		}
	}
}

func TestDiskWrite(t *testing.T) {
	invalidator := &CacheInvalidator{}

	cacheDirectory, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("Error creating temp dir: %s", err)
	}

	diskCache := invalidator.buildDiskCache(1, cacheDirectory)

	testKey1 := "testKey-1"
	testObject1 := []byte{97}

	diskCache.Set(testKey1, &testObject1)

	expectedLocation := blockTransform(testKey1)

	// Check the disk location for the file contents
	pathArgs := []string{cacheDirectory}
	pathArgs = append(pathArgs, expectedLocation...)
	pathArgs = append(pathArgs, testKey1)

	// Check the file exists
	_, err = ioutil.ReadFile(filepath.Join(pathArgs...))
	if err != nil {
		t.Fatalf("Error reading file: %s", err)
	}
}
