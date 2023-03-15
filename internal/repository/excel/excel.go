package excel

import (
	"context"
	"github.com/pkg/errors"
	"github.com/sku4/mslu-parser/models"
	"github.com/xuri/excelize/v2"
	"hash/crc32"
	"os"
	"strconv"
	"strings"
)

const (
	parserXlsFile   = "parser.xlsx"
	parserXlsSheet1 = "Sheet1"
)

type Excel struct {
	parserFile *excelize.File
	rowsCount  int
	crcTable   *crc32.Table
}

func New() *Excel {
	var f *excelize.File
	_, err := os.Stat(parserXlsFile)
	if os.IsNotExist(err) {
		f = excelize.NewFile()
		_, _ = f.NewSheet(parserXlsSheet1)
	} else {
		f, _ = excelize.OpenFile(parserXlsFile)
	}
	rows, _ := f.GetRows(parserXlsSheet1)
	crcTable := crc32.MakeTable(crc32.IEEE)

	return &Excel{
		parserFile: f,
		rowsCount:  len(rows),
		crcTable:   crcTable,
	}
}

func (e *Excel) GetUsedUrls(context.Context) (map[uint32]*models.ExcelRow, error) {
	ss := make(map[uint32]*models.ExcelRow, 0)

	rows, err := e.parserFile.GetRows(parserXlsSheet1)
	if err != nil {
		return nil, errors.Wrap(err, "Get used urls")
	}

	for i, row := range rows {
		url := crc32.Checksum([]byte(row[0]), e.crcTable)
		ss[url] = &models.ExcelRow{
			Row: i + 1,
		}
	}

	return ss, nil
}

func (e *Excel) SetComplex(ctx context.Context, modelComplex models.Complex) error {
	n := 0
	if modelComplex.ExcelRow != nil && modelComplex.ExcelRow.Row > 0 {
		n = modelComplex.ExcelRow.Row
	} else {
		e.rowsCount++
		n = e.rowsCount
	}
	err := e.parserFile.SetCellValue(parserXlsSheet1, "A"+strconv.Itoa(n), modelComplex.Url)
	if err != nil {
		return err
	}
	_ = e.parserFile.SetCellValue(parserXlsSheet1, "B"+strconv.Itoa(n), modelComplex.Title)
	_ = e.parserFile.SetCellValue(parserXlsSheet1, "C"+strconv.Itoa(n), modelComplex.OverTitle)
	_ = e.parserFile.SetCellValue(parserXlsSheet1, "D"+strconv.Itoa(n), modelComplex.Lead)
	_ = e.parserFile.SetCellValue(parserXlsSheet1, "E"+strconv.Itoa(n), strings.Join(modelComplex.Subtitles, "\n"))

	if checkEverySave() {
		if err = e.parserFile.SaveAs(parserXlsFile); err != nil {
			return err
		}
	}

	return nil
}

func (e *Excel) Close() error {
	if err := e.parserFile.SaveAs(parserXlsFile); err != nil {
		return err
	}
	if err := e.parserFile.Close(); err != nil {
		return err
	}

	return nil
}

var checkEverySave = func() func() bool {
	c := -1
	return func() bool {
		c++
		return c%10 == 0
	}
}()
