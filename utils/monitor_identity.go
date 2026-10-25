package utils

import (
	"os"

	"github.com/google/uuid"
)

// MonitorInstanceID is an opaque installation identifier supplied by the
// lifecycle owner. It is never a machine ID, provider ID or credential.
// An unset identifier keeps old readers working but cannot prove an origin.
func MonitorInstanceID() string {
	value := os.Getenv("MAY_MONITOR_INSTANCE_ID")
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil || id.String() != value {
		return ""
	}
	return value
}
