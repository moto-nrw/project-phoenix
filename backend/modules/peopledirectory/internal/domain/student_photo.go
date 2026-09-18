package domain

import (
	"errors"
	"time"
)

// Sentinel outcomes of the student-photo lifecycle. They are the owner's
// vocabulary; the HTTP adapter maps them to status codes.
var (
	ErrPhotoFeatureDisabled    = errors.New("photo feature disabled")
	ErrPhotoFeatureDisabledMid = errors.New("photo feature disabled mid-operation")
	ErrPhotoStudentForbidden   = errors.New("student photo access denied")
	ErrPhotoConsentRequired    = errors.New("photo consent required")
	ErrPhotoConsentWithdrawn   = errors.New("photo consent withdrawn mid-operation")
	ErrPhotoFilenameMismatch   = errors.New("photo filename mismatch")
	ErrPhotoNotSet             = errors.New("no photo set for this student")
)

// StudentPhoto is the photo slice of a child's row: where the image is stored
// and whether the voluntary consent behind it is on record.
type StudentPhoto struct {
	StudentID           int64
	Status              string
	PhotoPath           *string
	PhotoConsentGivenAt *time.Time
	PhotoConsentGivenBy *int64
}

func (p StudentPhoto) IsAlumnus() bool { return p.Status == StudentStatusAlumnus }

// StoredPhoto reports the stored URL, empty when the child has no photo.
func (p StudentPhoto) StoredPhoto() string {
	if p.PhotoPath == nil {
		return ""
	}
	return *p.PhotoPath
}

// PhotoConsentTransition is the pure result of reconciling a requested consent
// against the row's current state.
type PhotoConsentTransition struct {
	GrantedNow       bool
	WithdrawnNow     bool
	RemovedPhotoPath string
}

// ApplyPhotoConsent decides the transition and returns the photo slice as it
// must be written. Re-stamping an already-granted row is a no-op so idempotent
// PUTs preserve the original consent timestamp.
func ApplyPhotoConsent(
	requestedConsent *bool,
	current StudentPhoto,
	now time.Time,
	actorAccountID int64,
) (StudentPhoto, PhotoConsentTransition) {
	if requestedConsent == nil {
		return current, PhotoConsentTransition{}
	}
	hadConsent := current.PhotoConsentGivenAt != nil
	wantConsent := *requestedConsent

	switch {
	case !hadConsent && wantConsent:
		grantedAt, grantedBy := now, actorAccountID
		current.PhotoConsentGivenAt = &grantedAt
		current.PhotoConsentGivenBy = &grantedBy
		return current, PhotoConsentTransition{GrantedNow: true}
	case hadConsent && !wantConsent:
		removed := current.StoredPhoto()
		current.PhotoPath = nil
		current.PhotoConsentGivenAt = nil
		current.PhotoConsentGivenBy = nil
		return current, PhotoConsentTransition{WithdrawnNow: true, RemovedPhotoPath: removed}
	default:
		return current, PhotoConsentTransition{}
	}
}

// PhotoFilenameOf is the last path segment of a stored photo URL. The read
// route compares it after the access check, so a denied caller cannot probe
// whether a given filename exists.
func PhotoFilenameOf(storedURL string) string {
	for i := len(storedURL) - 1; i >= 0; i-- {
		if storedURL[i] == '/' {
			return storedURL[i+1:]
		}
	}
	return storedURL
}
