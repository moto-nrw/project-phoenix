package domain

import (
	"errors"
	"slices"
	"strconv"
	"strings"
)

// The Personalstamm of ADR 0089 (#4705): the rules that need no database.
// The suggestions follow section 5: the same account at several schools
// first, then the same personnel number, then name and birthday. A link
// takes every field from one source; a value it does not take goes into the
// trail with its school.

// Fields of a record, in the order the preview lists them.
const (
	StaffFieldFirstName            = "first_name"
	StaffFieldLastName             = "last_name"
	StaffFieldBirthday             = "birthday"
	StaffFieldPersonnelNumber      = "personnel_number"
	StaffFieldIBAN                 = "iban"
	StaffFieldTaxID                = "tax_id"
	StaffFieldSocialSecurityNumber = "social_security_number"
)

// StaffRecordFields lists every field a link resolves.
var StaffRecordFields = []string{
	StaffFieldFirstName, StaffFieldLastName, StaffFieldBirthday, StaffFieldPersonnelNumber,
	StaffFieldIBAN, StaffFieldTaxID, StaffFieldSocialSecurityNumber,
}

// IsFinancialStaffField reports the fields kept sealed.
func IsFinancialStaffField(field string) bool {
	return field == StaffFieldIBAN || field == StaffFieldTaxID || field == StaffFieldSocialSecurityNumber
}

// StaffSourceRecord is the source key of the record's own values.
const StaffSourceRecord = "record"

// Why employments are proposed together.
const (
	StaffMatchAccount         = "same_account"
	StaffMatchPersonnelNumber = "same_personnel_number"
	StaffMatchNameBirthday    = "same_name_birthday"
	StaffMatchNone            = "single"
)

// MaxLinkMembers bounds one link: a Träger has rarely more schools than
// this, and a person is employed at most once per school.
const MaxLinkMembers = 50

// Errors of a link the caller maps to a field.
var (
	ErrStaffChoiceMissing = errors.New("a field has different values and needs a choice")
	ErrStaffChoiceUnknown = errors.New("the choice names no source of the link")
)

// StaffChoiceError names the field whose choice is missing or unknown.
type StaffChoiceError struct {
	Field string
	Err   error
}

func (e *StaffChoiceError) Error() string { return e.Field + ": " + e.Err.Error() }
func (e *StaffChoiceError) Unwrap() error { return e.Err }

// StaffSource is one source of a link: an employment of one school or the
// existing record. Values holds the plaintext of every field it has; a
// field it lacks is absent or empty. Display holds what the preview may
// show: the value itself, or the mask of a bank or tax value.
type StaffSource struct {
	Key        string
	SchoolID   int64
	SchoolName string
	Values     map[string]string
	Display    map[string]string
}

// StaffFieldOption is one distinct value of a field and the sources that
// bring it, in source order.
type StaffFieldOption struct {
	Value   string
	Sources []StaffSource
}

// StaffFieldPreview is the distinct values of one field.
type StaffFieldPreview struct {
	Field   string
	Options []StaffFieldOption
}

// Conflict reports whether the field needs a choice.
func (p StaffFieldPreview) Conflict() bool { return len(p.Options) > 1 }

// PreviewStaffFields lists, per field, the distinct non-empty values of the
// sources. Equal values of several sources are one option.
func PreviewStaffFields(sources []StaffSource) []StaffFieldPreview {
	previews := make([]StaffFieldPreview, 0, len(StaffRecordFields))
	for _, field := range StaffRecordFields {
		preview := StaffFieldPreview{Field: field}
		for _, source := range sources {
			value := normalizedStaffValue(field, source.Values[field])
			if value == "" {
				continue
			}
			index := slices.IndexFunc(preview.Options, func(option StaffFieldOption) bool {
				return normalizedStaffValue(field, option.Value) == value
			})
			if index < 0 {
				preview.Options = append(preview.Options, StaffFieldOption{Value: source.Values[field]})
				index = len(preview.Options) - 1
			}
			preview.Options[index].Sources = append(preview.Options[index].Sources, source)
		}
		previews = append(previews, preview)
	}
	return previews
}

// normalizedStaffValue compares names without case and surrounding space,
// bank and tax values without spaces; it never changes the stored value.
func normalizedStaffValue(field, value string) string {
	value = strings.TrimSpace(value)
	if IsFinancialStaffField(field) {
		return strings.ToUpper(strings.ReplaceAll(value, " ", ""))
	}
	if field == StaffFieldFirstName || field == StaffFieldLastName {
		return strings.ToLower(value)
	}
	return value
}

// StaffNotChosen is a value a link did not take, with the school it came
// from. The record's own values are never "not chosen": they are changes.
type StaffNotChosen struct {
	Field  string
	Value  string
	Source StaffSource
}

// ResolvedStaffFields is the outcome of a link: the value of every field
// (empty when no source has one) and the values not taken.
type ResolvedStaffFields struct {
	Values    map[string]string
	NotChosen []StaffNotChosen
}

// ResolveStaffFields takes every field from its only value or, when the
// sources disagree, from the source choices names. A choice for a field
// without conflict is ignored; a missing or unknown one fails with a
// *StaffChoiceError.
func ResolveStaffFields(sources []StaffSource, choices map[string]string) (ResolvedStaffFields, error) {
	resolved := ResolvedStaffFields{Values: make(map[string]string, len(StaffRecordFields))}
	for _, preview := range PreviewStaffFields(sources) {
		if len(preview.Options) == 0 {
			continue
		}
		chosen, err := chosenOption(preview, choices[preview.Field])
		if err != nil {
			return ResolvedStaffFields{}, err
		}
		resolved.Values[preview.Field] = preview.Options[chosen].Value
		resolved.NotChosen = append(resolved.NotChosen, notChosenValues(preview, chosen)...)
	}
	return resolved, nil
}

// chosenOption returns the only option of a field or the one whose sources
// include the chosen key.
func chosenOption(preview StaffFieldPreview, choice string) (int, error) {
	if !preview.Conflict() {
		return 0, nil
	}
	key := strings.TrimSpace(choice)
	if key == "" {
		return 0, &StaffChoiceError{Field: preview.Field, Err: ErrStaffChoiceMissing}
	}
	for i, option := range preview.Options {
		if slices.ContainsFunc(option.Sources, func(source StaffSource) bool { return source.Key == key }) {
			return i, nil
		}
	}
	return 0, &StaffChoiceError{Field: preview.Field, Err: ErrStaffChoiceUnknown}
}

// notChosenValues lists the values of the other options with their schools;
// the record's own values are changes, not values of a school.
func notChosenValues(preview StaffFieldPreview, chosen int) []StaffNotChosen {
	var values []StaffNotChosen
	for i, option := range preview.Options {
		if i == chosen {
			continue
		}
		for _, source := range option.Sources {
			if source.Key != StaffSourceRecord {
				values = append(values, StaffNotChosen{Field: preview.Field, Value: option.Value, Source: source})
			}
		}
	}
	return values
}

// SchoolSourceKey is the source key of a school's employment.
func SchoolSourceKey(schoolID int64) string { return strconv.FormatInt(schoolID, 10) }

// StaffCandidate is one live employment no record holds.
type StaffCandidate struct {
	SchoolID        int64
	StaffID         int64
	AccountID       *int64
	FirstName       string
	LastName        string
	Birthday        string
	PersonnelNumber string
}

// StaffMatchRecord is what a suggestion compares of an existing record: its
// number, names, birthday, the accounts and the schools of its employments.
type StaffMatchRecord struct {
	ID              int64
	FirstName       string
	LastName        string
	Birthday        string
	PersonnelNumber string
	AccountIDs      []int64
	SchoolIDs       []int64
}

// StaffSuggestion is one proposal of candidates, for a new record or for
// the existing record RecordID.
type StaffSuggestion struct {
	Reason     string
	RecordID   int64
	Candidates []StaffCandidate
}

// SuggestStaffLinks proposes, in this order: candidates that match an
// existing record, then groups of candidates of different schools, then
// every remaining candidate on its own. Each criterion of ADR 0089 section
// 5 is tried in turn; a candidate lands in one proposal only, and a group
// never holds two employments of one school. Candidates without any match
// come last, so the list starts with what needs a decision.
func SuggestStaffLinks(records []StaffMatchRecord, candidates []StaffCandidate) []StaffSuggestion {
	used := make([]bool, len(candidates))
	var suggestions []StaffSuggestion
	criteria := []struct {
		reason string
		key    func(StaffCandidate) []string
		record func(StaffMatchRecord) []string
	}{
		{StaffMatchAccount, candidateAccountKey, recordAccountKeys},
		{StaffMatchPersonnelNumber, candidateNumberKey, recordNumberKey},
		{StaffMatchNameBirthday, candidateNameKey, recordNameKey},
	}
	for _, criterion := range criteria {
		suggestions = append(suggestions, matchRecords(records, candidates, used, criterion.reason, criterion.key, criterion.record)...)
	}
	for _, criterion := range criteria {
		suggestions = append(suggestions, groupCandidates(candidates, used, criterion.reason, criterion.key)...)
	}
	for i, candidate := range candidates {
		if !used[i] {
			suggestions = append(suggestions, StaffSuggestion{Reason: StaffMatchNone, Candidates: []StaffCandidate{candidate}})
		}
	}
	return suggestions
}

// matchRecords proposes the candidates a record matches by one criterion:
// only from schools the record does not hold yet.
func matchRecords(records []StaffMatchRecord, candidates []StaffCandidate, used []bool, reason string,
	key func(StaffCandidate) []string, recordKey func(StaffMatchRecord) []string,
) []StaffSuggestion {
	var suggestions []StaffSuggestion
	for _, record := range records {
		keys := recordKey(record)
		if len(keys) == 0 {
			continue
		}
		schools := slices.Clone(record.SchoolIDs)
		var matched []StaffCandidate
		for i, candidate := range candidates {
			if used[i] || slices.Contains(schools, candidate.SchoolID) || !sharesKey(keys, key(candidate)) {
				continue
			}
			used[i] = true
			schools = append(schools, candidate.SchoolID)
			matched = append(matched, candidate)
		}
		if len(matched) > 0 {
			suggestions = append(suggestions, StaffSuggestion{Reason: reason, RecordID: record.ID, Candidates: matched})
		}
	}
	return suggestions
}

// groupCandidates proposes candidates of different schools that share a key.
func groupCandidates(candidates []StaffCandidate, used []bool, reason string, key func(StaffCandidate) []string) []StaffSuggestion {
	var suggestions []StaffSuggestion
	for i, first := range candidates {
		keys := key(first)
		if used[i] || len(keys) == 0 {
			continue
		}
		group := []StaffCandidate{first}
		schools := []int64{first.SchoolID}
		for j := i + 1; j < len(candidates); j++ {
			other := candidates[j]
			if used[j] || slices.Contains(schools, other.SchoolID) || !sharesKey(keys, key(other)) {
				continue
			}
			used[j] = true
			group = append(group, other)
			schools = append(schools, other.SchoolID)
		}
		if len(group) > 1 {
			used[i] = true
			suggestions = append(suggestions, StaffSuggestion{Reason: reason, Candidates: group})
		}
	}
	return suggestions
}

func sharesKey(a, b []string) bool {
	for _, key := range b {
		if slices.Contains(a, key) {
			return true
		}
	}
	return false
}

func candidateAccountKey(candidate StaffCandidate) []string {
	if candidate.AccountID == nil || *candidate.AccountID <= 0 {
		return nil
	}
	return []string{strconv.FormatInt(*candidate.AccountID, 10)}
}

func recordAccountKeys(record StaffMatchRecord) []string {
	keys := make([]string, 0, len(record.AccountIDs))
	for _, id := range record.AccountIDs {
		keys = append(keys, strconv.FormatInt(id, 10))
	}
	return keys
}

func candidateNumberKey(candidate StaffCandidate) []string {
	return nonEmpty(candidate.PersonnelNumber)
}
func recordNumberKey(record StaffMatchRecord) []string { return nonEmpty(record.PersonnelNumber) }

func candidateNameKey(candidate StaffCandidate) []string {
	return nameBirthdayKey(candidate.FirstName, candidate.LastName, candidate.Birthday)
}

func recordNameKey(record StaffMatchRecord) []string {
	return nameBirthdayKey(record.FirstName, record.LastName, record.Birthday)
}

// nameBirthdayKey needs a birthday: a name alone is no match.
func nameBirthdayKey(firstName, lastName, birthday string) []string {
	if strings.TrimSpace(birthday) == "" || strings.TrimSpace(firstName) == "" || strings.TrimSpace(lastName) == "" {
		return nil
	}
	return []string{strings.ToLower(strings.TrimSpace(firstName)) + "\x00" + strings.ToLower(strings.TrimSpace(lastName)) + "\x00" + strings.TrimSpace(birthday)}
}

func nonEmpty(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return []string{strings.TrimSpace(value)}
}
