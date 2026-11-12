package textnorm

import (
	"encoding/csv"
	"slices"
	"strings"
	"testing"
)

func TestCSVDelimiterSkipsLeadingEmptyLines(t *testing.T) {
	t.Parallel()

	for _, prefix := range []string{"", "\n", "\r\n", "\n\r\n\n"} {
		for _, delimiter := range []rune{',', ';', '\t'} {
			header := []string{"first_name", "last_name"}
			text := prefix + strings.Join(header, string(delimiter)) + "\n"
			got, err := CSVDelimiter(text, "")
			if err != nil {
				t.Fatal(err)
			}
			if got != delimiter {
				t.Errorf("CSVDelimiter with prefix %q = %q, want %q", prefix, got, delimiter)
			}
			reader := csv.NewReader(strings.NewReader(text))
			reader.Comma = got
			record, err := reader.Read()
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(record, header) {
				t.Errorf("header with prefix %q and delimiter %q = %q, want %q", prefix, delimiter, record, header)
			}
		}
	}
}
