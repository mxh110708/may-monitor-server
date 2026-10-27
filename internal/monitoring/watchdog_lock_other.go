//go:build !linux

package monitoring

import "errors"

func lockWatchdogState(string) (func(), error) {
	return nil, errors.New("watchdog state locking currently requires Linux")
}
