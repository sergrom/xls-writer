// Package xlswriter creates BIFF8 XLS workbooks containing text cells.
// Workbooks are built in memory and are not safe for concurrent mutation.
package xlswriter

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"
)

// Limits supported by the encoder. MaxSheets is an implementation limit.
const (
	MaxRows       = 65536
	MaxColumns    = 256
	MaxCellLength = 32767 // UTF-16 code units
	MaxSheets     = 255
)

// XlsWriter represents one workbook. Its zero value is ready to use.
type XlsWriter struct {
	properties Properties
	sheets     []*Sheet
}

// Sheet is a worksheet created by AddSheet.
type Sheet struct{ data worksheet }

// Properties contains workbook metadata. Empty fields are omitted.
type Properties struct {
	Title          string
	Subject        string
	Creator        string
	Keywords       string
	Description    string
	LastModifiedBy string
}

// New creates an empty workbook.
func New() *XlsWriter { return &XlsWriter{} }

// SetProperties replaces the workbook metadata.
func (w *XlsWriter) SetProperties(properties Properties) { w.properties = properties }

// AddSheet appends a sheet with a unique, case-insensitive name.
// Names must contain 1–31 UTF-16 code units and no Excel-reserved characters.
func (w *XlsWriter) AddSheet(name string) (*Sheet, error) {
	if !utf8.ValidString(name) || len(utf16.Encode([]rune(name))) > 31 || strings.TrimSpace(name) == "" || strings.ContainsAny(name, "[]:*?/\\\x00\x03") || strings.HasPrefix(name, "'") || strings.HasSuffix(name, "'") {
		return nil, fmt.Errorf("xlswriter: invalid sheet name %q", name)
	}
	for _, sheet := range w.sheets {
		if strings.EqualFold(sheet.data.Name, name) {
			return nil, fmt.Errorf("xlswriter: duplicate sheet name %q", name)
		}
	}
	if len(w.sheets) >= MaxSheets {
		return nil, fmt.Errorf("xlswriter: maximum of %d sheets exceeded", MaxSheets)
	}
	sheet := &Sheet{data: worksheet{Name: name, ColumnWidths: make(map[int]float64)}}
	w.sheets = append(w.sheets, sheet)
	return sheet, nil
}

// AppendRow appends a copy of values. Cells are always text, never formulas.
// An invalid row is rejected without changing the sheet. Empty rows count.
func (s *Sheet) AppendRow(values []string) error {
	if len(s.data.Grid) >= MaxRows {
		return fmt.Errorf("xlswriter: maximum of %d rows exceeded", MaxRows)
	}
	if len(values) > MaxColumns {
		return fmt.Errorf("xlswriter: maximum of %d columns exceeded", MaxColumns)
	}
	for col, value := range values {
		if !utf8.ValidString(value) || len(utf16.Encode([]rune(value))) > MaxCellLength {
			return fmt.Errorf("xlswriter: invalid text at row %d, column %d", len(s.data.Grid), col)
		}
	}
	s.data.Grid = append(s.data.Grid, append([]string(nil), values...))
	return nil
}

// SetColumnWidth sets a zero-based column's width in characters (0–255).
// Width is rounded to the nearest 1/256 character; zero hides the column.
func (s *Sheet) SetColumnWidth(column int, width float64) error {
	if column < 0 || column >= MaxColumns || math.IsNaN(width) || math.IsInf(width, 0) || width < 0 || width > 255 {
		return fmt.Errorf("xlswriter: invalid column or width")
	}
	if s.data.ColumnWidths == nil {
		s.data.ColumnWidths = make(map[int]float64)
	}
	s.data.ColumnWidths[column] = math.Round(width*256) / 256
	return nil
}

// WriteTo writes a complete XLS file without closing dst. It implements
// io.WriterTo. At least one sheet is required. Data stays available for reuse.
// The workbook and its encoded output are held in memory.
func (w *XlsWriter) WriteTo(dst io.Writer) (int64, error) {
	if dst == nil {
		return 0, fmt.Errorf("xlswriter: nil destination")
	}
	data, err := w.encode()
	if err != nil {
		return 0, err
	}
	return data.WriteTo(dst)
}

// Save creates or truncates filename, writes the workbook, and closes the file.
// On an I/O error the file may contain partial output.
func (w *XlsWriter) Save(filename string) error {
	data, err := w.encode()
	if err != nil {
		return err
	}
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	_, writeErr := data.WriteTo(f)
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func (w *XlsWriter) encode() (*bytes.Buffer, error) {
	if len(w.sheets) == 0 {
		return nil, fmt.Errorf("xlswriter: workbook has no sheets")
	}
	sc := stringCollection{stringMap: make(map[string]int)}
	for _, sheet := range w.sheets {
		for _, row := range sheet.data.Grid {
			sc.addRow(row)
		}
	}
	sheetData := make([]string, 0, len(w.sheets))
	wb := workbook{stringCollection: &sc}
	for _, sheet := range w.sheets {
		data := sheet.data.getData(&sc)
		sheetData = append(sheetData, data)
		wb.WorksheetNames = append(wb.WorksheetNames, sheet.data.Name)
		wb.WorksheetSizes = append(wb.WorksheetSizes, len(data))
	}
	var data strings.Builder
	data.WriteString(wb.getWorksheetSizesData())
	for _, sheet := range sheetData {
		data.WriteString(sheet)
	}
	p := w.properties
	now := time.Now().Unix()
	summary := getSummaryInformation(p.Title, p.Subject, p.Creator, p.Keywords, p.Description, p.LastModifiedBy, now, now)
	entries := []pps{
		{No: 0, Name: ascToUcs("Root Entry"), PpsType: olePpsTypeRoot, PrevPps: 0xFFFFFFFF, NextPps: 0xFFFFFFFF, DirPps: 1},
		{No: 1, Name: ascToUcs("Workbook"), PpsType: olePpsTypeFile, PrevPps: 0xFFFFFFFF, NextPps: 2, DirPps: 0xFFFFFFFF, Data: data.String()},
		{No: 2, Name: ascToUcs("\x05SummaryInformation"), PpsType: olePpsTypeFile, PrevPps: 0xFFFFFFFF, NextPps: 0xFFFFFFFF, DirPps: 0xFFFFFFFF, Data: summary},
	}
	small, big, directory := calcSize(entries)
	result := new(bytes.Buffer)
	saveHeader(result, small, big, directory)
	entries[0].Data = makeSmallData(result, entries)
	saveBigData(result, small, entries)
	savePps(result, entries)
	saveBbd(result, small, big, directory, entries)
	return result, nil
}
