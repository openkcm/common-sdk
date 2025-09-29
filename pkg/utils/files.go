package utils

import (
	"errors"
	"io/fs"
	"os"
)

// FileExist checks if a filesystem path exists. It distinguishes between
// "does not exist" and other errors (e.g., permission denied).
func FileExist(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}

	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}

	return false, err
}
