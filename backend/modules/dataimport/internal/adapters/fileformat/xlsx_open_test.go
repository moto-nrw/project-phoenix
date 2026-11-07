package fileformat

import (
	"archive/zip"
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

// oversizedXLSX returns a valid workbook plus one entry that declares more
// uncompressed bytes than the unzip limit. archive/zip refuses to read past a
// declared size, so the declared size is what bounds decompression; the test
// needs no large allocation.
func oversizedXLSX(t *testing.T) []byte {
	t.Helper()
	workbook, err := io.ReadAll(buildStaffXLSX(t, [][]string{{"Vorname", "Nachname"}}))
	require.NoError(t, err)
	source, err := zip.NewReader(bytes.NewReader(workbook), int64(len(workbook)))
	require.NoError(t, err)

	out := new(bytes.Buffer)
	w := zip.NewWriter(out)
	for _, entry := range source.File {
		require.NoError(t, w.Copy(entry))
	}
	_, err = w.CreateRaw(&zip.FileHeader{
		Name:               "xl/media/padding.bin",
		Method:             zip.Store,
		UncompressedSize64: xlsxUnzipSizeLimit + 1,
	})
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return out.Bytes()
}

// sparseWideXLSX returns a workbook of a few KB whose rows each hold one value
// in the last column (XFD), which excelize pads to 16,384 cells per row.
func sparseWideXLSX(t *testing.T) []byte {
	t.Helper()
	f, err := newExcelSheet("Sheet1")
	require.NoError(t, err)
	sheet := f.GetSheetName(0)
	rows := xlsxCellBudget/16384 + 1
	for r := 1; r <= rows; r++ {
		cell, err := excelCellName(16384, r)
		require.NoError(t, err)
		require.NoError(t, f.SetCellStr(sheet, cell, "x"))
	}
	buf := new(bytes.Buffer)
	require.NoError(t, f.Write(buf))
	return buf.Bytes()
}

func TestXLSXParsersRejectDecompressionBombs(t *testing.T) {
	t.Parallel()
	files := map[string][]byte{
		"unzip size exceeds the 67108864 bytes limit": oversizedXLSX(t),
		errXLSXTooManyCells.Error():                   sparseWideXLSX(t),
	}

	parsers := map[string]func(io.Reader) error{
		"students": func(r io.Reader) error { _, err := NewXLSXParser().ParseStudents(r); return err },
		"staff":    func(r io.Reader) error { _, err := ParseStaffXLSX(r); return err },
		"class list": func(r io.Reader) error {
			_, err := ParseClassListXLSX(r)
			return err
		},
		"opening balances": func(r io.Reader) error {
			_, err := ParseOpeningBalanceXLSX(r)
			return err
		},
	}
	for want, file := range files {
		for name, parse := range parsers {
			t.Run(name+"/"+want, func(t *testing.T) {
				t.Parallel()
				err := parse(bytes.NewReader(file))
				require.ErrorContains(t, err, want)
			})
		}
	}
}
