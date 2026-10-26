//go:build !linux

package reviewedtheme

import "os"

// Reviewed static-theme operations are currently supported on Debian only.
func rootOwned(info os.FileInfo) bool { return false }
