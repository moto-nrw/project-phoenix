package enrollmenthttp_test

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Read the actual first XLSX worksheet with the standard library. Keeping
// the assertions here does not grant Enrollment a Rendering implementation
// dependency just to decode its own HTTP response.
func holidayWorkbookRows(t *testing.T, data []byte) [][]string {
	t.Helper()
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	var shared struct {
		Items []struct {
			Text string `xml:"t"`
		} `xml:"si"`
	}
	var sheet struct {
		Rows []struct {
			Cells []struct {
				Type   string `xml:"t,attr"`
				Value  string `xml:"v"`
				Inline struct {
					Text string `xml:"t"`
				} `xml:"is"`
			} `xml:"c"`
		} `xml:"sheetData>row"`
	}
	for _, file := range archive.File {
		isSheet := strings.HasPrefix(file.Name, "xl/worksheets/sheet") && strings.HasSuffix(file.Name, ".xml")
		if file.Name != "xl/sharedStrings.xml" && !isSheet {
			continue
		}
		reader, err := file.Open()
		require.NoError(t, err)
		raw, err := io.ReadAll(reader)
		require.NoError(t, err)
		require.NoError(t, reader.Close())
		if file.Name == "xl/sharedStrings.xml" {
			require.NoError(t, xml.Unmarshal(raw, &shared))
		} else {
			require.NoError(t, xml.Unmarshal(raw, &sheet))
		}
	}
	require.NotEmpty(t, sheet.Rows)
	rows := make([][]string, len(sheet.Rows))
	for i, row := range sheet.Rows {
		for _, cell := range row.Cells {
			value := cell.Value
			switch cell.Type {
			case "s":
				index, err := strconv.Atoi(value)
				require.NoError(t, err)
				require.GreaterOrEqual(t, index, 0)
				require.Less(t, index, len(shared.Items))
				value = shared.Items[index].Text
			case "inlineStr":
				value = cell.Inline.Text
			}
			rows[i] = append(rows[i], value)
		}
	}
	return rows
}
