package downloader

import "os"

type file struct {
	file *os.File
	path string
}

type FileInfo struct {
	Size           int64
	Name           string
	RangeSupported bool
}
