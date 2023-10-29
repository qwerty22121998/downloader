package downloader

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"sync"
	"time"
)

type Downloader struct {
	Config   *Config
	URL      string
	FileInfo *FileInfo
	MpPart   map[int64]*Part
	client   *http.Client
	wg       sync.WaitGroup
	err      chan error
}

func LoadDownloader(data io.Reader) (*Downloader, error) {
	var d *Downloader
	if err := gob.NewDecoder(data).Decode(&d); err != nil {
		return nil, err
	}
	return d, nil
}

func NewDownloader(url string, config Config) (*Downloader, error) {
	d := &Downloader{
		Config: &config,
		URL:    url,
		client: &http.Client{
			Timeout: 0,
		},
		FileInfo: nil,
		MpPart:   make(map[int64]*Part),
		err:      make(chan error),
	}
	if err := d.getFileInfo(); err != nil {
		return nil, err
	}
	if err := d.splitPart(); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *Downloader) Save() ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(d); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (d *Downloader) getFileInfo() error {
	req, err := http.NewRequest(http.MethodHead, d.URL, nil)
	if err != nil {
		return err
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer func(body io.ReadCloser) {
		err := body.Close()
		if err != nil {
			slog.Error("error when close response body", "error", err)
		}
	}(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("get file info failed with status code %v", resp.StatusCode)
	}
	// check range header
	acceptRange := resp.Header.Get("Accept-Ranges")
	// file size
	contentLengthStr := resp.Header.Get("Content-Length")
	if len(contentLengthStr) == 0 {
		return errors.New("can not get content length")
	}
	contentLength, err := strconv.Atoi(contentLengthStr)
	if err != nil {
		return err
	}
	// guess file name
	fileName := ""
	contentDisposition := resp.Header.Get("Content-Disposition")
	if len(contentDisposition) != 0 {
		_, params, err := mime.ParseMediaType(contentDisposition)
		if err == nil {
			fileName = params["filename"]
		}
	}
	// cannot get name from header
	if fileName == "" {
		fileURL, err := url.Parse(d.URL)
		if err != nil {
			return err
		}
		fileURL.RawQuery = ""
		fileName = path.Base(fileURL.String())
	}
	d.FileInfo = &FileInfo{
		Size:           int64(contentLength),
		Name:           fileName,
		RangeSupported: acceptRange == "bytes",
	}
	return nil
}

func (d *Downloader) splitPart() error {
	if !d.FileInfo.RangeSupported {
		d.Config.ConnectionNumber = 1
		part := &Part{
			Index:   0,
			From:    0,
			To:      d.FileInfo.Size,
			tmpFile: nil,
		}
		d.MpPart[part.Index] = part
	}
	cNumber := d.Config.ConnectionNumber
	partSize := d.FileInfo.Size / cNumber
	var from int64
	for i := int64(0); i < cNumber; i++ {
		curSize := partSize
		if i == cNumber-1 {
			curSize += d.FileInfo.Size % cNumber
		}
		part, err := NewPart(i, curSize, from, from+curSize-1, d.Config.generateTmpPath(d.FileInfo.Name, i))
		if err != nil {
			return err
		}

		from += curSize
		d.MpPart[part.Index] = part
	}
	return nil
}

func (d *Downloader) combineFile() error {
	file, err := forceCreateFile(d.FileInfo.Name)
	if err != nil {
		return err
	}
	defer file.Close()
	for i := int64(0); i < d.Config.ConnectionNumber; i++ {
		if err := func() error {
			part := d.MpPart[i]
			_, err := part.tmpFile.file.Seek(0, 0)
			if err != nil {
				return err
			}
			defer part.tmpFile.file.Close()

			writeAt := io.NewOffsetWriter(file, part.From)
			if _, err := io.Copy(writeAt, part.tmpFile.file); err != nil {
				return err
			}
			return nil
		}(); err != nil {
			return err
		}
	}
	for i := int64(0); i < d.Config.ConnectionNumber; i++ {
		slog.Info("rem", "part", d.MpPart[i].tmpFile.path)
		if err := os.Remove(d.MpPart[i].tmpFile.path); err != nil {
			slog.Info("error when remove file", "path", d.MpPart[i].tmpFile.path, "error", err)
		}
	}
	return nil
}

func (d *Downloader) processPart(p *Part) {
	defer d.wg.Done()
	req, err := http.NewRequest(http.MethodGet, d.URL, nil)
	if err != nil {
		d.err <- err
		return
	}
	req.Header.Add("Range", p.Range())
	resp, err := d.client.Do(req)
	if err != nil {
		d.err <- err
		return
	}
	defer func(body io.ReadCloser) {
		err := body.Close()
		if err != nil {
			slog.Error("error when close response body", "error", err)
		}
	}(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		d.err <- fmt.Errorf("request failed with status code %v", resp.StatusCode)
	}
	//speed limit
	if d.Config.BytePerSec != nil {
		bufferSize := *d.Config.BytePerSec
		t := time.Tick(time.Second)
		for {
			select {
			case <-p.stop:
				return
			case <-t:
				written, err := io.CopyN(p.tmpFile.file, resp.Body, bufferSize)
				if err != nil {
					if err == io.EOF {
						return
					}
					d.err <- err
					return
				}
				p.Progress.Processed += written
			}
		}
	} else {
		for {
			select {
			case <-p.stop:
				return
			default:
				written, err := io.CopyN(p.tmpFile.file, resp.Body, d.Config.BufferSize)
				if err != nil {
					if err == io.EOF {
						return
					}
					d.err <- err
					return
				}
				p.Progress.Processed += written
			}
		}
	}

}

func (d *Downloader) debug() {
	t := time.Tick(time.Second)
	for range t {
		for i := int64(0); i < d.Config.ConnectionNumber; i++ {
			slog.Info("info", "thread", i, "progress", d.MpPart[i].Progress.Percent())
		}
		slog.Info("------------------------")
	}
}

func (d *Downloader) Start() error {
	if err := d.splitPart(); err != nil {
		return nil
	}
	for _, part := range d.MpPart {
		part := part
		d.wg.Add(1)
		go d.processPart(part)
	}
	go func() {
		for err := range d.err {
			slog.Error("error received", "url", d.URL, "error", err)
			for _, part := range d.MpPart {
				part.stop <- struct{}{}
			}
		}
	}()
	d.wg.Wait()

	return d.combineFile()
}
