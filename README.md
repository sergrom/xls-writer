[![Go Reference](https://pkg.go.dev/badge/github.com/sergrom/xls-writer.svg)](https://pkg.go.dev/github.com/sergrom/xls-writer)
[![Version](https://img.shields.io/github/v/tag/sergrom/xls-writer)](https://github.com/sergrom/xls-writer/tags)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

# xls-writer

A Go library for creating binary Excel XLS files (BIFF8), with multiple sheets,
text cells, column widths, and document properties.

The XLS encoder was extracted from `github.com/sergrom/csv2xls/v3` by copying
its source code. This library has no external Go module dependencies.

## Installation

```sh
go get github.com/sergrom/xls-writer
```

## Example

Create a workbook with European cities and save it to `cities.xls`:

```go
package main

import (
	"log"

	"github.com/sergrom/xls-writer"
)

func main() {
	w := xlswriter.New()
	w.SetProperties(xlswriter.Properties{
		Title:   "European cities",
		Creator: "Sergei",
	})

	sheet, err := w.AddSheet("Cities")
	if err != nil {
		log.Fatal(err)
	}
	if err := sheet.SetColumnWidth(0, 24); err != nil {
		log.Fatal(err)
	}

	for _, row := range [][]string{
		{"City", "Country"},
		{"Paris", "France"},
		{"Berlin", "Germany"},
		{"Madrid", "Spain"},
	} {
		if err := sheet.AppendRow(row); err != nil {
			log.Fatal(err)
		}
	}

	if err := w.Save("cities.xls"); err != nil {
		log.Fatal(err)
	}
}
```

## API

| Method | Description |
| --- | --- |
| `New()` | Creates a workbook. The zero value of `XlsWriter` is also ready to use. |
| `SetProperties(Properties)` | Replaces document properties. |
| `AddSheet(name)` | Creates a sheet with a unique, case-insensitive name. |
| `Sheet.AppendRow([]string)` | Copies and appends a row to the sheet. |
| `Sheet.SetColumnWidth(column, width)` | Sets a zero-based column's width in characters, from 0 to 255. Zero hides the column. |
| `WriteTo(io.Writer) (int64, error)` | Writes an XLS file without closing the destination. |
| `Save(filename) error` | Creates or overwrites a file and closes it. An I/O error may leave a partial file. |

`Properties` supports `Title`, `Subject`, `Creator`, `Keywords`, `Description`,
and `LastModifiedBy`. Empty fields are omitted.

## Cell values and limits

All cells are text: `001` keeps its leading zeros, and `=1+1` remains text
rather than becoming a formula. Empty strings are written as blank cells.
Rows may have different lengths; an empty row still occupies a row in the sheet.

| Item | Limit |
| --- | --- |
| Rows per sheet | 65,536 |
| Columns per sheet | 256 |
| Cell text | 32,767 UTF-16 code units |
| Sheets per workbook | 255 (current implementation limit) |
| Sheet name | 1–31 UTF-16 code units, subject to name validation |

A workbook must contain at least one sheet before it can be written.
Empty sheets are allowed. Invalid rows are rejected without changing the sheet.

## Memory and file handling

The workbook and encoded XLS output are held in memory. The same workbook
can be written repeatedly. Objects are not safe for concurrent mutation.
Creation and modification timestamps are set each time the workbook is written.

Use `WriteTo` to write to an `io.Writer`, such as a `bytes.Buffer` or an HTTP
response. The caller owns and closes the destination. Use `Save` when the
library should handle opening and closing the output file.

CSV parsing and automatic splitting across sheets are the caller's responsibility.

## Development

```sh
go test ./...
go test -race ./...
go vet ./...
```

## License

[MIT](LICENSE).
