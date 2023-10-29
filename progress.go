package downloader

import "fmt"

type Progress struct {
	Total     int64
	Processed int64
}

func (p *Progress) Percent() int64 {
	return (100 * p.Processed) / p.Total
}

func (p *Progress) String() string {
	return fmt.Sprint(p.Percent(), "%")
}
