package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/careplan/masterdatarequests"
)

// The fields a Stammdaten request can change.
const (
	masterDataFieldFirstName   = "first_name"
	masterDataFieldLastName    = "last_name"
	masterDataFieldBirthday    = "birthday"
	masterDataFieldSchoolClass = "school_class"
	masterDataFieldDeparture   = "allowed_departure_modes"
)

// applyChange writes the request's new value to the child's record, but only
// while the live value still equals the baseline the family saw. It reports
// whether the write trimmed a "läuft mit" link.
func (s *MasterDataDecisions) applyChange(ctx context.Context, req *careplan.StudentDataChangeRequest, reviewedBy int64) (bool, error) {
	switch req.Target {
	case masterdatarequests.TargetPerson:
		return false, s.applyPersonChange(ctx, req)
	case masterdatarequests.TargetStudent:
		return s.applyStudentChange(ctx, req, reviewedBy)
	case masterdatarequests.TargetDeparture:
		return s.applyDepartureChange(ctx, req, reviewedBy)
	default:
		return false, masterdatarequests.ErrReviewInvalidTarget
	}
}

func (s *MasterDataDecisions) applyStudentChange(ctx context.Context, req *careplan.StudentDataChangeRequest, reviewedBy int64) (bool, error) {
	if req.FieldKey != masterDataFieldSchoolClass {
		return false, masterdatarequests.ErrReviewInvalidTarget
	}
	var value string
	if err := json.Unmarshal(req.NewValue, &value); err != nil {
		return false, masterdatarequests.ErrReviewInvalidValue
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return false, masterdatarequests.ErrReviewInvalidValue
	}
	trimmed, err := s.records.UpdateStudent(ctx, req.StudentID, reviewedBy, func(student ports.MasterDataStudent) (ports.MasterDataStudentWrite, error) {
		if !domain.SameJSON(domain.JSONString(student.SchoolClass), req.OldValue) {
			return ports.MasterDataStudentWrite{}, masterdatarequests.ErrReviewStaleValue
		}
		return ports.MasterDataStudentWrite{SchoolClass: &value}, nil
	})
	return trimmed, labelMasterDataWrite(err, "review: load student", "review: update student class", "review: audit student class")
}

func (s *MasterDataDecisions) applyPersonChange(ctx context.Context, req *careplan.StudentDataChangeRequest) error {
	var value string
	if err := json.Unmarshal(req.NewValue, &value); err != nil {
		return masterdatarequests.ErrReviewInvalidValue
	}
	if req.FieldKey == masterDataFieldBirthday {
		if _, err := careplan.ParseDate(value); err != nil {
			return masterdatarequests.ErrReviewInvalidValue
		}
	}
	student, err := s.records.FindStudent(ctx, req.StudentID)
	if err != nil {
		return fmt.Errorf("review: load student: %w", err)
	}
	err = s.records.UpdatePerson(ctx, student.PersonID, func(person *ports.MasterDataPerson) error {
		current, fieldErr := masterDataPersonValue(person, req.FieldKey)
		if fieldErr != nil {
			return fieldErr
		}
		if !domain.SameJSON(current, req.OldValue) {
			return masterdatarequests.ErrReviewStaleValue
		}
		return setMasterDataPersonField(person, req.FieldKey, value, masterdatarequests.ErrReviewInvalidTarget)
	})
	return labelMasterDataWrite(err, "review: load person", "review: update person", "review: audit person")
}

func (s *MasterDataDecisions) applyDepartureChange(ctx context.Context, req *careplan.StudentDataChangeRequest, reviewedBy int64) (bool, error) {
	if req.FieldKey != masterDataFieldDeparture {
		return false, masterdatarequests.ErrReviewInvalidTarget
	}
	modes, err := decodeRequestedDeparture(req.NewValue)
	if err != nil {
		return false, err
	}
	trimmed, err := s.records.UpdateStudent(ctx, req.StudentID, reviewedBy, func(student ports.MasterDataStudent) (ports.MasterDataStudentWrite, error) {
		previous, valid := domain.DecodeDepartureModes(req.OldValue)
		if !valid {
			return ports.MasterDataStudentWrite{}, masterdatarequests.ErrReviewInvalidValue
		}
		if !domain.SameDepartureModes(student.DepartureModes.Normalize(), previous.Normalize()) {
			return ports.MasterDataStudentWrite{}, masterdatarequests.ErrReviewStaleValue
		}
		return ports.MasterDataStudentWrite{DepartureModes: modes.Normalize()}, nil
	})
	return trimmed, labelMasterDataWrite(err, "review: load student", "review: update student departure", "review: audit student departure")
}

// decodeRequestedDeparture reads a departure plan a request or a staff value
// asks for. "Geht mit" is never requestable: who the child leaves with is a
// companion link, not a plan value.
func decodeRequestedDeparture(raw json.RawMessage) (domain.DepartureModes, error) {
	modes, valid := domain.DecodeDepartureModes(raw)
	if !valid || modes.HasMode(domain.DepartureModeAccompanied) {
		return nil, masterdatarequests.ErrReviewInvalidValue
	}
	return modes, nil
}

// masterDataPersonValue is the live value of a person field as a request
// stores it. A child without a recorded birthday reads as JSON null.
func masterDataPersonValue(person *ports.MasterDataPerson, field string) (json.RawMessage, error) {
	switch field {
	case masterDataFieldFirstName:
		return domain.JSONString(person.FirstName), nil
	case masterDataFieldLastName:
		return domain.JSONString(person.LastName), nil
	case masterDataFieldBirthday:
		if person.Birthday == "" {
			return json.RawMessage("null"), nil
		}
		return domain.JSONString(person.Birthday), nil
	default:
		return nil, masterdatarequests.ErrReviewInvalidTarget
	}
}

// setMasterDataPersonField writes one person field. A birthday must be a
// calendar day; an unknown field answers unsupported.
func setMasterDataPersonField(person *ports.MasterDataPerson, field, value string, unsupported error) error {
	switch field {
	case masterDataFieldFirstName:
		person.FirstName = value
	case masterDataFieldLastName:
		person.LastName = value
	case masterDataFieldBirthday:
		birthday, err := careplan.ParseDate(value)
		if err != nil {
			return masterdatarequests.ErrReviewInvalidValue
		}
		person.Birthday = birthday.String()
	default:
		return unsupported
	}
	return nil
}

// labelMasterDataWrite names the step of a People Directory write that
// failed. A refusal from the change function passes through untouched.
func labelMasterDataWrite(err error, load, update, audit string) error {
	var failure *ports.MasterDataWriteError
	if !errors.As(err, &failure) {
		return err
	}
	switch failure.Step {
	case ports.MasterDataWriteLoad:
		return fmt.Errorf("%s: %w", load, failure.Err)
	case ports.MasterDataWriteAudit:
		return fmt.Errorf("%s: %w", audit, failure.Err)
	default:
		return fmt.Errorf("%s: %w", update, failure.Err)
	}
}
