package textnorm

import (
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// ContentDisposition builds a Content-Disposition header value that keeps a
// file name with accents (#5791). filename carries the ASCII spelling that
// every client understands; when the name is not plain ASCII, filename*
// (RFC 6266, RFC 8187) carries it in UTF-8, and browsers prefer that one.
// Characters that could end the quoted value or the header become "_" in
// the ASCII spelling and are percent-encoded in filename*.
func ContentDisposition(disposition, filename string) string {
	filename = norm.NFC.String(filename)
	if filename == "" {
		return disposition
	}
	fallback := asciiFilename(filename)
	value := disposition + `; filename="` + fallback + `"`
	if fallback != filename {
		value += "; filename*=UTF-8''" + percentEncode(filename)
	}
	return value
}

// asciiFilename transliterates the name and replaces what may not stand in a
// quoted header value: non-ASCII, control characters, quote and backslash,
// and "%", which some browsers decode in filename.
func asciiFilename(filename string) string {
	var b strings.Builder
	for _, r := range ASCII(filename) {
		if r >= utf8.RuneSelf || r < 0x20 || r == 0x7f || r == '"' || r == '\\' || r == '%' {
			b.WriteByte('_')
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// percentEncode encodes every byte outside RFC 8187's attr-char.
func percentEncode(value string) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(value); i++ {
		c := value[i]
		if isAttrChar(c) {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(hex[c>>4])
		b.WriteByte(hex[c&0x0f])
	}
	return b.String()
}

func isAttrChar(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return true
	}
	return strings.IndexByte("!#$&+-.^_`|~", c) >= 0
}
