package auditlog

import (
	"context"
	"errors"
	"time"
)

// The access trail (#4655) answers who read or changed personal data at a
// set of schools, and when: the reads of audit.data_access_log and the
// changes of the Audit ledgers that record an actor on personal data. The
// Träger-Portal's access log reads it inside the Träger projection's
// administrative transaction (ADR 0084 section 7, ADR 0111); every statement
// filters on the given school set, so the trail never reaches a school
// outside it.

// The kinds of an access trail entry.
const (
	AccessTrailRead   = "read"
	AccessTrailChange = "change"
)

// The data of a change entry: the ledger it comes from. A read entry carries
// the resource type of its audit.data_access_log row instead.
const (
	AccessTrailStudentRecord        = "student_record"
	AccessTrailGuardianRecord       = "guardian_record"
	AccessTrailGuardianBankDetails  = "guardian_bank_details"
	AccessTrailStudentConsent       = "student_consent"
	AccessTrailStaffRecord          = "staff_record"
	AccessTrailStaffPersonnelNumber = "staff_personnel_number"
	AccessTrailAttendanceCorrection = "attendance_correction"
	AccessTrailStudentDeletion      = "student_deletion"
)

// ErrAccessTrailSchoolsRequired reports a trail query without a school set.
var ErrAccessTrailSchoolsRequired = errors.New("audit access trail: the school set is required")

// AccessTrailFilter selects trail entries of the school set Schools. From and
// To bound the instant (From inclusive, To exclusive). SchoolID, AccountID and
// Kind narrow further when set. A ledger that records the acting staff
// member instead of the account matches AccountID through StaffIDs, the
// staff IDs of that account at the schools.
type AccessTrailFilter struct {
	Schools   []int64
	From      time.Time
	To        time.Time
	SchoolID  int64
	AccountID int64
	StaffIDs  []int64
	Kind      string
	Limit     int
	Offset    int
}

// AccessTrailEntry is one read or change. The actor is an account
// (AccountID) or, where a ledger records the acting staff member, a staff ID
// (StaffID); both are nil for a system step or an account deleted since.
// ActorNameSnapshot is the name a ledger stored with the change, empty where
// it stores none.
type AccessTrailEntry struct {
	// ID is the row of the ledger the entry comes from; Kind and Data name
	// that ledger, so the three together identify the entry.
	ID                int64
	SchoolID          int64
	OccurredAt        time.Time
	AccountID         *int64
	StaffID           *int64
	ActorNameSnapshot string
	Kind              string
	Data              string
}

// AccessTrailActor is an account or a staff member (one of AccountID and
// StaffID is set) with at least one entry at a school.
type AccessTrailActor struct {
	SchoolID          int64
	AccountID         int64
	StaffID           int64
	ActorNameSnapshot string
}

// AccessTrailQuery reads the access trail of a school set. It joins the
// caller's transaction.
type AccessTrailQuery interface {
	// ListAccessTrail lists the entries of the filter, newest first.
	ListAccessTrail(ctx context.Context, filter AccessTrailFilter) ([]AccessTrailEntry, error)
	// ListAccessTrailActors lists the accounts with an entry at a school of
	// the set between from (inclusive) and to (exclusive).
	ListAccessTrailActors(ctx context.Context, schools []int64, from, to time.Time) ([]AccessTrailActor, error)
}
