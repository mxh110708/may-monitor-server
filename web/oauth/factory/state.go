package factory

import (
	"github.com/patrickmn/go-cache"
	"sync"
)

var stateMu sync.Mutex

func StoreState(c *cache.Cache, state string) bool {
	stateMu.Lock()
	defer stateMu.Unlock()
	if c == nil || state == "" {
		return false
	}
	c.DeleteExpired()
	if c.ItemCount() >= 1024 {
		return false
	}
	c.Set(state, true, cache.DefaultExpiration)
	return true
}

// Lookup and removal are atomic so two concurrent callbacks cannot reuse state.
func ConsumeState(c *cache.Cache, state string) bool {
	stateMu.Lock()
	defer stateMu.Unlock()
	if c == nil || state == "" {
		return false
	}
	_, ok := c.Get(state)
	c.Delete(state)
	return ok
}
