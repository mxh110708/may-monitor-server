package integration

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

func TestReadCredentialFailsClosed(t *testing.T) {
	now := time.Now().UTC()
	token := TokenPrefix + base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	sum := sha256.Sum256([]byte(token))
	digest := hex.EncodeToString(sum[:])
	id := "a8098c1a-f86e-41d8-b4c6-68dd79a3b344"
	for _, tt := range []struct {
		name, hash, clients, expires, header string
		valid                                bool
	}{
		{"valid", digest, id, now.Add(time.Hour).Format(time.RFC3339), "Bearer " + token, true},
		{"not-enabled", "", id, now.Add(time.Hour).Format(time.RFC3339), "Bearer " + token, false},
		{"wrong-digest", strings.Repeat("f", 64), id, now.Add(time.Hour).Format(time.RFC3339), "Bearer " + token, false},
		{"admin-key", digest, id, now.Add(time.Hour).Format(time.RFC3339), "Bearer synthetic-admin-key", false},
		{"missing-header", digest, id, now.Add(time.Hour).Format(time.RFC3339), "", false},
		{"missing-scope", digest, "", now.Add(time.Hour).Format(time.RFC3339), "Bearer " + token, false},
		{"invalid-id", digest, "*", now.Add(time.Hour).Format(time.RFC3339), "Bearer " + token, false},
		{"nil-id", digest, "00000000-0000-0000-0000-000000000000", now.Add(time.Hour).Format(time.RFC3339), "Bearer " + token, false},
		{"duplicate-id", digest, id + "," + id, now.Add(time.Hour).Format(time.RFC3339), "Bearer " + token, false},
		{"oversized-scope", digest, strings.Repeat(id+",", MaxClients+1), now.Add(time.Hour).Format(time.RFC3339), "Bearer " + token, false},
		{"missing-expiry", digest, id, "", "Bearer " + token, false},
		{"expired", digest, id, now.Add(-time.Second).Format(time.RFC3339), "Bearer " + token, false},
		{"excessive-expiry", digest, id, now.Add(91 * 24 * time.Hour).Format(time.RFC3339), "Bearer " + token, false},
		{"malformed-token", digest, id, now.Add(time.Hour).Format(time.RFC3339), "Bearer may_read_short", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("MAY_MONITOR_READ_TOKEN_SHA256", tt.hash)
			t.Setenv("MAY_MONITOR_READ_CLIENT_UUIDS", tt.clients)
			t.Setenv("MAY_MONITOR_READ_EXPIRES_AT", tt.expires)
			ids, ok := authorize(tt.header, now)
			if ok != tt.valid || ok && (len(ids) != 1 || ids[0] != id) {
				t.Fatalf("authorization result=%v, want=%v", ok, tt.valid)
			}
		})
	}
}
