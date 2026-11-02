package enrollment

import "errors"

// HolidayFormDocument is a PDF owned by one Träger holiday period. Saved
// versions retain its ID; bytes and filename are immutable after upload.
type HolidayFormDocument struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	Size     int    `json:"size"`
	Content  []byte `json:"-"`
}

var (
	ErrHolidayFormDocumentNotFound   = errors.New("holiday form document not found")
	ErrHolidayFormDocumentReferenced = errors.New("a saved holiday form references this document")
)
