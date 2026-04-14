//go:build windows

package offset

import "os"

func fileInode(fi os.FileInfo) uint64 {
	return 0 // Windows doesn't have inodes; rotation detection skipped
}
