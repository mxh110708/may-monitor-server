//go:build linux

package monitoring

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func lockWatchdogState(path string) (func(), error) {
	fd, err := unix.Open(path+".lock", unix.O_RDWR|unix.O_CREAT|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if err != nil {
		return nil, errors.New("watchdog state lock is unavailable")
	}
	file := os.NewFile(uintptr(fd), path+".lock")
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB) != nil {
		file.Close()
		return nil, errors.New("watchdog state is unsafe or already in use")
	}
	return func() { file.Close() }, nil
}
