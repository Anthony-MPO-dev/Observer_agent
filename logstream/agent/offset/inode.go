package offset

import "os"

// FileInode returns the inode number of a file.
// On Linux this uses Stat_t.Ino. Falls back to 0 on unsupported platforms.
func FileInode(path string) (uint64, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return fileInode(fi), nil
}
