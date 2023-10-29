package downloader

import "fmt"

type Part struct {
	Progress Progress
	Index    int64
	From     int64
	To       int64
	tmpFile  *file
	stop     chan struct{}
}

func (c *Part) Range() string {
	return fmt.Sprintf("bytes=%d-%d", c.From, c.To)
}

func NewPart(index int64, size int64, from int64, to int64, tmpPath string) (*Part, error) {
	part := &Part{
		Progress: Progress{
			Total:     size,
			Processed: 0,
		},
		Index: index,
		From:  from,
		To:    to,
		tmpFile: &file{
			file: nil,
			path: tmpPath,
		},
		stop: make(chan struct{}),
	}
	if err := part.init(); err != nil {
		return nil, err
	}
	return part, nil
}

func (p *Part) init() error {
	tmpFile, err := forceCreateFile(p.tmpFile.path)
	if err != nil {
		return err
	}
	p.tmpFile.file = tmpFile
	return nil
}
