package cache

import (
	"strings"
	"time"

	gocache "github.com/patrickmn/go-cache"
)

var cache = gocache.New(5*time.Minute, 10*time.Hour)

func GetCache[T any](key string) (value T, ok bool) {
	v, found := cache.Get(key)
	if !found {
		return value, false
	}
	value, ok = v.(T)
	if !ok {
		return value, false
	}
	return value, true
}

func SetCache[T any](key string, value T, ttl time.Duration) {
	cache.Set(key, value, ttl)
}

func RemoveCacheWithPrefix(prefix string) {
	items := cache.Items()
	for key := range items {
		if strings.HasPrefix(key, prefix) {
			cache.Delete(key)
		}
	}
}

func RemoveCache(key string) {
	if _, ok := cache.Get(key); ok {
		cache.Delete(key)
	}
}
