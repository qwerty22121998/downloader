package downloader

import (
	"bytes"
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewDownloader(t *testing.T) {
	url := "https://dl.google.com/go/go1.21.3.linux-amd64.tar.gz"
	d, err := NewDownloader(url, Config{
		ConnectionNumber: 1 << 2,
		TmpPath:          "./tmp",
		BytePerSec:       nil, //ptr[int64](1 << 20),
		BufferSize:       1 << 10,
	})
	assert.NoError(t, err)
	assert.Equal(t, d.FileInfo.RangeSupported, true)
	assert.Equal(t, d.FileInfo.Name, "go1.21.3.linux-amd64.tar.gz")
	assert.Equal(t, d.FileInfo.Size, int64(66641773))
	data, err := d.Save()
	assert.NoError(t, err)
	fmt.Println(string(data))

	parsed, err := LoadDownloader(bytes.NewReader(data))
	assert.NoError(t, err)
	assert.Equal(t, parsed.FileInfo.RangeSupported, true)
	assert.Equal(t, parsed.FileInfo.Name, "go1.21.3.linux-amd64.tar.gz")
	assert.Equal(t, parsed.FileInfo.Size, int64(66641773))
	//go d.debug()
	assert.NoError(t, d.Start())

}
