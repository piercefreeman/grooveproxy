package cache

import (
	"container/list"
	"sync"
)

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
	linkedListLock   *sync.RWMutex
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
		linkedListLock:   &sync.RWMutex{},
	}
}

func (cache *LRUCache) Get(key string) (*[]byte, error) {
	// If so, move it to the front of the list
	cache.linkedListLock.RLock()
	metadata, ok := cache.keyToElement[key]
	cache.linkedListLock.RUnlock()
	if ok {
		cache.linkedListLock.Lock()
		cache.orderedCacheKeys.MoveToFront(metadata)
		cache.linkedListLock.Unlock()
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
		cache.linkedListLock.Lock()
		for cache.currentSize+metadata.Size > cache.maxSize {
			oldestElement := cache.orderedCacheKeys.Back()
			if oldestElement == nil {
				break
			}
			oldestKey := oldestElement.Value.(CacheMetadata).Key
			// We already hold an exclusive lock and don't want to try and re-aquire
			cache.deleteWithProtection(oldestKey, false)
		}
		cache.linkedListLock.Unlock()
	}

	// It's possible we would still exhaust the memory space, in which case we shouldn't
	// store it at all. Ensure we stay under the threshold.
	// We wrap this in a full lock as well since once we verify that we have the space overhead
	// we want to be the process that writes to it
	if hasSpace := cache.allocateSpace(&metadata); !hasSpace {
		return nil
	}

	// Determine if we were able to successfully save the value in the store
	// We only want to retain the metadata if it succeeded
	err := cache.SetValueCallback(cache, key, value)

	// The next size/list manipulations we need to do under lock
	cache.linkedListLock.Lock()
	defer cache.linkedListLock.Unlock()

	if err != nil {
		cache.currentSize -= metadata.Size
		return err
	}

	cache.keyToElement[key] = cache.orderedCacheKeys.PushFront(metadata)
	return nil
}

func (cache *LRUCache) Has(key string) bool {
	return cache.HasValueCallback(cache, key)
}

func (cache *LRUCache) Delete(key string) {
	cache.deleteWithProtection(key, true)
}

func (cache *LRUCache) DeleteAll() {
	cache.linkedListLock.Lock()
	cache.orderedCacheKeys = list.New()
	cache.currentSize = 0
	cache.keyToElement = make(map[string]*list.Element)
	cache.linkedListLock.Unlock()

	cache.DeleteAllCallback(cache)
}

func (cache *LRUCache) deleteWithProtection(key string, protected bool) {
	// We want the entire delete block to be protected because we are manipulating
	// the linked list and want to find the key, hold exclusive access to it, and
	// then delete it
	if protected {
		cache.linkedListLock.Lock()
	}

	// Housekeeping for the metadata
	metadata, ok := cache.keyToElement[key]
	if ok {
		cache.orderedCacheKeys.Remove(metadata)
		cache.currentSize -= metadata.Value.(CacheMetadata).Size
		delete(cache.keyToElement, key)
	}

	if protected {
		cache.linkedListLock.Unlock()
	}

	// Perform the actual deletion
	cache.DeleteKeyCallback(cache, key)
}

func (cache *LRUCache) allocateSpace(metadata *CacheMetadata) bool {
	/*
	 * Determine if we have enough space to allocate a new metadata in the cache
	 * If so, increment the space
	 */
	cache.linkedListLock.Lock()
	defer cache.linkedListLock.Unlock()

	hasSpace := cache.currentSize+metadata.Size <= cache.maxSize
	if hasSpace {
		cache.currentSize += metadata.Size
	}
	return hasSpace
}
