package domain

import (
	"bytes"
	"encoding/json"
	"image"
	"image/jpeg"
	"image/png"
	"regexp"

	"github.com/moto-nrw/project-phoenix/sharedkernel/sepa"
)

// Checks of the master data of a Träger (#4653, ADR 0112).

// phonePattern admits digits with the separators people write.
var phonePattern = regexp.MustCompile(`^\+?[0-9][0-9 ()/\-]{2,39}$`)

// CompactCode removes white space and upper-cases an IBAN or BIC as people
// type it.
func CompactCode(value string) string { return sepa.Compact(value) }

// ValidIBAN reports whether a compacted IBAN has the ISO 13616 shape, the
// length of its country and its check digits; a German one also has the
// German BBAN. The rule is the shared kernel's, the same the mandates of
// the Elternabrechnung take (#4695).
func ValidIBAN(iban string) bool { return sepa.ValidIBAN(iban) }

// ValidBIC reports whether a compacted BIC has the ISO 9362 shape.
func ValidBIC(bic string) bool { return sepa.ValidBIC(bic) }

// ValidPhone reports whether a trimmed phone number looks like one.
func ValidPhone(phone string) bool { return phonePattern.MatchString(phone) }

// LogoImage is a checked logo.
type LogoImage struct {
	Data          []byte
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
	decoded, err := decode(bytes.NewReader(data))
	if err != nil {
		return LogoImage{}, false
	}
	// The full PNG decode above guarantees the IHDR offsets: bit depth at
	// byte 24, interlace method at byte 28. Keep compatible originals intact.
	// If normalization grows past the storage limit, retain the valid original;
	// the renderer also normalizes legacy and oversized-normalization logos.
	if contentType == "image/png" && (data[24] > 8 || data[28] != 0) {
		var normalized bytes.Buffer
		if err := png.Encode(&normalized, image8Bit(decoded)); err == nil && normalized.Len() <= maxBytes {
			data = normalized.Bytes()
		}
	}
	return LogoImage{Data: data, ContentType: contentType, Width: config.Width, Height: config.Height}, true
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
	// TraegerAuditSubjectWithdrawal is the withdrawal contact (#6200).
	TraegerAuditSubjectWithdrawal = "withdrawal"
	// TraegerAuditSubjectTemplate is a document template (#4677); a save
	// creates its next version.
	TraegerAuditSubjectTemplate = "template"

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

// image8Bit only converts PNG types whose 16-bit samples gopdf cannot read.
// Keep 8-bit and palette images intact, including their alpha values.
func image8Bit(source image.Image) image.Image {
	switch source.(type) {
	case *image.Gray16, *image.RGBA64, *image.NRGBA64:
		pixels := image.NewNRGBA(source.Bounds())
		for y := pixels.Bounds().Min.Y; y < pixels.Bounds().Max.Y; y++ {
			for x := pixels.Bounds().Min.X; x < pixels.Bounds().Max.X; x++ {
				pixels.Set(x, y, source.At(x, y))
			}
		}
		return pixels
	default:
		return source
	}
}
