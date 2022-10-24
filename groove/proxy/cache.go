package main

import (
	"fmt"
	"log"
	"net/http"
	"os/user"
	"path"
	"sync"
	"time"

	goproxy "github.com/piercefreeman/goproxy"
	"github.com/pquerna/cachecontrol"

	lrucache "grooveproxy/cache"
)

const (
	// Caching is completely disabled on the proxy side, it will be up to
	// the client implementations to handle caching. When this is enabled we
	// will re-issue every request that we get
	CacheModeOff = iota

	// Caching is at the standard policy defined by the server. We will inspect
	// return headers for `Cache-Control` and if this is specified and we're within
	// the given time windwo, we'll return the cached results
	// We also ensure that only one outbound request for any URL is conducted at the same
	// time. If two requests come through for URL1 while URL1 is still being resolved from
	// the host, the second will block until the first comes back and we verify its cache entry.
	// This has the drawback of blocking until the request is completed (latency) while providing
	// the upshot of fewer requests to the end server.
	CacheModeStandard = iota

	// Cache all GET requests. A special case of CacheModeAggressive to allow POST requests to dynamically
	// update server content.
	CacheModeGetAggressive = iota

	// Cache everything, regardless of server driven cache status
	// Like `CacheModeStandard`, this will block until all end requests are resolved. But since we
	// cache everything here, we are guaranteed that the second request will resolve once the first
	// request has finished transit.
	CacheModeAggressive = iota
)

type CacheEntry struct {
	CacheInvalidation time.Time
	Value             *ArchivedResponse
	Error             string
}

type Cache struct {
	mode int // CacheModeOff | CacheModeStandard | CacheModeAggressive

	// Disk cache comes bundled with a RWMutex so we can call functions directly
	// and the lock will be handled internally
	cacheDiskCache *lrucache.CacheInvalidator

	inflightRequests map[string]*sync.Mutex
	lockGeneration   *sync.Mutex

	// Current locks that are blocking on gaining exclusive access to their lock
	blockingLocks      map[string]int
	blockingLocksMutex *sync.RWMutex
}

func NewCache(cacheSizeMaxMB uint64) *Cache {
	user, err := user.Current()
	if err != nil {
		log.Fatal(fmt.Errorf("Unable to resolve current user: %w", err))
		return nil
	}

	// We treat the disk cache as a raw key/value store and manipulate it accordingly
	// We therefore append a fresh cache directory to their path to ensure no file condicts
	cachePath := path.Join(user.HomeDir, ".grooveproxy/cache")

	return &Cache{
		mode:               CacheModeStandard,
		cacheDiskCache:     lrucache.NewCacheInvalidator(cachePath, 20, 500, 10),
		inflightRequests:   map[string]*sync.Mutex{},
		lockGeneration:     &sync.Mutex{},
		blockingLocks:      make(map[string]int),
		blockingLocksMutex: &sync.RWMutex{},
	}
}

func (c *Cache) SetValidCacheContents(request *http.Request, response *http.Response) {
	/*
	 * Attempts to update the current cache with given request/response. As part of this function
	 * we will determine if this is a valid payload to cache and will no-op if invalid.
	 */
	// No-op if we are disabled
	if c.mode == CacheModeOff {
		return
	}

	// Some older implements don't provide this and instead use Last-Modified
	// According to [Mozilla](https://developer.mozilla.org/en-US/docs/Web/HTTP/Caching) the heuristic
	// is to use 0.1% of the difference between last modified and current date but I was unable
	// to find a source for this in the W3 spec: https://www.rfc-editor.org/rfc/rfc9110.html#name-caches
	// For now we only implement Cache-Control
	// TODO: Explicit handling for redirects?
	noCacheReasons, expires, _ := cachecontrol.CachableResponse(request, response, cachecontrol.Options{})

	if c.isModeAggressive(request) || len(noCacheReasons) == 0 {
		log.Printf("Caching response for %s", request.URL.String())
		cacheEntry := &CacheEntry{
			CacheInvalidation: expires,
			Value:             responseToArchivedResponse(response),
		}
		err := c.cacheDiskCache.Set(getCacheKey(request), cacheEntry)
		if err != nil {
			log.Printf("Failed to set cache entry for key: %s %s", request.URL.String(), err.Error())
		}
	}
}

func (c *Cache) SetFailedCacheContents(request *http.Request, err error) {
	/*
	 * Indicate that the given request failed (no response was received at all), can be used to avoid
	 * querying that same resource in the future.
	 * Only applies for aggressive modes
	 */
	if !c.isModeAggressive(request) {
		return
	}

	// Set default expiration in 24 hours
	expires := time.Now().Add(24 * time.Hour)

	cacheEntry := &CacheEntry{
		CacheInvalidation: expires,
		Value:             nil,
		Error:             err.Error(),
	}
	err = c.cacheDiskCache.Set(getCacheKey(request), cacheEntry)
	if err != nil {
		log.Printf("Failed to set cache entry for key: %s", request.URL.String())
	}
}

func (c *Cache) GetCacheContents(request *http.Request) *CacheEntry {
	// No-op if we are disabled, since we don't use request based locks
	if c.mode == CacheModeOff {
		return nil
	}

	requestKey := getCacheKey(request)

	if !c.cacheDiskCache.Has(requestKey) {
		return nil
	}

	var cache CacheEntry
	err := c.cacheDiskCache.Get(requestKey, &cache)

	if err != nil {
		log.Printf("Failed to read cache entry for key: %s (%s)", request.URL.String(), requestKey)
		return nil
	}

	// Determine if the cache is still valid
	if c.cacheEntryValid(request, &cache) {
		log.Printf("Return cache value: %s (%s)", request.URL.String(), requestKey)
		return &cache
	}

	log.Printf("Cache miss: %s (%s)", request.URL.String(), requestKey)
	return nil
}

func (c *Cache) AcquireRequestLock(url string) {
	/*
	 * Behavior of this function is mode dependent
	 * Cache enabled: If there's an inflight request for this URL, we should wait
	 *   If we access this lock, we know there are no other requests in flight
	 * Cache disabled: Immediately continue
	 */

	// no-op if cache is disabled, an unlimited number of clients should be able to acquire
	// a resource lock at any one time
	if c.mode == CacheModeOff {
		return
	}

	// Avoid race condition introduced by adding multiple url-based locks
	// at the same time
	c.lockGeneration.Lock()

	lock, ok := c.inflightRequests[url]
	if !ok {
		lock = &sync.Mutex{}
		c.inflightRequests[url] = lock
	}

	// Explicitly unlock here since we want to unlock it right after getting the main lock
	c.lockGeneration.Unlock()

	c.blockingLocksMutex.Lock()
	if _, ok := c.blockingLocks[url]; !ok {
		c.blockingLocks[url] = 0
	}
	c.blockingLocks[url] += 1
	c.blockingLocksMutex.Unlock()

	c.LogBlockingRequests()

	lock.Lock()

	c.blockingLocksMutex.Lock()
	c.blockingLocks[url] -= 1
	c.blockingLocksMutex.Unlock()

	c.LogBlockingRequests()
}

func (c *Cache) LogBlockingRequests() {
	c.blockingLocksMutex.RLock()

	log.Println("---BLOCKING REQUESTS---")
	for count_url, count_values := range c.blockingLocks {
		if count_values > 0 {
			log.Printf("Blocking Locks: %s: %d\n", count_url, count_values)
		}
	}
	log.Println("---END---")

	c.blockingLocksMutex.RUnlock()
}

func (c *Cache) ReleaseRequestLock(url string) {
	// We opt not to have a mode check here, because in the case of changing
	// the cache mode mid-operation we still want to allow client callers to
	// unlock the locks of inflight requests

	lock, ok := c.inflightRequests[url]
	if ok {
		// We want to allow for liberal request unlocking, ie. we want to call this
		// function even if we aren't guaranteed that there's a lock
		// Calling Unlock by default will result in a runtime exception if it
		// hasn't already been locked. TryLock here will return a boolean if the
		// locking fails (which is okay here since it means that it's been locked elsewhere)
		// The combination of these two stages ensures that we can always successfully unlock
		lock.TryLock()
		lock.Unlock()
	}
}

func (c *Cache) Clear() {
	// Will erase everything from the given disk
	c.cacheDiskCache.Clear()
}

func (c *Cache) cacheEntryValid(request *http.Request, cacheEntry *CacheEntry) bool {
	/*
	 * Given a cache entry determine if the specified date is still valid
	 */
	// If a cache value exists and we are performing aggressive caching, we don't care
	// about expiration time
	if c.isModeAggressive(request) {
		return true
	}

	now := time.Now()
	return now.Before(cacheEntry.CacheInvalidation)
}

func (c *Cache) isModeAggressive(request *http.Request) bool {
	/*
	 * Aggressive modes are a special form of caching, where we will cache everything assuming the request
	 * meets some properties.
	 * - CacheModeAggressive: Cache everything
	 * - CacheModeGetAggressive: Cache all GET requests
	 *
	 * This function will return true if the given request should quality for aggressive handling AND if the
	 * current mode allows for aggressive caching.
	 */
	if c.mode == CacheModeAggressive {
		return true
	}

	if c.mode == CacheModeGetAggressive && request.Method == "GET" {
		return true
	}

	return false
}

func setupCacheMiddleware(proxy *goproxy.ProxyHttpServer, cache *Cache, recorder *Recorder) {
	proxy.OnRequest().DoFunc(
		/*
		 * Cache layer
		 */
		func(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
			// Only cache if we are not replaying the tape
			if recorder.mode == RecorderModeRead {
				return r, nil
			}

			// Determine if we have a cache result available
			cacheValue := cache.GetCacheContents(r)
			if cacheValue != nil {
				if cacheValue.Value != nil {
					return r, archivedResponseToResponse(r, cacheValue.Value)
				} else {
					// We should fail the request
					return r, goproxy.NewResponse(r, goproxy.ContentTypeText, http.StatusBadGateway, cacheValue.Error)
				}
			}

			// If we got here, we couldn't immediately resolve the cache
			// Determine if we have permission to proceed for this URL
			// FIX: This causes a deadlock right now because these request handling aren't goroutines
			// therefore they will run inline with the rest of the program and block each other
			log.Printf("Will acquire lock: %s\n", r.URL.String())
			cache.AcquireRequestLock(r.URL.String())
			log.Printf("Did acquire lock: %s\n", r.URL.String())

			// We now have permission to access this URL and should continue until complete
			return r, nil
		},
	)

	proxy.OnResponse().DoFunc(
		/*
		 * Cache layer
		 */
		func(response *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
			if ctx.Error != nil {
				request := ctx.Req
				cache.SetFailedCacheContents(request, ctx.Error)
				cache.ReleaseRequestLock(request.URL.String())
				return nil
			}

			if recorder.mode == RecorderModeRead {
				return response
			}

			// Attempt to update the cache with each historical value, since there
			// might have been multiple hops in this request?
			requestHistory, responseHistory := getRedirectHistory(response)

			// When replaying this we want to replay it in order to capture all the
			// event history and test redirect handlers
			for i := 0; i < len(requestHistory); i++ {
				request := requestHistory[i]
				response := responseHistory[i]

				cache.SetValidCacheContents(request, response)
				log.Printf("Will release lock: %s\n", request.URL.String())
				cache.ReleaseRequestLock(request.URL.String())
				log.Printf("Did release lock: %s\n", request.URL.String())
			}

			return response
		},
	)
}
