package domain

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

// Grade transition lifecycle and ledger vocabulary. The values are stored
// verbatim in education.grade_transitions and its ledgers.
const (
	TransitionStatusDraft    = "draft"
	TransitionStatusApplied  = "applied"
	TransitionStatusReverted = "reverted"

	TransitionActionPromoted  = "promoted"
	TransitionActionGraduated = "graduated"

	LedgerActionRemoved = "removed"
	LedgerActionCreated = "created"
)

var (
	ErrTransitionNotFound = errors.New("grade transition not found")
	// ErrTransitionStateConflict reports a status guard that matched no row:
	// the transition left the status the caller expected before the write.
	ErrTransitionStateConflict = errors.New("grade transition is not in the expected status")
	ErrInvalidTransition       = errors.New("invalid grade transition")
)

// InvalidTransitionError carries the validation reason and unwraps to
// ErrInvalidTransition.
type InvalidTransitionError struct{ Reason string }

func (e *InvalidTransitionError) Error() string { return e.Reason }
func (e *InvalidTransitionError) Unwrap() error { return ErrInvalidTransition }

var academicYearPattern = regexp.MustCompile(`^\d{4}-\d{4}$`)

type Transition struct {
	ID                       int64
	TenantID                 int64
	CreatedAt                time.Time
	UpdatedAt                time.Time
	AcademicYear             string
	Status                   string
	AppliedAt                *time.Time
	AppliedBy                *int64
	RevertedAt               *time.Time
	RevertedBy               *int64
	CreatedBy                int64
	Notes                    *string
	RosterBaselineInstanceID *int64
	Mappings                 []TransitionMapping
}

type TransitionMapping struct {
	ID           int64
	TenantID     int64
	TransitionID int64
	FromClass    string
	ToClass      *string
}

// TransitionMappingInput is one class rename of a draft; a nil target
// graduates the class.
type TransitionMappingInput struct {
	FromClass string
	ToClass   *string
}

type TransitionDraft struct {
	AcademicYear string
	Notes        *string
	CreatedBy    int64
	Mappings     []TransitionMappingInput
}

// TransitionUpdate patches a draft. A nil Mappings slice keeps the stored
// mappings; an empty one clears them.
type TransitionUpdate struct {
	ID           int64
	AcademicYear *string
	Notes        *string
	Mappings     []TransitionMappingInput
}

type TransitionFilter struct {
	Status       string
	AcademicYear string
	// AfterID switches the listing to an ascending id window.
	AfterID int64
	Limit   int
	Offset  int
}

type TransitionHistoryEntry struct {
	ID           int64
	TenantID     int64
	CreatedAt    time.Time
	TransitionID int64
	StudentID    int64
	PersonName   string
	FromClass    string
	ToClass      *string
	Action       string
	FromStatus   *string
	RFIDTag      *string
}

type TransitionClassTeacherEntry struct {
	ID           int64
	TransitionID int64
	StaffID      int64
	SchoolClass  string
	Action       string
}

type TransitionClassListEntry struct {
	ID           int64
	TransitionID int64
	EntryID      *int64
	FirstName    string
	LastName     string
	SchoolClass  string
	Action       string
}

// ValidateAcademicYear normalizes and checks the YYYY-YYYY form.
func ValidateAcademicYear(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", &InvalidTransitionError{Reason: "academic year is required"}
	}
	if !academicYearPattern.MatchString(value) {
		return "", &InvalidTransitionError{Reason: "academic year must be in format YYYY-YYYY (e.g., 2025-2026)"}
	}
	return value, nil
}

// NormalizeMappings trims every mapping, turns an empty target into a
// graduation, and refuses an empty source, a self-mapping and a duplicate
// source class.
func NormalizeMappings(inputs []TransitionMappingInput) ([]TransitionMappingInput, error) {
	result := make([]TransitionMappingInput, 0, len(inputs))
	seen := make(map[string]bool, len(inputs))
	for _, input := range inputs {
		from := strings.TrimSpace(input.FromClass)
		if from == "" {
			return nil, &InvalidTransitionError{Reason: "invalid mapping for class " + input.FromClass + ": from_class is required"}
		}
		var to *string
		if input.ToClass != nil {
			trimmed := strings.TrimSpace(*input.ToClass)
			if trimmed != "" {
				to = &trimmed
			}
		}
		if to != nil && *to == from {
			return nil, &InvalidTransitionError{Reason: "invalid mapping for class " + from + ": from_class and to_class cannot be the same"}
		}
		if seen[from] {
			return nil, &InvalidTransitionError{Reason: "duplicate mapping for class " + from}
		}
		seen[from] = true
		result = append(result, TransitionMappingInput{FromClass: from, ToClass: to})
	}
	return result, nil
}

func (e TransitionHistoryEntry) validate() error {
	if e.TransitionID <= 0 || e.StudentID <= 0 {
		return &InvalidTransitionError{Reason: "history entry needs a transition and a student"}
	}
	if strings.TrimSpace(e.PersonName) == "" || strings.TrimSpace(e.FromClass) == "" {
		return &InvalidTransitionError{Reason: "history entry needs a person name and a from class"}
	}
	if e.Action != TransitionActionPromoted && e.Action != TransitionActionGraduated {
		return &InvalidTransitionError{Reason: "history entry action must be promoted or graduated"}
	}
	return nil
}

// ValidateHistory checks every ledger row before an append.
func ValidateHistory(entries []TransitionHistoryEntry) error {
	for _, entry := range entries {
		if err := entry.validate(); err != nil {
			return err
		}
	}
	return nil
}

func validLedgerAction(action string) bool {
	return action == LedgerActionRemoved || action == LedgerActionCreated
}

func ValidateClassTeacherLedger(entries []TransitionClassTeacherEntry) error {
	for _, entry := range entries {
		if entry.TransitionID <= 0 || entry.StaffID <= 0 || strings.TrimSpace(entry.SchoolClass) == "" || !validLedgerAction(entry.Action) {
			return &InvalidTransitionError{Reason: "invalid class teacher ledger entry"}
		}
	}
	return nil
}

func ValidateClassListLedger(entries []TransitionClassListEntry) error {
	for _, entry := range entries {
		if entry.TransitionID <= 0 || strings.TrimSpace(entry.FirstName) == "" || strings.TrimSpace(entry.LastName) == "" ||
			strings.TrimSpace(entry.SchoolClass) == "" || !validLedgerAction(entry.Action) {
			return &InvalidTransitionError{Reason: "invalid class list ledger entry"}
		}
	}
	return nil
}
