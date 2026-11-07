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

func TestXLSXParsersRejectOversizedUnzip(t *testing.T) {
	t.Parallel()
	file := oversizedXLSX(t)

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
	for name, parse := range parsers {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := parse(bytes.NewReader(file))
			require.ErrorContains(t, err, "unzip size exceeds the 67108864 bytes limit")
		})
	}
}
