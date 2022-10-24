package cache

import "container/list"

type CacheMetadata struct {
	/*
	 * Store properties about the cache entries - these are persisted to disk
	 * separately from the objects themselves
	 */
	Key  string `json:"key"`
	Size int64  `json:"size"`
}

type LRUCache struct {
	/*
	 * A common API for cache backends that allows for callbacks and invalidation logic
	 */
	backingCache any

	// Linked-list, where freshest cache items are at the front and oldest (and closest to invalidation)
	// are at the end.
	// Instances of CacheMetadata
	orderedCacheKeys *list.List
	keyToElement     map[string]*list.Element

	// Bytes
	// If `maxSize` is -1, will grow without bound
	// If instead maxSize is 0, won't populate the LRU cache
	currentSize int64
	maxSize     int64

	/*
	 * User-override functions - each should be implemented for a full LRU implementation
	 */
	SetValueCallback func(cache *LRUCache, key string, value *[]byte) error
	GetValueCallback func(cache *LRUCache, key string) (*[]byte, error)
	HasValueCallback func(cache *LRUCache, key string) bool
	// Explicit deletion or because of forced cache size validation
	DeleteKeyCallback func(cache *LRUCache, key string)
	DeleteAllCallback func(cache *LRUCache)
}

func NewLRUCache(backingCache any, currentSize int64, maxSize int64) *LRUCache {
	return &LRUCache{
		backingCache:     backingCache,
		orderedCacheKeys: list.New(),
		keyToElement:     make(map[string]*list.Element),
		currentSize:      currentSize,
		maxSize:          maxSize,
	}
}

func (cache *LRUCache) Get(key string) (*[]byte, error) {
	// If so, move it to the front of the list
	metadata, ok := cache.keyToElement[key]
	if ok {
		cache.orderedCacheKeys.MoveToFront(metadata)
	}

	return cache.GetValueCallback(cache, key)
}

func (cache *LRUCache) Set(key string, value *[]byte) error {
	// Determine if this object can fit in the cache
	metadata := CacheMetadata{
		Key:  key,
		Size: int64(len(*value)),
	}

	// If we are already in the cache, remove the old entry
	// This will free up the metadata and other attributes before we add a new one
	// We shouldn't clear out other parts of the cache if we
	if cache.Has(key) {
		cache.Delete(key)
	}

	// Check if either the memory or disk cache would become full with this object and purge accordingly
	// Purge the oldest entries until we have enough space
	if cache.maxSize > -1 {
		for cache.currentSize+metadata.Size > cache.maxSize {
			oldestElement := cache.orderedCacheKeys.Back()
			if oldestElement == nil {
				break
			}
			oldestKey := oldestElement.Value.(CacheMetadata).Key
			cache.Delete(oldestKey)
		}
	}

	// It's possible we would still exhaust the memory space, in which case we shouldn't
	// store it at all. Ensure we stay under the threshold.
	if cache.currentSize+metadata.Size <= cache.maxSize {
		err := cache.SetValueCallback(cache, key, value)
		if err != nil {
			return err
		}
		cache.currentSize += metadata.Size
		cache.keyToElement[key] = cache.orderedCacheKeys.PushFront(metadata)
	}

	return nil
}

func (cache *LRUCache) Has(key string) bool {
	return cache.HasValueCallback(cache, key)
}

func (cache *LRUCache) Delete(key string) {
	// Housekeeping for the metadata
	metadata, ok := cache.keyToElement[key]
	if ok {
		cache.orderedCacheKeys.Remove(metadata)
		cache.currentSize -= metadata.Value.(CacheMetadata).Size
		delete(cache.keyToElement, key)
	}

	// Perform the actual deletion
	cache.DeleteKeyCallback(cache, key)
}

func (cache *LRUCache) DeleteAll() {
	cache.DeleteAllCallback(cache)
}
