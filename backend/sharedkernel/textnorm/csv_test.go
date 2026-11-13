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

// A cell in NUL quotes loses them; any other NUL refuses the cell (#7009).
func TestUnquoteNULCell(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{"a": "a", "\x00a\x00": "a", "\x00\x00": "", "\x00a\"b\x00": "a\"b", "": ""} {
		got, ok := UnquoteNULCell(in)
		if !ok || got != want {
			t.Errorf("UnquoteNULCell(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"\x00", "\x00a", "a\x00", "a\x00b", "\x00a\x00b\x00"} {
		if _, ok := UnquoteNULCell(in); ok {
			t.Errorf("UnquoteNULCell(%q) accepted", in)
		}
	}
	if _, _, err := DecodeCSVText([]byte("a;\x00b\x00\n")); err == nil {
		t.Error("DecodeCSVText accepted a NUL")
	}
	if text, _, err := DecodeCSVTextWithNULQuotes([]byte("\xEF\xBB\xBFa;\x00b\x00\n")); err != nil || text != "a;\x00b\x00\n" {
		t.Errorf("DecodeCSVTextWithNULQuotes = %q, %v", text, err)
	}
	if _, _, err := DecodeCSVTextWithNULQuotes([]byte("a;\x01b\n")); err == nil {
		t.Error("DecodeCSVTextWithNULQuotes accepted another control character")
	}
}
