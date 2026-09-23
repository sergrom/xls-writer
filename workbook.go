package xlswriter

import (
	"bytes"
	"encoding/binary"
)

// rgb ...
type rgb struct {
	index       int
	red         uint8
	green       uint8
	blue        uint8
	transparent uint8
}

var wbPalette = []rgb{
	rgb{0x08, 0x00, 0x00, 0x00, 0x00},
	rgb{0x09, 0xff, 0xff, 0xff, 0x00},
	rgb{0x0A, 0xff, 0x00, 0x00, 0x00},
	rgb{0x0B, 0x00, 0xff, 0x00, 0x00},
	rgb{0x0C, 0x00, 0x00, 0xff, 0x00},
	rgb{0x0D, 0xff, 0xff, 0x00, 0x00},
	rgb{0x0E, 0xff, 0x00, 0xff, 0x00},
	rgb{0x0F, 0x00, 0xff, 0xff, 0x00},
	rgb{0x10, 0x80, 0x00, 0x00, 0x00},
	rgb{0x11, 0x00, 0x80, 0x00, 0x00},
	rgb{0x12, 0x00, 0x00, 0x80, 0x00},
	rgb{0x13, 0x80, 0x80, 0x00, 0x00},
	rgb{0x14, 0x80, 0x00, 0x80, 0x00},
	rgb{0x15, 0x00, 0x80, 0x80, 0x00},
	rgb{0x16, 0xc0, 0xc0, 0xc0, 0x00},
	rgb{0x17, 0x80, 0x80, 0x80, 0x00},
	rgb{0x18, 0x99, 0x99, 0xff, 0x00},
	rgb{0x19, 0x99, 0x33, 0x66, 0x00},
	rgb{0x1A, 0xff, 0xff, 0xcc, 0x00},
	rgb{0x1B, 0xcc, 0xff, 0xff, 0x00},
	rgb{0x1C, 0x66, 0x00, 0x66, 0x00},
	rgb{0x1D, 0xff, 0x80, 0x80, 0x00},
	rgb{0x1E, 0x00, 0x66, 0xcc, 0x00},
	rgb{0x1F, 0xcc, 0xcc, 0xff, 0x00},
	rgb{0x20, 0x00, 0x00, 0x80, 0x00},
	rgb{0x21, 0xff, 0x00, 0xff, 0x00},
	rgb{0x22, 0xff, 0xff, 0x00, 0x00},
	rgb{0x23, 0x00, 0xff, 0xff, 0x00},
	rgb{0x24, 0x80, 0x00, 0x80, 0x00},
	rgb{0x25, 0x80, 0x00, 0x00, 0x00},
	rgb{0x26, 0x00, 0x80, 0x80, 0x00},
	rgb{0x27, 0x00, 0x00, 0xff, 0x00},
	rgb{0x28, 0x00, 0xcc, 0xff, 0x00},
	rgb{0x29, 0xcc, 0xff, 0xff, 0x00},
	rgb{0x2A, 0xcc, 0xff, 0xcc, 0x00},
	rgb{0x2B, 0xff, 0xff, 0x99, 0x00},
	rgb{0x2C, 0x99, 0xcc, 0xff, 0x00},
	rgb{0x2D, 0xff, 0x99, 0xcc, 0x00},
	rgb{0x2E, 0xcc, 0x99, 0xff, 0x00},
	rgb{0x2F, 0xff, 0xcc, 0x99, 0x00},
	rgb{0x30, 0x33, 0x66, 0xff, 0x00},
	rgb{0x31, 0x33, 0xcc, 0xcc, 0x00},
	rgb{0x32, 0x99, 0xcc, 0x00, 0x00},
	rgb{0x33, 0xff, 0xcc, 0x00, 0x00},
	rgb{0x34, 0xff, 0x99, 0x00, 0x00},
	rgb{0x35, 0xff, 0x66, 0x00, 0x00},
	rgb{0x36, 0x66, 0x66, 0x99, 0x00},
	rgb{0x37, 0x96, 0x96, 0x96, 0x00},
	rgb{0x38, 0x00, 0x33, 0x66, 0x00},
	rgb{0x39, 0x33, 0x99, 0x66, 0x00},
	rgb{0x3A, 0x00, 0x33, 0x00, 0x00},
	rgb{0x3B, 0x33, 0x33, 0x00, 0x00},
	rgb{0x3C, 0x99, 0x33, 0x00, 0x00},
	rgb{0x3D, 0x99, 0x33, 0x66, 0x00},
	rgb{0x3E, 0x33, 0x33, 0x99, 0x00},
	rgb{0x3F, 0x33, 0x33, 0x33, 0x00},
}

// workbook ...
type workbook struct {
	WorksheetSizes   []int
	WorksheetNames   []string
	stringCollection *stringCollection
}

func (wb *workbook) getWorksheetSizesData() string {
	buf := new(bytes.Buffer)

	// Calculate the number of selected worksheet tabs and call the finalization
	// methods for each worksheet
	totalWorksheets := len(wb.WorksheetSizes)

	// Add part 1 of the workbook globals, what goes before the SHEET records
	wb.storeBof(buf)
	wb.writeCodepage(buf)
	wb.writeWindow1(buf)

	wb.writeDateMode(buf)
	wb.writeAllFonts(buf)
	wb.writeAllNumberFormats(buf)
	wb.writeAllXfs(buf)
	wb.writeAllStyles(buf)
	wb.writePalette(buf)

	// Prepare part 3 of the workbook global stream, what goes after the SHEET records
	part3Buf := new(bytes.Buffer)

	wb.writeRecalcId(part3Buf)

	wb.writeSupbookInternal(part3Buf, totalWorksheets)
	/* TODO: store external SUPBOOK records and XCT and CRN records
	   in case of external references for BIFF8 */
	wb.writeExternalsheetBiff8(part3Buf, totalWorksheets)
	wb.writeAllDefinedNamesBiff8(part3Buf)
	wb.writeMsoDrawingGroup(part3Buf)
	wb.writeSharedStringsTable(part3Buf)

	wb.writeEof(part3Buf)

	// Add part 2 of the workbook globals, the SHEET records
	worksheetOffsets := wb.calcSheetOffsets(buf.Len()+part3Buf.Len(), totalWorksheets)
	for i := 0; i < totalWorksheets; i++ {
		wb.writeBoundSheet(buf, wb.WorksheetNames[i], worksheetOffsets[i])
	}

	// Add part 3 of the workbook globals
	buf.Write(part3Buf.Bytes())

	return buf.String()
}

func (wb *workbook) storeBof(buffer *bytes.Buffer) {
	var wbType uint16 = 0x0005

	var record uint16 = 0x0809 // Record identifier    (BIFF5-BIFF8)
	var length uint16 = 0x0010

	var build uint16 = 0x0DBB //    Excel 97
	var year uint16 = 0x07CC  //    Excel 97

	var version uint16 = 0x0600 //    BIFF8

	putVar(buffer, record, length)
	putVar(buffer, version, wbType, build, year)

	// by inspection of real files, MS Office Excel 2007 writes the following
	putVar(buffer, uint32(0x000100D1), uint32(0x00000406))
}

func (wb *workbook) writeCodepage(buffer *bytes.Buffer) {
	var record uint16 = 0x0042 // Record identifier
	var length uint16 = 0x0002 // Number of bytes to follow
	var cv uint16 = 0x04B0     // The code page

	putVar(buffer, record, length, cv)
}

func (wb *workbook) writeWindow1(buffer *bytes.Buffer) {
	var record uint16 = 0x003D // Record identifier
	var length uint16 = 0x0012 // Number of bytes to follow

	var xWn uint16 = 0x0000  // Horizontal position of window
	var yWn uint16 = 0x0000  // Vertical position of window
	var dxWn uint16 = 0x25BC // Width of window
	var dyWn uint16 = 0x1572 // Height of window

	var grbit uint16 = 0x0038 // Option flags

	// not supported by PhpSpreadsheet, so there is only one selected sheet, the active
	var ctabsel uint16 = 1 // Number of workbook tabs selected

	var wTabRatio uint16 = 0x0258 // Tab to scrollbar ratio

	// not supported by PhpSpreadsheet, set to 0
	var itabFirst uint16 = 0 // 1st displayed worksheet
	var itabCur uint16 = 0   // Active worksheet

	putVar(buffer, record, length)
	putVar(buffer, xWn, yWn, dxWn, dyWn, grbit, itabCur, itabFirst, ctabsel, wTabRatio)
}

func (wb *workbook) writeDateMode(buffer *bytes.Buffer) {
	var record uint16 = 0x0022 // Record identifier
	var length uint16 = 0x0002 // Bytes to follow

	var f1904 uint16 = 0 // Flag for 1904 date system

	putVar(buffer, record, length, f1904)
}

func (wb *workbook) writeAllFonts(buffer *bytes.Buffer) {
	var icv uint16 = 8 // Index to color palette
	var sss uint16 = 0

	var bFamily uint8 = 0     // Font family
	var bCharSet uint8 = 0x00 // Character set
	var record uint16 = 0x31  // Record identifier
	var reserved uint8 = 0x00 // Reserved
	var grbit uint16 = 0x00   // Font attributes

	dataBuf := new(bytes.Buffer)

	var fontSize uint16 = 11
	putVar(dataBuf,
		fontSize*20,
		grbit,
		icv,           // Colour
		uint16(0x190), // Font weight (0x190=400=normal)
		sss,           // Superscript/Subscript
		uint8(0x00),   // Underline
		bFamily,
		bCharSet,
		reserved,
		[]byte(utf8toBIFF8UnicodeShort("Calibri")),
	)

	putVar(buffer, record, uint16(dataBuf.Len()))
	buffer.Write(dataBuf.Bytes())
}

func (wb *workbook) writeAllNumberFormats(buffer *bytes.Buffer) {
	// empty
}

func (wb *workbook) writeAllXfs(buffer *bytes.Buffer) {
	var record uint16 = 0x00E0 // Record identifier
	var length uint16 = 0x0014 // Number of bytes to follow

	for i := 0; i < 15; i++ {
		putVar(buffer, record, length)
		putVar(buffer, uint16(0), uint16(0), uint16(0xFFF5), uint8(32))
		putVar(buffer, uint8(0), uint8(0), uint8(0xC0))
		putVar(buffer, uint32(0), uint32(0), uint16(1033))
	}

	putVar(buffer, record, length)
	putVar(buffer, uint16(0), uint16(0), uint16(1), uint8(32))
	putVar(buffer, uint8(0), uint8(0), uint8(0xC0))
	putVar(buffer, uint32(0), uint32(0), uint16(1033))
}

func (wb *workbook) writeAllStyles(buffer *bytes.Buffer) {
	var record uint16 = 0x0293 // Record identifier
	var length uint16 = 0x0004 // Bytes to follow

	var ixfe uint16 = 0x8000 // Index to cell style XF
	var BuiltIn uint8 = 0x00 // Built-in style
	var iLevel uint8 = 0xff  // Outline style level

	putVar(buffer, record, length)
	putVar(buffer, ixfe, BuiltIn, iLevel)
}

func (wb *workbook) writePalette(buffer *bytes.Buffer) {
	var record uint16 = 0x0092     // Record identifier
	length := 2 + 4*len(wbPalette) // Number of bytes to follow
	ccv := len(wbPalette)          // Number of RGB values to follow

	putVar(buffer, record, uint16(length), uint16(ccv))

	// Pack the RGB data
	for _, color := range wbPalette {
		putVar(buffer, color.red, color.green, color.blue, color.transparent)
	}
}

func (wb *workbook) writeRecalcId(buffer *bytes.Buffer) {
	var record uint16 = 0x01C1 // Record identifier
	var length uint16 = 8      // Number of bytes to follow

	putVar(buffer, record, length)

	// by inspection of real Excel files, MS Office Excel 2007 writes this
	putVar(buffer, uint32(0x000001C1), uint32(0x00001E667))
}

func (wb *workbook) writeSupbookInternal(buffer *bytes.Buffer, totalWorksheets int) {
	var record uint16 = 0x01AE // Record identifier
	var length uint16 = 0x0004 // Bytes to follow

	putVar(buffer, record, length)
	putVar(buffer, uint16(totalWorksheets), uint16(0x0401))
}

func (wb *workbook) writeExternalsheetBiff8(buffer *bytes.Buffer, totalWorksheets int) {
	if totalWorksheets > 255 {
		panic("Too many worksheets")
	}

	tmpBuf := new(bytes.Buffer)

	cWorksheets := uint16(totalWorksheets)

	var record uint16 = 0x0017            // Record identifier
	var length uint16 = 2 + 6*cWorksheets // Number of bytes to follow

	putVar(tmpBuf, record, length)
	putVar(tmpBuf, cWorksheets)

	var i uint16
	for i = 0; i < cWorksheets; i++ {
		putVar(tmpBuf, uint16(0x00), i, i)
	}

	wb.writeData(buffer, tmpBuf)
}

func (wb *workbook) writeAllDefinedNamesBiff8(buffer *bytes.Buffer) {
	// empty
}

func (wb *workbook) writeMsoDrawingGroup(buffer *bytes.Buffer) {
	// empty
}

func (wb *workbook) writeData(bufferTo *bytes.Buffer, bufferFrom *bytes.Buffer) {
	if bufferFrom.Len()-4 > 8224 {
		wb.addContinue(bufferTo, bufferFrom)
		return
	}

	bufferTo.Write(bufferFrom.Bytes())
}

func (wb *workbook) addContinue(bufferTo *bytes.Buffer, bufferFrom *bytes.Buffer) {
	var limit uint16 = 8224
	var record uint16 = 0x003C // Record identifier

	putVar(bufferTo, substr(bufferFrom.Bytes(), 0, 2), limit, substr(bufferFrom.Bytes(), 4, 8224))

	bufFromLength := bufferFrom.Len()

	var i int
	for i = int(limit + 4); i < (bufFromLength - int(limit)); i += int(limit) {
		putVar(bufferTo, record, limit)
		putVar(bufferTo, substr(bufferFrom.Bytes(), i, int(limit)))
	}

	// Retrieve the last chunk of data
	putVar(bufferTo, record, uint16(bufferFrom.Len()-i), bufferFrom.Bytes()[i:])
}

func (wb *workbook) writeSharedStringsTable(buffer *bytes.Buffer) {
	const limit = 8224
	block := new(bytes.Buffer)
	record := uint16(0x00FC)
	flush := func() {
		putVar(buffer, record, uint16(block.Len()), block.Bytes())
		block.Reset()
		record = 0x003C
	}
	putVar(block, uint32(wb.stringCollection.stringTotal), uint32(wb.stringCollection.stringUnique))
	for _, encoded := range wb.stringCollection.stringList {
		// Keep the string header and first character together.
		if limit-block.Len() < 7 {
			flush()
		}
		block.WriteString(encoded[:3])
		remaining := []byte(encoded[3:])
		for len(remaining) > 0 {
			count := (limit - block.Len()) / 2 * 2
			if count > len(remaining) {
				count = len(remaining)
			}
			// Keep surrogate pairs together, including across CONTINUE records.
			if count > 0 && count < len(remaining) {
				last := binary.LittleEndian.Uint16(remaining[count-2 : count])
				if last >= 0xD800 && last <= 0xDBFF {
					count -= 2
				}
			}
			block.Write(remaining[:count])
			remaining = remaining[count:]
			if len(remaining) > 0 {
				flush()
				block.WriteByte(1)
			}
		}
	}
	if block.Len() > 0 {
		flush()
	}
}

func (wb *workbook) writeEof(buffer *bytes.Buffer) {
	var record uint16 = 0x000A // Record identifier
	var length uint16 = 0x0000 // Number of bytes to follow

	putVar(buffer, record, length)
}

func (wb *workbook) calcSheetOffsets(dataSize int, totalWorksheets int) []uint32 {
	worksheetOffsets := make([]uint32, 0)
	boundSheetLength := 10 // fixed length for a BOUNDSHEET record

	// size of workbook globals part 1 + 3
	offset := dataSize

	// add size of workbook globals part 2, the length of the SHEET records
	for _, sheetTitle := range wb.WorksheetNames {
		offset += boundSheetLength + len(utf8toBIFF8UnicodeShort(sheetTitle))
	}

	// add the sizes of each of the Sheet substreams, respectively
	for i := 0; i < totalWorksheets; i++ {
		worksheetOffsets = append(worksheetOffsets, uint32(offset))
		offset += wb.WorksheetSizes[i]
	}

	return worksheetOffsets
}

func (wb *workbook) writeBoundSheet(buffer *bytes.Buffer, sheetName string, offset uint32) {
	var record uint16 = 0x0085 // Record identifier
	var ss uint8 = 0x00

	// sheet type
	var st uint8 = 0x00

	biff8SheetName := utf8toBIFF8UnicodeShort(sheetName)
	length := 6 + len(biff8SheetName)

	putVar(buffer, record, uint16(length))
	putVar(buffer, offset, ss, st)
	putVar(buffer, []byte(biff8SheetName))
}
