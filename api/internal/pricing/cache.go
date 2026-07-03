package pricing

import (
	"sort"
	"time"

	"github.com/dgraph-io/ristretto/v2"
)

type Cache struct {
	c   *ristretto.Cache[string, *LookupResult]
	ttl time.Duration
}

func NewCache(maxCost int64, ttl time.Duration) (*Cache, error) {
	c, err := ristretto.NewCache(&ristretto.Config[string, *LookupResult]{
		NumCounters: maxCost * 10,
		MaxCost:     maxCost,
		BufferItems: 64,
	})
	if err != nil {
		return nil, err
	}
	return &Cache{c: c, ttl: ttl}, nil
}

func (c *Cache) Get(key string) (*LookupResult, bool) {
	return c.c.Get(key)
}

func (c *Cache) Set(key string, val *LookupResult) {
	c.c.SetWithTTL(key, val, 1, c.ttl)
}

func CacheKey(serviceCode, region, term string, attrs map[string]string) string {
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	buf := serviceCode + "|" + region + "|" + term
	for _, k := range keys {
		buf += "|" + k + "=" + attrs[k]
	}
	return buf
}
