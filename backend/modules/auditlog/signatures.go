package auditlog

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/png"
	"strings"
	"time"
	"unicode/utf8"
)

// Signatures (#4785): the shared building block for a simple electronic
// signature with a drawn signature. A consumer (Einverständnis, enrollment,
// pickup permission) decides who may sign what and keeps the reference; the
// Audit Platform stores the image with its signer, time and the checksum of
// the signed wording, append-only and bound to the school. The ledger never
// decides authorization: it only stores and returns rows of the caller's
// school.

// Signature purposes name the consumer; the database refuses any other.
const (
	SignaturePurposeDeclaration      = "declaration"
	SignaturePurposeEnrollment       = "enrollment"
	SignaturePurposePickupPermission = "pickup_permission"
)

// Signature methods: drawn with finger, pen or mouse, or the typed name the
// client rendered into the same image (the keyboard alternative).
const (
	SignatureMethodDrawn = "drawn"
	SignatureMethodTyped = "typed"
)

// Image limits. The client draws on a canvas of at most these dimensions; a
// drawn signature is a few kilobytes, the limit leaves room for high-density
// screens without letting the column grow into an upload store.
const (
	SignatureImageMaxBytes  = 200 << 10
	SignatureImageMaxWidth  = 1600
	SignatureImageMaxHeight = 800
	SignatureTypedNameMin   = 2
	SignatureTypedNameMax   = 120
)

// ErrSignatureInvalid reports an image or typed name that does not pass the
// checks: not a PNG, too large, empty, or a typed signature without a name.
var ErrSignatureInvalid = errors.New("signature: invalid signature")

var pngMagic = []byte("\x89PNG\r\n\x1a\n")

// SignatureImage is a checked signature as the signer gave it.
type SignatureImage struct {
	Method    string
	PNG       []byte
	TypedName string
}

// ParseSignatureImage checks a signature from the wire: the method, the
// base64 PNG (magic number, size, dimensions, at least one visible pixel)
// and, for a typed signature, the typed name. Every failure wraps
// ErrSignatureInvalid.
func ParseSignatureImage(method, pngBase64, typedName string) (SignatureImage, error) {
	pngBase64 = strings.TrimSpace(pngBase64)
	if base64.StdEncoding.DecodedLen(len(pngBase64)) > SignatureImageMaxBytes+3 {
		return SignatureImage{}, fmt.Errorf("%w: image exceeds %d bytes", ErrSignatureInvalid, SignatureImageMaxBytes)
	}
	data, err := base64.StdEncoding.DecodeString(pngBase64)
	if err != nil {
		return SignatureImage{}, fmt.Errorf("%w: image is not base64", ErrSignatureInvalid)
	}
	return ParseSignatureImageBytes(method, data, typedName)
}

// ParseSignatureImageBytes is ParseSignatureImage for already decoded bytes.
func ParseSignatureImageBytes(method string, data []byte, typedName string) (SignatureImage, error) {
	if method != SignatureMethodDrawn && method != SignatureMethodTyped {
		return SignatureImage{}, fmt.Errorf("%w: unknown method %q", ErrSignatureInvalid, method)
	}
	typedName = strings.TrimSpace(typedName)
	if method == SignatureMethodTyped {
		if n := utf8.RuneCountInString(typedName); n < SignatureTypedNameMin || n > SignatureTypedNameMax {
			return SignatureImage{}, fmt.Errorf("%w: typed name must have %d to %d characters", ErrSignatureInvalid, SignatureTypedNameMin, SignatureTypedNameMax)
		}
	} else {
		typedName = ""
	}
	if err := checkSignaturePNG(data); err != nil {
		return SignatureImage{}, err
	}
	return SignatureImage{Method: method, PNG: data, TypedName: typedName}, nil
}

func checkSignaturePNG(data []byte) error {
	if len(data) == 0 || len(data) > SignatureImageMaxBytes {
		return fmt.Errorf("%w: image must have 1 to %d bytes", ErrSignatureInvalid, SignatureImageMaxBytes)
	}
	if !bytes.HasPrefix(data, pngMagic) {
		return fmt.Errorf("%w: image is not a PNG", ErrSignatureInvalid)
	}
	config, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("%w: image header: %v", ErrSignatureInvalid, err)
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > SignatureImageMaxWidth || config.Height > SignatureImageMaxHeight {
		return fmt.Errorf("%w: image must be at most %dx%d pixels", ErrSignatureInvalid, SignatureImageMaxWidth, SignatureImageMaxHeight)
	}
	decoded, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("%w: image data: %v", ErrSignatureInvalid, err)
	}
	if !hasInk(decoded) {
		return fmt.Errorf("%w: image is empty", ErrSignatureInvalid)
	}
	return nil
}

// hasInk reports whether any pixel is visible and not white: an untouched
// canvas is transparent or white.
func hasInk(img image.Image) bool {
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			if a > 0x1000 && (r < 0xE000 || g < 0xE000 || b < 0xE000) {
				return true
			}
		}
	}
	return false
}

// SignatureEntry is one signature to store. DocumentSHA256 is the checksum of
// exactly what was signed (for an Einverständnis the content hash of the
// frozen version); the consumer computes it. Exactly one subject binds the
// signature's lifecycle: StudentID for a flow about a child (declaration,
// pickup permission), EnrollmentRequestID for an enrollment that has no
// child yet. Deleting the subject deletes the signature. SignerAccountID is
// nil for a public enrollment without an account. SignedAt is truncated to
// microseconds, the precision PostgreSQL keeps, so the record hash stays
// verifiable.
type SignatureEntry struct {
	Purpose             string
	StudentID           *int64
	EnrollmentRequestID *int64
	SignerAccountID     *int64
	SignerName          string
	DocumentSHA256      string
	SignedAt            time.Time
	Image               SignatureImage
}

// Signature is one stored signature. IntegrityOK is false when the image or
// the row no longer matches its stored checksum.
type Signature struct {
	ID                  int64
	TenantID            int64
	Purpose             string
	StudentID           *int64
	EnrollmentRequestID *int64
	SignerAccountID     *int64
	SignerName          string
	Method              string
	TypedName           string
	PNG                 []byte
	ImageSHA256         string
	DocumentSHA256      string
	RecordHash          string
	SignedAt            time.Time
	IntegrityOK         bool
}

// SignatureLedger appends and reads signatures inside the caller's tenant
// transaction, so a signature commits or rolls back with the record that
// references it. Reads return only rows of that school; ids of another school
// are simply absent.
type SignatureLedger interface {
	RecordSignature(context.Context, SignatureEntry) (*Signature, error)
	Signatures(ctx context.Context, ids []int64) (map[int64]*Signature, error)
}
