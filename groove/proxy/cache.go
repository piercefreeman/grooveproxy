package main

import (
	"log"
	"net/http"
	"sync"
	"time"

	goproxy "github.com/piercefreeman/goproxy"
	"github.com/pquerna/cachecontrol"
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

	// Cache everything, regardless of server driven cache status
	// Like `CacheModeStandard`, this will block until all end requests are resolved. But since we
	// cache everything here, we are guaranteed that the second request will resolve once the first
	// request has finished transit.
	CacheModeAggressive = iota
)

type CacheEntry struct {
	cacheInvalidation time.Time
	// TODO: make pointer to underlying value
	value *ArchivedResponse
}

type Cache struct {
	mode             int // CacheModeOff | CacheModeStandard | CacheModeAggressive
	cacheValues      map[string]*CacheEntry
	inflightRequests map[string]*sync.Mutex
	lockGeneration   *sync.Mutex
}

func NewCache() *Cache {
	return &Cache{
		mode:             CacheModeStandard,
		cacheValues:      map[string]*CacheEntry{},
		inflightRequests: map[string]*sync.Mutex{},
		lockGeneration:   &sync.Mutex{},
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
	reasons, expires, _ := cachecontrol.CachableResponse(request, response, cachecontrol.Options{})

	if c.mode == CacheModeAggressive || len(reasons) == 0 {
		c.cacheValues[request.URL.String()] = &CacheEntry{
			cacheInvalidation: expires,
			value:             responseToArchivedResponse(response),
		}
	}
}

func (c *Cache) GetCacheContents(url string) *ArchivedResponse {
	// No-op if we are disabled, since we don't use request based locks
	if c.mode == CacheModeOff {
		return nil
	}

	// Only one cache entry appears in the cache at any one time
	// Determine if this can instantly be retrieved without having to acquire the lock
	// (allows for slightly faster access in a non-race condition risk way)
	cache, ok := c.cacheValues[url]

	if ok {
		// Determine if the cache is still valid
		if c.cacheEntryValid(cache) {
			return cache.value
		}
	}

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
	defer c.lockGeneration.Unlock()

	lock, ok := c.inflightRequests[url]
	if !ok {
		lock = &sync.Mutex{}
		c.inflightRequests[url] = lock
	}

	lock.Lock()
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
	for key, _ := range c.cacheValues {
		delete(c.cacheValues, key)
	}
}

func (c *Cache) cacheEntryValid(cacheEntry *CacheEntry) bool {
	/*
	 * Given a cache entry determine if the specified date is still valid
	 */
	// If a cache value exists and we are performing aggressive caching, we don't care
	// about expiration time
	if c.mode == CacheModeAggressive {
		return true
	}

	now := time.Now()
	return now.Before(cacheEntry.cacheInvalidation)
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
			cacheValue := cache.GetCacheContents(r.URL.String())
			if cacheValue != nil {
				return r, archivedResponseToResponse(r, cacheValue)
			}

			// If we got here, we couldn't immediately resolve the cache
			// Determine if we have permission to proceed for this URL
			log.Println("Will acquire lock")
			cache.AcquireRequestLock(r.URL.String())
			log.Println("Did acquire lock")

			// We now have permission to access this URL and should continue until complete
			return r, nil
		},
	)

	proxy.OnResponse().DoFunc(
		/*
		 * Cache layer
		 */
		func(response *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
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
				cache.ReleaseRequestLock(request.URL.String())
			}

			return response
		},
	)
}
