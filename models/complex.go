package models

type Complex struct {
	Title           string
	OverTitle       string
	Lead            string
	Subtitles       []string
	ImageTitles     []string
	TooManyRequests bool
	ExcelUrl
}
