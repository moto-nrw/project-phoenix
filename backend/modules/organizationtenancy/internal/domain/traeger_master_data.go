package domain

import (
	"bytes"
	"encoding/json"
	"image"
	"image/jpeg"
	"image/png"
	"regexp"
	"strings"
	"unicode"
)

// Checks of the master data of a Träger (#4653, ADR 0112).

var (
	ibanPattern = regexp.MustCompile(`^[A-Z]{2}[0-9]{2}[A-Z0-9]{11,30}$`)
	bicPattern  = regexp.MustCompile(`^[A-Z]{6}[A-Z0-9]{2}([A-Z0-9]{3})?$`)
	// phonePattern admits digits with the separators people write.
	phonePattern = regexp.MustCompile(`^\+?[0-9][0-9 ()/\-]{2,39}$`)
)

// CompactCode removes white space and upper-cases an IBAN or BIC as people
// type it.
func CompactCode(value string) string {
	return strings.ToUpper(strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, value))
}

// ibanLengths are the ISO 13616 lengths of the SEPA countries a Träger banks
// in, the same list People Directory takes for parents' accounts. The check
// digits alone accept a truncated IBAN that happens to satisfy mod 97.
var ibanLengths = map[string]int{
	"AD": 24, "AT": 20, "BE": 16, "BG": 22, "CH": 21, "CY": 28,
	"CZ": 24, "DE": 22, "DK": 18, "EE": 20, "ES": 24, "FI": 18,
	"FR": 27, "GB": 22, "GI": 23, "GR": 27, "HR": 21, "HU": 28,
	"IE": 22, "IS": 26, "IT": 27, "LI": 21, "LT": 20, "LU": 20,
	"LV": 21, "MC": 27, "MT": 31, "NL": 18, "NO": 15, "PL": 28,
	"PT": 25, "RO": 24, "SE": 24, "SI": 19, "SK": 24,
}

// germanIBANPattern is the German BBAN: an eight-digit bank code and a
// ten-digit account number.
var germanIBANPattern = regexp.MustCompile(`^DE[0-9]{20}$`)

// ValidIBAN reports whether a compacted IBAN has the ISO 13616 shape, the
// length of its country and its check digits; a German one also has the
// German BBAN.
func ValidIBAN(iban string) bool {
	if !ibanPattern.MatchString(iban) {
		return false
	}
	if length, known := ibanLengths[iban[:2]]; !known || len(iban) != length {
		return false
	}
	if iban[:2] == "DE" && !germanIBANPattern.MatchString(iban) {
		return false
	}
	rearranged := iban[4:] + iban[:4]
	remainder := 0
	for _, r := range rearranged {
		switch {
		case r >= '0' && r <= '9':
			remainder = (remainder*10 + int(r-'0')) % 97
		default:
			remainder = (remainder*100 + int(r-'A') + 10) % 97
		}
	}
	return remainder == 1
}

// ValidBIC reports whether a compacted BIC has the ISO 9362 shape.
func ValidBIC(bic string) bool { return bicPattern.MatchString(bic) }

// ValidPhone reports whether a trimmed phone number looks like one.
func ValidPhone(phone string) bool { return phonePattern.MatchString(phone) }

// LogoImage is a checked logo.
type LogoImage struct {
	ContentType   string
	Width, Height int
}

var (
	pngSignature  = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	jpegSignature = []byte{0xff, 0xd8, 0xff}
)

// CheckLogo accepts a PNG or JPEG image of at most maxBytes whose sides do
// not exceed maxDimension. The signature names the format; the header gives
// the size before any pixel is decoded, so an oversized image is refused
// without allocating it; the full decode proves the file is a whole image.
func CheckLogo(data []byte, maxBytes, maxDimension int) (LogoImage, bool) {
	if len(data) == 0 || len(data) > maxBytes {
		return LogoImage{}, false
	}
	var contentType string
	var decodeConfig func(*bytes.Reader) (image.Config, error)
	var decode func(*bytes.Reader) (image.Image, error)
	switch {
	case bytes.HasPrefix(data, pngSignature):
		contentType = "image/png"
		decodeConfig = func(r *bytes.Reader) (image.Config, error) { return png.DecodeConfig(r) }
		decode = func(r *bytes.Reader) (image.Image, error) { return png.Decode(r) }
	case bytes.HasPrefix(data, jpegSignature):
		contentType = "image/jpeg"
		decodeConfig = func(r *bytes.Reader) (image.Config, error) { return jpeg.DecodeConfig(r) }
		decode = func(r *bytes.Reader) (image.Image, error) { return jpeg.Decode(r) }
	default:
		return LogoImage{}, false
	}
	config, err := decodeConfig(bytes.NewReader(data))
	if err != nil || config.Width <= 0 || config.Height <= 0 || config.Width > maxDimension || config.Height > maxDimension {
		return LogoImage{}, false
	}
	if _, err := decode(bytes.NewReader(data)); err != nil {
		return LogoImage{}, false
	}
	return LogoImage{ContentType: contentType, Width: config.Width, Height: config.Height}, true
}

// TraegerLogoWrite is a checked logo image.
type TraegerLogoWrite struct {
	ContentType string
	Data        []byte
	Width       int
	Height      int
}

// Subjects and actions of the master data log.
const (
	TraegerAuditSubjectMasterData  = "master_data"
	TraegerAuditSubjectContact     = "contact"
	TraegerAuditSubjectLogo        = "logo"
	TraegerAuditSubjectBankAccount = "bank_account"

	TraegerAuditActionRead   = "read"
	TraegerAuditActionSet    = "set"
	TraegerAuditActionDelete = "delete"
	TraegerAuditActionCreate = "create"
	TraegerAuditActionUpdate = "update"
)

// TraegerMasterDataAuditEntry is one entry of the master data log. SubjectID
// names the bank account; it is zero for the other subjects.
type TraegerMasterDataAuditEntry struct {
	OrganizationID int64
	Subject        string
	SubjectID      int64
	Action         string
	OldValue       json.RawMessage
	NewValue       json.RawMessage
	ActorAccountID int64
}
