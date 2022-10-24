package cache

import (
	"io/ioutil"
	"testing"
)

func TestSaveReadIndex(t *testing.T) {
	cacheDirectory, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("Error creating temp dir: %s", err)
	}

	invalidator := NewCacheInvalidator(cacheDirectory, 10, 10, 1)
	invalidator.Set("testKey", &TestSimpleObject{"testValue"})

	// Ensure it saved automatically - should have spanwed a goroutine
	invalidator.saveWaiter.Wait()

	// Attempt to read the index file
	fileContents, _, err := invalidator.readIndex()

	if err != nil {
		t.Fatalf("Error reading index: %s", err)
	}

	if len(fileContents) != 1 {
		t.Fatalf("Index should have one entry")
	}

	if fileContents[0].Key != "testKey" {
		t.Fatalf("Index should have testKey (actual: %s)", fileContents[0].Key)
	}

	if fileContents[0].Size == 0 {
		t.Fatalf("Index should have non-zero size (actual: %d)", fileContents[0].Size)
	}

	// Determine if depenent modules can also load this file
	invalidator.memoryCache = invalidator.buildMemoryCache(1)
	invalidator.diskCache = invalidator.buildDiskCache(1, cacheDirectory)

	// Then dump to disk one more time to make sure we can save the loaded representation
	invalidator.writeIndex()
	invalidator.saveWaiter.Wait()
}
