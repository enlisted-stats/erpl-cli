//go:build !windows

package replays

import (
	"io/fs"
	"time"
)

// created returns the modification time: other systems do not keep the creation time portably.
func created(info fs.FileInfo) time.Time { return info.ModTime() }
