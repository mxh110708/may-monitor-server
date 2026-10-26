package reviewedtheme

import (
	"os"
	"syscall"
)

func rootOwned(info os.FileInfo) bool {
	value, ok := info.Sys().(*syscall.Stat_t)
	return ok && value.Uid == 0
}
