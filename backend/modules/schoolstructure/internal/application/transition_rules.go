package application

import (
	"regexp"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
)

var academicYearPattern = regexp.MustCompile(`^\d{4}-\d{4}$`)

// validateAcademicYear normalizes and checks the YYYY-YYYY form.
func validateAcademicYear(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", &schoolstructure.InvalidTransitionError{Reason: "academic year is required"}
	}
	if !academicYearPattern.MatchString(value) {
		return "", &schoolstructure.InvalidTransitionError{Reason: "academic year must be in format YYYY-YYYY (e.g., 2025-2026)"}
	}
	return value, nil
}

// normalizeMappings trims every mapping, turns an empty target into a
// graduation, and refuses an empty source, a self-mapping and a duplicate
// source class.
func normalizeMappings(inputs []schoolstructure.TransitionMappingInput) ([]schoolstructure.TransitionMappingInput, error) {
	result := make([]schoolstructure.TransitionMappingInput, 0, len(inputs))
	seen := make(map[string]bool, len(inputs))
	for _, input := range inputs {
		from := strings.TrimSpace(input.FromClass)
		if from == "" {
			return nil, &schoolstructure.InvalidTransitionError{Reason: "invalid mapping for class " + input.FromClass + ": from_class is required"}
		}
		var to *string
		if input.ToClass != nil {
			trimmed := strings.TrimSpace(*input.ToClass)
			if trimmed != "" {
				to = &trimmed
			}
		}
		if to != nil && *to == from {
			return nil, &schoolstructure.InvalidTransitionError{Reason: "invalid mapping for class " + from + ": from_class and to_class cannot be the same"}
		}
		if seen[from] {
			return nil, &schoolstructure.InvalidTransitionError{Reason: "duplicate mapping for class " + from}
		}
		seen[from] = true
		result = append(result, schoolstructure.TransitionMappingInput{FromClass: from, ToClass: to})
	}
	return result, nil
}

func validateHistoryEntry(e schoolstructure.TransitionHistoryEntry) error {
	if e.TransitionID <= 0 || e.StudentID <= 0 {
		return &schoolstructure.InvalidTransitionError{Reason: "history entry needs a transition and a student"}
	}
	if strings.TrimSpace(e.PersonName) == "" || strings.TrimSpace(e.FromClass) == "" {
		return &schoolstructure.InvalidTransitionError{Reason: "history entry needs a person name and a from class"}
	}
	if e.Action != schoolstructure.TransitionActionPromoted && e.Action != schoolstructure.TransitionActionGraduated {
		return &schoolstructure.InvalidTransitionError{Reason: "history entry action must be promoted or graduated"}
	}
	return nil
}

// validateHistory checks every ledger row before an append.
func validateHistory(entries []schoolstructure.TransitionHistoryEntry) error {
	for _, entry := range entries {
		if err := validateHistoryEntry(entry); err != nil {
			return err
		}
	}
	return nil
}

func validLedgerAction(action string) bool {
	return action == schoolstructure.LedgerActionRemoved || action == schoolstructure.LedgerActionCreated
}

func validateClassTeacherLedger(entries []schoolstructure.TransitionClassTeacherEntry) error {
	for _, entry := range entries {
		if entry.TransitionID <= 0 || entry.StaffID <= 0 || strings.TrimSpace(entry.SchoolClass) == "" || !validLedgerAction(entry.Action) {
			return &schoolstructure.InvalidTransitionError{Reason: "invalid class teacher ledger entry"}
		}
	}
	return nil
}

func validateClassListLedger(entries []schoolstructure.TransitionClassListEntry) error {
	for _, entry := range entries {
		if entry.TransitionID <= 0 || strings.TrimSpace(entry.FirstName) == "" || strings.TrimSpace(entry.LastName) == "" ||
			strings.TrimSpace(entry.SchoolClass) == "" || !validLedgerAction(entry.Action) {
			return &schoolstructure.InvalidTransitionError{Reason: "invalid class list ledger entry"}
		}
	}
	return nil
}
