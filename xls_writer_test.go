package xlswriter

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"
)

var _ io.WriterTo = (*XlsWriter)(nil)

func TestValidation(t *testing.T) {
	w := New()
	if _, err := w.WriteTo(io.Discard); err == nil {
		t.Fatal("empty workbook accepted")
	}
	for _, name := range []string{"", " ", "a/b", "'a", "a'", strings.Repeat("a", 32), string([]byte{255})} {
		if _, err := w.AddSheet(name); err == nil {
			t.Fatalf("invalid name accepted: %q", name)
		}
	}
	s, err := w.AddSheet("Data")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.AddSheet("DATA"); err == nil {
		t.Fatal("duplicate accepted")
	}
	row := []string{"001", "=1+1", "Hello 🐧"}
	if err := s.AppendRow(row); err != nil {
		t.Fatal(err)
	}
	row[0] = "changed"
	if s.data.Grid[0][0] != "001" {
		t.Fatal("row not copied")
	}
	for _, row := range [][]string{make([]string, 257), {strings.Repeat("🐧", 16384)}, {string([]byte{255})}} {
		if err := s.AppendRow(row); err == nil {
			t.Fatal("invalid row accepted")
		}
	}
	if len(s.data.Grid) != 1 {
		t.Fatal("invalid rows mutated sheet")
	}
	if err := s.SetColumnWidth(255, 12.5); err != nil {
		t.Fatal(err)
	}
	if err := s.SetColumnWidth(256, 12); err == nil {
		t.Fatal("invalid column accepted")
	}
	if err := s.SetColumnWidth(0, -1); err == nil {
		t.Fatal("invalid width accepted")
	}
	for len(s.data.Grid) < MaxRows {
		if err := s.AppendRow(nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.AppendRow(nil); err == nil {
		t.Fatal("row overflow accepted")
	}
	for len(w.sheets) < MaxSheets {
		_, err := w.AddSheet(fmt.Sprintf("sheet%d", len(w.sheets)))
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := w.AddSheet("overflow"); err == nil {
		t.Fatal("sheet overflow accepted")
	}
}

type failedWriter struct{ err error }

func (f failedWriter) Write(p []byte) (int, error) { return 7, f.err }

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) { return 1, nil }

func TestOutput(t *testing.T) {
	w := New()
	s, _ := w.AddSheet("Sheet 🐧")
	_ = s.AppendRow([]string{"001", "Hello 🐧", strings.Repeat("é", 32767)})
	w.SetProperties(Properties{Title: "Report 🐧"})
	var buf bytes.Buffer
	n, err := w.WriteTo(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if n != int64(buf.Len()) || !bytes.Equal(buf.Bytes()[:8], []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}) {
		t.Fatal("invalid output")
	}
	marker := errors.New("destination failed")
	n, err = w.WriteTo(failedWriter{marker})
	if n != 7 || !errors.Is(err, marker) {
		t.Fatalf("lost write error: %d %v", n, err)
	}
	if _, err = w.WriteTo(shortWriter{}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short write: %v", err)
	}
	filename := filepath.Join(t.TempDir(), "report.xls")
	if err = w.Save(filename); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filename)
	if err != nil || len(data) != buf.Len() {
		t.Fatal("save failed", err)
	}
	if err = New().Save(filename); err == nil {
		t.Fatal("empty workbook saved")
	}
	after, _ := os.ReadFile(filename)
	if !bytes.Equal(data, after) {
		t.Fatal("validation truncated file")
	}
	if _, err = w.WriteTo(io.Discard); err != nil {
		t.Fatal("repeat write failed", err)
	}
}

// Large metadata and workbook streams must have separate, terminated FAT chains.
func TestLargeStreamChains(t *testing.T) {
	w := New()
	w.SetProperties(Properties{Description: strings.Repeat("é", 5000)})
	s, _ := w.AddSheet("Data")
	_ = s.AppendRow([]string{strings.Repeat("🐧", 4000)})
	buf, err := w.encode()
	if err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	u32 := func(offset int) uint32 { return binary.LittleEndian.Uint32(data[offset:]) }
	fatSector := u32(76)
	dirSector := u32(48)
	for entry := 1; entry <= 2; entry++ {
		offset := 512 + int(dirSector)*512 + entry*128
		first, size := u32(offset+116), u32(offset+120)
		block := first
		for i := uint32(0); i < (size+511)/512; i++ {
			if block >= 128 {
				t.Fatal("fixture exceeded one FAT sector")
			}
			next := u32(512 + int(fatSector)*512 + int(block)*4)
			if i+1 == (size+511)/512 && next != 0xFFFFFFFE {
				t.Fatalf("stream %d not terminated", entry)
			}
			block = next
		}
	}
}

func TestUnicodeContinuation(t *testing.T) {
	value := strings.Repeat("🐧", 16000)
	sc := stringCollection{stringMap: make(map[string]int)}
	sc.addRow([]string{value})
	wb := workbook{stringCollection: &sc}
	var out bytes.Buffer
	wb.writeSharedStringsTable(&out)
	data := out.Bytes()
	var units []uint16
	first := true
	for len(data) > 0 {
		n := int(binary.LittleEndian.Uint16(data[2:4]))
		if n > 8224 {
			t.Fatal("oversized record")
		}
		chunk := data[4 : 4+n]
		data = data[4+n:]
		if first {
			chunk = chunk[11:]
			first = false
		} else {
			if chunk[0] != 1 {
				t.Fatal("missing Unicode flag")
			}
			chunk = chunk[1:]
		}
		if len(chunk)%2 != 0 {
			t.Fatal("split UTF-16 unit")
		}
		for len(chunk) > 0 {
			units = append(units, binary.LittleEndian.Uint16(chunk))
			chunk = chunk[2:]
		}
		if len(units) > 0 && units[len(units)-1] >= 0xD800 && units[len(units)-1] <= 0xDBFF {
			t.Fatal("split surrogate pair")
		}
	}
	if string(utf16.Decode(units)) != value {
		t.Fatal("corrupt continued string")
	}
}
