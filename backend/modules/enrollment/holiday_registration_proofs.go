package enrollment

import (
	"errors"
	"time"
)

// Proof of benefits in the public holiday registration (#5238): parents who
// state that they receive benefits may attach the proof at once; without it
// the Träger-Büro collects it as before. The upload follows the file field of
// the online registration (#4783): PDF, JPEG or PNG by content, at most
// MaxFileFieldSizeMB per file and MaxFilesPerField files, a single-use token
// the submission claims, unclaimed uploads expire after
// UnclaimedUploadLifetime. A proof belongs to the Träger, never to a school:
// only the detail of its registration shows it, never the list, the export
// or the hosting OGS.

// HolidayProofOptions are the limits of a proof: the types and the size of
// the file field of the online registration, several files allowed.
var HolidayProofOptions = FormFileOptions{
	AllowedTypes: []string{FileTypePDF, FileTypeJPG, FileTypePNG},
	MaxSizeMB:    MaxFileFieldSizeMB,
	Multiple:     true,
}

var (
	// ErrHolidayProofExpired reports a proof token of a submission that is
	// unknown, expired or already claimed.
	ErrHolidayProofExpired = errors.New("holiday registration proof is unknown, expired or already used")
	// ErrHolidayProofNotFound reports a proof that is not one of the
	// registration's.
	ErrHolidayProofNotFound = errors.New("holiday registration proof not found")
)

// HolidayProofUpload is one validated file a parent uploads through the
// public link. The transport detected ContentType from the bytes, checked
// that the extension names it and measured SizeBytes.
type HolidayProofUpload struct {
	Filename    string
	ContentType string
	Extension   string
	SizeBytes   int64
	Content     []byte
}

// HolidayProofReceipt answers an upload: the token the form sends with the
// submission and the file as the parent sees it.
type HolidayProofReceipt struct {
	Token       string
	Filename    string
	SizeBytes   int64
	ContentType string
}

// HolidayRegistrationProof is one claimed proof in the detail of its
// registration, without its bytes.
type HolidayRegistrationProof struct {
	ID          int64     `json:"id"`
	Filename    string    `json:"filename"`
	SizeBytes   int64     `json:"size_bytes"`
	ContentType string    `json:"content_type"`
	UploadedAt  time.Time `json:"uploaded_at"`
}

// HolidayProofFile is an opened proof with its bytes.
type HolidayProofFile struct {
	Filename    string
	ContentType string
	Content     []byte
}
