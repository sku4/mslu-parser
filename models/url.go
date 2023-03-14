package models

type ExcelUrl struct {
	Url string
	*ExcelRow
}

type ExcelRow struct {
	Row int
}
