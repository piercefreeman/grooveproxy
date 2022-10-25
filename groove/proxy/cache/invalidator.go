package cache

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/peterbourgon/diskv/v3"
)

type MemoryCache map[string]*[]byte

type CacheInvalidator struct {
	/*
	 * Light wapper around a disk cache to provide automatic cache invalidation
	 * if the disk cache expands past a certain size
	 */
	// Path to the disk index
	indexPath      string
	indexWriteLock sync.Mutex

	diskCache *LRUCache

	// We re-implement the in-memory cache even though diskv has an alternative because we need
	// to handle custom dequeuing logic.
	memoryCacheLock sync.RWMutex
	memoryCache     *LRUCache

	// Amount of edits before the meta-index is saved to disk, used to minimize the amount of full
	// disk writes
	saveInterval     int
	operationCounter int
	saveWaiter       *sync.WaitGroup
}

const transformBlockSize = 2 // Grouping of chars per directory depth

func blockTransform(s string) []string {
	var (
		sliceSize = len(s) / transformBlockSize
		pathSlice = make([]string, sliceSize)
	)
	for i := 0; i < sliceSize; i++ {
		from, to := i*transformBlockSize, (i*transformBlockSize)+transformBlockSize
		pathSlice[i] = s[from:to]
	}
	return pathSlice
}

func NewCacheInvalidator(
	diskCacheLocation string,
	maxMemorySizeMB int64,
	maxDiskSizeMB int64,
	saveInterval int,
) *CacheInvalidator {
	invalidator := &CacheInvalidator{
		indexPath:       filepath.Join(diskCacheLocation, "index.json"),
		memoryCacheLock: sync.RWMutex{},
		saveInterval:    saveInterval,
		saveWaiter:      &sync.WaitGroup{},
	}
	log.Printf("Cache path: %s", invalidator.indexPath)

	invalidator.diskCache = invalidator.buildDiskCache(1024*1024*maxDiskSizeMB, diskCacheLocation)
	invalidator.memoryCache = invalidator.buildMemoryCache(1024 * 1024 * maxMemorySizeMB)

	return invalidator
}

func (cache *CacheInvalidator) buildMemoryCache(maxMemorySize int64) *LRUCache {
	memoryCache := NewLRUCache(make(MemoryCache), 0, maxMemorySize)
	memoryCache.SetValueCallback = func(lru *LRUCache, key string, value *[]byte) error {
		cache.memoryCacheLock.Lock()
		lru.backingCache.(MemoryCache)[key] = value
		cache.memoryCacheLock.Unlock()
		return nil
	}
	memoryCache.GetValueCallback = func(lru *LRUCache, key string) (*[]byte, error) {
		cache.memoryCacheLock.RLock()
		defer cache.memoryCacheLock.RUnlock()
		val, ok := lru.backingCache.(MemoryCache)[key]
		if !ok {
			return nil, fmt.Errorf("Key %s not found in memory cache", key)
		}
		return val, nil
	}
	memoryCache.HasValueCallback = func(lru *LRUCache, key string) bool {
		cache.memoryCacheLock.RLock()
		defer cache.memoryCacheLock.RUnlock()
		_, ok := lru.backingCache.(MemoryCache)[key]
		return ok
	}
	memoryCache.DeleteKeyCallback = func(lru *LRUCache, key string) {
		cache.memoryCacheLock.Lock()
		delete(lru.backingCache.(MemoryCache), key)
		cache.memoryCacheLock.Unlock()
	}
	memoryCache.DeleteAllCallback = func(lru *LRUCache) {
		lru.backingCache = make(MemoryCache)
	}

	return memoryCache
}

func (cache *CacheInvalidator) buildDiskCache(maxDiskSize int64, diskCacheLocation string) *LRUCache {
	diskMetadata, totalDiskSize, _ := cache.readIndex()

	diskCache := NewLRUCache(
		diskv.New(diskv.Options{
			BasePath:     diskCacheLocation,
			Transform:    blockTransform,
			CacheSizeMax: 0,
		}),
		totalDiskSize,
		maxDiskSize,
	)
	diskCache.SetValueCallback = func(lru *LRUCache, key string, value *[]byte) error {
		return lru.backingCache.(*diskv.Diskv).Write(key, *value)
	}
	diskCache.GetValueCallback = func(lru *LRUCache, key string) (value *[]byte, err error) {
		encodedValue, err := lru.backingCache.(*diskv.Diskv).Read(key)
		if err != nil {
			return nil, err
		}
		return &encodedValue, nil
	}
	diskCache.HasValueCallback = func(lru *LRUCache, key string) bool {
		return lru.backingCache.(*diskv.Diskv).Has(key)
	}
	diskCache.DeleteKeyCallback = func(lru *LRUCache, key string) {
		lru.backingCache.(*diskv.Diskv).Erase(key)
	}
	diskCache.DeleteAllCallback = func(lru *LRUCache) {
		// Will erase everything from the given disk folder
		lru.backingCache.(*diskv.Diskv).EraseAll()
	}

	// Load in the saved metadatas - the index file stores these as we intend; most recently
	// accessed should be first
	for _, metadata := range diskMetadata {
		element := diskCache.orderedCacheKeys.PushBack(*metadata)
		diskCache.keyToElement[metadata.Key] = element
	}

	return diskCache
}

func (cache *CacheInvalidator) Get(key string, obj any) (err error) {
	var encodedValue *[]byte
	if cache.memoryCache.Has(key) {
		encodedValue, err = cache.memoryCache.Get(key)
		if err != nil {
			return err
		}
	}
	if cache.diskCache.Has(key) {
		encodedValue, err = cache.diskCache.Get(key)
		if err != nil {
			return err
		}
	}

	if encodedValue == nil {
		return fmt.Errorf("Key %s not found in cache", key)
	}

	return objectFromBytes(*encodedValue, obj)
}

func (cache *CacheInvalidator) Set(key string, value any) error {
	encodedValue, err := objectToBytes(value)
	if err != nil {
		return err
	}

	cache.memoryCache.Set(key, &encodedValue)
	cache.diskCache.Set(key, &encodedValue)

	// Write index owns the logic of whether now is a time to write to the disk index
	cache.writeIndex()

	return nil
}

func (cache *CacheInvalidator) Has(key string) bool {
	return cache.memoryCache.Has(key) || cache.diskCache.Has(key)
}

func (cache *CacheInvalidator) Delete(key string) {
	cache.memoryCache.Delete(key)
	cache.diskCache.Delete(key)

	// Write index owns the logic of whether now is a time to write to the disk index
	cache.writeIndex()
}

func (cache *CacheInvalidator) Clear() {
	cache.memoryCache.DeleteAll()
	cache.diskCache.DeleteAll()
}

func (cache *CacheInvalidator) readIndex() (orderedMetadata []*CacheMetadata, totalSize int64, err error) {
	// Read from the cached index file, if it exists
	// If not we are starting the disk store from scratch
	indexFile, err := os.Open(cache.indexPath)
	if err != nil {
		return nil, 0, err
	}

	defer indexFile.Close()

	var metadatas []*CacheMetadata
	byteValue, _ := ioutil.ReadAll(indexFile)
	json.Unmarshal(byteValue, &metadatas)

	// Determine how large the cached sizes are
	totalSize = int64(0)
	for _, metadata := range metadatas {
		totalSize += metadata.Size
	}

	return metadatas, totalSize, nil
}

func (cache *CacheInvalidator) writeIndex() {
	cache.operationCounter += 1

	if cache.operationCounter < cache.saveInterval {
		return
	}

	cache.operationCounter = 0

	cache.saveWaiter.Add(1)

	go func() {
		defer cache.saveWaiter.Done()

		cache.indexWriteLock.Lock()
		defer cache.indexWriteLock.Unlock()

		// Open the current file if possible
		indexFile, err := os.Open(cache.indexPath)
		if err != nil {
			// Create a new index file
			indexFile, err = os.Create(cache.indexPath)
			if err != nil {
				panic(err)
			}
		}

		defer indexFile.Close()

		// Write the current metadata to the file
		var metadata []CacheMetadata = nil

		for element := cache.diskCache.orderedCacheKeys.Front(); element != nil; element = element.Next() {
			metadata = append(metadata, element.Value.(CacheMetadata))
		}

		metadataBytes, err := json.Marshal(metadata)
		if err != nil {
			panic(err)
		}

		indexFile.Write(metadataBytes)
	}()
}
