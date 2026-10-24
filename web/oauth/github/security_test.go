package github

import (
	"strings"
	"testing"
	"time"
)

func TestGitHubStateExpiresAndCannotBeReplayed(t *testing.T) {
	provider := &Github{}
	if err := provider.Init(); err != nil {
		t.Fatal(err)
	}
	defer provider.Destroy()
	_, state := provider.GetAuthorizationURL("")
	item, ok := provider.stateCache.Items()[state]
	if !ok || item.Expiration <= time.Now().UnixNano() {
		t.Fatal("state missing expiration")
	}
	// No code means no upstream HTTP request is made, but the nonce is consumed.
	if _, err := provider.OnCallback(nil, state, map[string]string{}, ""); err == nil {
		t.Fatal("empty authorization accepted")
	}
	if _, err := provider.OnCallback(nil, state, map[string]string{"code": "synthetic"}, ""); err == nil || !strings.Contains(err.Error(), "invalid state") {
		t.Fatal("OAuth state replay was not rejected before network access")
	}
}
