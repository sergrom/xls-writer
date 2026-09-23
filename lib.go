package xlswriter

import (
	"bytes"
	"encoding/binary"
	"unicode/utf16"
)

// padRight ...
func padRight(str, pad string, lenght int) string {
	for {
		str += pad
		if len(str) > lenght {
			return str[0:lenght]
		}
	}
}

// putVar ...
// putVar only writes fixed-size internal values into an in-memory buffer.
func putVar(w *bytes.Buffer, args ...interface{}) {
	for _, value := range args {
		if err := binary.Write(w, binary.LittleEndian, value); err != nil {
			panic(err)
		}
	}
}

// localDateToOLE ...
func localDateToOLE(timestamp int64) string {
	buf := new(bytes.Buffer)
	putVar(buf, uint64(timestamp+11644473600)*10000000)
	return buf.String()
}

// ascToUcs utility function to transform ASCII text to Unicode.
func ascToUcs(ascii string) string {
	buf := new(bytes.Buffer)
	for i := 0; i < len(ascii); i++ {
		putVar(buf, ascii[i], []byte("\x00"))
	}

	return buf.String()
}

// utf8toBIFF8UnicodeShort converts a UTF-8 string into BIFF8 Unicode string data (8-bit string length)
func utf8toBIFF8UnicodeShort(value string) string {
	buf := new(bytes.Buffer)
	utf16str := utf16.Encode([]rune(value))
	putVar(buf, uint8(len(utf16str)), uint8(0x0001), utf16str)

	return buf.String()
}

// utf8toBIFF8UnicodeLong converts a UTF-8 string into BIFF8 Unicode string data (16-bit string length)
func utf8toBIFF8UnicodeLong(value string) string {
	buf := new(bytes.Buffer)
	utf16str := utf16.Encode([]rune(value))
	putVar(buf, uint16(len(utf16str)), uint8(0x0001), utf16str)

	return buf.String()
}

// max returns the larger of x or y.
func max(x, y int) int {
	if x < y {
		return y
	}
	return x
}

// maxUInt16 ...
func maxUInt16(x, y uint16) uint16 {
	if x < y {
		return y
	}
	return x
}

// minUInt16 ...
func minUInt16(x, y uint16) uint16 {
	if x > y {
		return y
	}
	return x
}

func substr(slice []byte, start, length int) []byte {
	return slice[start : start+length]
}
