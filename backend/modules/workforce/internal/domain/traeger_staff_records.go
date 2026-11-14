package domain

import (
	"errors"
	"time"
)

// Stored shapes of the Personalstamm (#4705).

// ErrTraegerStaffRecordMissing answers a record the Träger does not keep.
var ErrTraegerStaffRecordMissing = errors.New("traeger staff record not found")

// ErrTraegerStaffReleaseInUse answers a withdrawal the records still need.
var ErrTraegerStaffReleaseInUse = errors.New("traeger staff release still in use")

// ErrTraegerStaffEmploymentLinked answers a link of an employment that
// already belongs to a record, or a second employment of one school.
var ErrTraegerStaffEmploymentLinked = errors.New("traeger staff employment already linked")

// ErrStaffKeptByTraeger refuses a school's change of a value its Träger
// keeps in the Personalstamm.
var ErrStaffKeptByTraeger = errors.New("the Träger keeps this value in its staff record")

// ErrTraegerStaffReleaseTaken answers a second release of one Träger.
var ErrTraegerStaffReleaseTaken = errors.New("traeger staff release exists")

// TraegerStaffRelease is the release „Personal beim Träger“.
type TraegerStaffRelease struct {
	OrganizationID       int64
	Representation       string
	ReleasedByOperatorID int64
	ReleasedAt           time.Time
}

// TraegerStaffRecord is one Personalstamm with its Einsatzorte. Birthday is
// a calendar day or empty; the bank and tax values stay sealed.
type TraegerStaffRecord struct {
	ID                         int64
	OrganizationID             int64
	FirstName                  string
	LastName                   string
	Birthday                   string
	PersonnelNumber            *string
	IBANSealed                 []byte
	IBANMasked                 *string
	TaxIDSealed                []byte
	SocialSecurityNumberSealed []byte
	Links                      []TraegerStaffLink
}

// HasFinancial reports whether the record keeps any bank or tax value.
func (r TraegerStaffRecord) HasFinancial() bool {
	return len(r.IBANSealed) > 0 || len(r.TaxIDSealed) > 0 || len(r.SocialSecurityNumberSealed) > 0
}

// TraegerStaffLink is one Einsatzort: the employment StaffID of SchoolID.
type TraegerStaffLink struct {
	SchoolID int64
	StaffID  int64
	LinkedAt time.Time
}

// TraegerStaffChange is one trail entry. A bank or tax value is sealed with
// its mask; every other value is text.
type TraegerStaffChange struct {
	RecordID    int64
	Kind        string
	Field       string
	ValueText   *string
	ValueSealed []byte
	ValueMasked *string
	SchoolID    *int64
	AccountID   int64
	OccurredAt  time.Time
}

// Kinds of a trail entry.
const (
	StaffChangeLinked    = "linked"
	StaffChangeNotChosen = "not_chosen"
	StaffChangeChanged   = "changed"
	// StaffChangeFieldSchool is the field of a linked entry: the Einsatzort.
	StaffChangeFieldSchool = "school"
)

// MaskStaffValue masks a bank or tax value for the trail and the preview:
// the IBAN keeps its last four characters, the others show nothing of
// themselves.
func MaskStaffValue(field, value string) string {
	if value == "" {
		return ""
	}
	if field == StaffFieldIBAN {
		return deref(MaskIBAN(&value))
	}
	return maskedAll
}
