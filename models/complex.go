package models

type Complex struct {
	Title           string
	OverTitle       string
	Lead            string
	Subtitles       []string
	TooManyRequests bool
	ExcelUrl
}
