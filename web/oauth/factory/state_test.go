package factory

import (
	"fmt"
	"github.com/patrickmn/go-cache"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestOAuthStateIsOneTimeExpiringAndBounded(t *testing.T) {
	c := cache.New(time.Minute, 0)
	if !StoreState(c, "synthetic") {
		t.Fatal("store failed")
	}
	item := c.Items()["synthetic"]
	if item.Expiration == 0 {
		t.Fatal("state never expires")
	}
	var successes atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ConsumeState(c, "synthetic") {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatalf("state consumed %d times", successes.Load())
	}
	c.Set("expired", true, time.Nanosecond)
	time.Sleep(time.Millisecond)
	if ConsumeState(c, "expired") {
		t.Fatal("expired state accepted")
	}
	for i := 0; i < 1024; i++ {
		if !StoreState(c, fmt.Sprintf("synthetic-%d", i)) {
			t.Fatal("early capacity rejection")
		}
	}
	if StoreState(c, "overflow") || c.ItemCount() > 1024 {
		t.Fatal("unbounded state cache")
	}
}
