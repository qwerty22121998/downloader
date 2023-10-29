package downloader

import "fmt"

type Config struct {
	ConnectionNumber int64
	TmpPath          string
	BytePerSec       *int64
	BufferSize       int64
}

func (c *Config) generateTmpPath(name string, index int64) string {
	return fmt.Sprintf("%v/%v.part%v", c.TmpPath, name, index)
}
