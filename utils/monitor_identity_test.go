package utils

import (
	"testing"

	"github.com/google/uuid"
)

func TestMonitorInstanceID(t *testing.T) {
	for _, value := range []string{"", "host-secret", uuid.Nil.String(), "../private"} {
		t.Setenv("MAY_MONITOR_INSTANCE_ID", value)
		if MonitorInstanceID() != "" {
			t.Fatal("invalid installation identifier exposed")
		}
	}
	id := uuid.NewString()
	t.Setenv("MAY_MONITOR_INSTANCE_ID", id)
	if MonitorInstanceID() != id {
		t.Fatal("installation identifier changed")
	}
}
