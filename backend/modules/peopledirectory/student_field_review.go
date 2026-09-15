package peopledirectory

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
)

// StudentFieldChange is a proposed change to one People Directory field.
// Request IDs identify results only; this capability never reads or writes
// request rows, which belong to Care Plan.
type StudentFieldChange struct {
	RequestID int64
	StudentID int64
	Target    string
	Field     string
	OldValue  json.RawMessage
	NewValue  json.RawMessage
}

// StudentFieldReview contains the owner's baseline and validation facts.
// Care Plan applies the caller's review scope separately.
type StudentFieldReview struct {
	Student              *Student
	FirstName            string
	LastName             string
	BulkEligible         bool
	BulkIneligibleReason string
	BulkIneligibleText   string
	CurrentValueChanged  *bool
}

type StudentFieldReviewQuery interface {
	ReviewStudentFields(context.Context, []StudentFieldChange) (map[int64]StudentFieldReview, error)
}

func (m *Module) ReviewStudentFields(ctx context.Context, changes []StudentFieldChange) (map[int64]StudentFieldReview, error) {
	studentIDs := make([]int64, 0, len(changes))
	departureIDs := make([]int64, 0)
	for _, change := range changes {
		studentIDs = append(studentIDs, change.StudentID)
		if change.Target == "departure" {
			departureIDs = append(departureIDs, change.StudentID)
		}
	}
	students, err := m.ListStudentsByID(ctx, studentIDs)
	if err != nil {
		return nil, fmt.Errorf("review: load students: %w", err)
	}
	studentByID := make(map[int64]*Student, len(students))
	personIDs := make([]int64, 0, len(students))
	for i := range students {
		studentByID[students[i].ID] = &students[i]
		personIDs = append(personIDs, students[i].PersonID)
	}
	persons, err := m.ListPersonsByID(ctx, personIDs)
	if err != nil {
		return nil, fmt.Errorf("review: load persons: %w", err)
	}
	personByID := make(map[int64]*Person, len(persons))
	for i := range persons {
		personByID[persons[i].ID] = &persons[i]
	}
	plans, err := m.ListStudentDepartureModes(ctx, departureIDs)
	if err != nil {
		return nil, fmt.Errorf("review: load departure modes: %w", err)
	}
	result := make(map[int64]StudentFieldReview, len(changes))
	for _, change := range changes {
		student := studentByID[change.StudentID]
		var person *Person
		if student != nil {
			person = personByID[student.PersonID]
		}
		result[change.RequestID] = reviewStudentField(change, student, person, plans[change.StudentID])
	}
	return result, nil
}

const (
	fieldReviewSingleOnly = "Diese Anfrage kann nur einzeln freigegeben werden."
	fieldReviewStale      = "Der aktuelle Wert wurde nach der Anfrage geändert."
)

func reviewStudentField(change StudentFieldChange, student *Student, person *Person, modes map[string][]string) StudentFieldReview {
	result := StudentFieldReview{Student: student, BulkIneligibleReason: "single_only", BulkIneligibleText: fieldReviewSingleOnly}
	if student == nil {
		result.BulkIneligibleReason, result.BulkIneligibleText = "child_unavailable", "Das Kind ist nicht mehr verfügbar."
		return result
	}
	if person != nil {
		result.FirstName, result.LastName = person.FirstName, person.LastName
	}
	var current json.RawMessage
	valid, readable := false, false
	switch change.Target {
	case "person":
		if person == nil {
			result.BulkIneligibleReason, result.BulkIneligibleText = "child_unavailable", "Die Stammdaten sind nicht mehr verfügbar."
			return result
		}
		switch change.Field {
		case "first_name":
			current, _ = json.Marshal(person.FirstName)
			readable, valid = true, validFieldString(change.NewValue)
		case "last_name":
			current, _ = json.Marshal(person.LastName)
			readable, valid = true, validFieldString(change.NewValue)
		case "birthday":
			current = json.RawMessage("null")
			if person.Birthday != "" {
				current, _ = json.Marshal(person.Birthday)
			}
			var birthday string
			valid = json.Unmarshal(change.NewValue, &birthday) == nil && strings.TrimSpace(birthday) != "" && validateBirthday(birthday) == nil
			readable = true
		}
	case "student":
		if change.Field == "school_class" {
			current, _ = json.Marshal(student.SchoolClass)
			readable, valid = true, validFieldString(change.NewValue)
		}
	case "departure":
		return reviewDepartureField(result, change, modes)
	}
	if readable {
		changed := !equalFieldJSON(current, change.OldValue)
		result.CurrentValueChanged = &changed
		if valid {
			setFieldEligibility(&result, changed)
		}
	}
	return result
}

func validFieldString(raw json.RawMessage) bool {
	var value string
	return json.Unmarshal(raw, &value) == nil && strings.TrimSpace(value) != ""
}

func equalFieldJSON(a, b json.RawMessage) bool {
	var left, right any
	return json.Unmarshal(a, &left) == nil && json.Unmarshal(b, &right) == nil && reflect.DeepEqual(left, right)
}

func setFieldEligibility(result *StudentFieldReview, changed bool) {
	if changed {
		result.BulkIneligibleReason, result.BulkIneligibleText = "stale", fieldReviewStale
		return
	}
	result.BulkEligible = true
	result.BulkIneligibleReason, result.BulkIneligibleText = "", ""
}

func reviewDepartureField(result StudentFieldReview, change StudentFieldChange, modes map[string][]string) StudentFieldReview {
	previous, validPrevious := decodeReviewDeparture(change.OldValue)
	if !validPrevious {
		return result
	}
	changed := !reflect.DeepEqual(normalizedReviewDeparture(modes), normalizedReviewDeparture(previous))
	result.CurrentValueChanged = &changed
	requested, validRequested := decodeReviewDeparture(change.NewValue)
	if change.Field != "allowed_departure_modes" || !validRequested {
		return result
	}
	for _, values := range requested {
		if slices.Contains(values, "accompanied") {
			return result
		}
	}
	setFieldEligibility(&result, changed)
	return result
}

var reviewDepartureDays = []string{"mon", "tue", "wed", "thu", "fri"}
var reviewDepartureModes = []string{"alone", "bus", "pickup", "accompanied"}

func decodeReviewDeparture(raw json.RawMessage) (map[string][]string, bool) {
	var modes map[string][]string
	if json.Unmarshal(raw, &modes) != nil {
		return nil, false
	}
	for day, values := range modes {
		if !slices.Contains(reviewDepartureDays, day) {
			return nil, false
		}
		seen := make(map[string]bool, len(values))
		for _, value := range values {
			if !slices.Contains(reviewDepartureModes, value) || seen[value] {
				return nil, false
			}
			seen[value] = true
		}
	}
	return modes, true
}

func normalizedReviewDeparture(modes map[string][]string) map[string][]string {
	result := make(map[string][]string)
	for _, day := range reviewDepartureDays {
		for _, mode := range reviewDepartureModes {
			if slices.Contains(modes[day], mode) {
				result[day] = append(result[day], mode)
			}
		}
	}
	return result
}
