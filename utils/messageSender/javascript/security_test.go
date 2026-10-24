package javascript

import (
	"context"
	"github.com/komari-monitor/komari/utils/messageSender"
	"testing"
)

func TestScriptNotificationDoesNotBypassExtensionPolicy(t *testing.T) {
	t.Setenv("MAY_MONITOR_ALLOW_EXTENSIONS", "")
	var sender JavaScriptSender
	err := sender.Send(context.Background(), messageSender.Notification{}, map[string]any{"script": "function sendMessage() { return true; }"})
	if err == nil || sender.runtime != nil {
		t.Fatal("notification script started while extensions were disabled")
	}
}
