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

// A NUL in the role of a quote becomes a quote, and a quote inside such a
// field is doubled; a NUL elsewhere stays (#7009): inside a field of another
// delimiter and inside an ordinary quoted field too.
func TestQuoteNULDelimiters(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"\x00a\x00;\x00\x00\r\n\x00b\"c\x00": "\"a\";\"\"\r\n\"b\"\"c\"",
		"\xEF\xBB\xBF\x00a\x00;b":            "\"a\";b",
		"a;b\n":                              "a;b\n",
		"a\x00b;c":                           "a\x00b;c",
		"\x00a;b":                            "\x00a;b",
		"x;prefix,\x00value\x00,suffix":      "x;prefix,\x00value\x00,suffix",
		"\"a\x00\x00b\";\x00c\x00":           "\"a\x00\x00b\";\"c\"",
		"\"a\"\"\x00\";b":                    "\"a\"\"\x00\";b",
		"x;\"a\"b;\x00value\x00;suffix\"\n":  "x;\"a\"b;\x00value\x00;suffix\"\n",
	} {
		if got := string(QuoteNULDelimiters([]byte(in), ';')); got != want {
			t.Errorf("QuoteNULDelimiters(%q) = %q, want %q", in, got, want)
		}
	}
}
