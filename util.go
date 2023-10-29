package downloader

import (
	"os"
	"path/filepath"
)

func forceCreateFile(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
		return nil, err
	}
	return os.Create(path)
}

func ptr[T any](value T) *T {
	return &value
}
