package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/careplan/masterdatarequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/parentrequests"
)

// The Stammdaten queue's part in the cross-kind bulk approval and conflict
// resolution (#2267). The coordinator owns the group; this file owns the
// payload.

var (
	_ ports.MasterDataBulkPort = (*MasterDataDecisions)(nil)
	_ ports.ConflictPort       = (*MasterDataDecisions)(nil)
)

// GetBulkCandidate reads one open request with the facts the bulk approval
// validates: its version and whether it may be approved together with
// others. A request the caller could not decide reads as not found.
func (s *MasterDataDecisions) GetBulkCandidate(ctx context.Context, requestID int64) (*careplan.MasterDataReviewItem, error) {
	return s.bulkCandidate(ctx, requestID)
}

func (s *MasterDataDecisions) bulkCandidate(ctx context.Context, requestID int64) (*careplan.MasterDataReviewItem, error) {
	row, err := s.requests.FindStudentDataRequest(ctx, requestID, false)
	if errors.Is(err, careplan.ErrStudentDataRequestNotFound) {
		return nil, masterdatarequests.ErrReviewNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("review: load bulk candidate: %w", err)
	}
	if row.Status != masterdatarequests.StatusPending {
		return nil, masterdatarequests.ErrReviewNotFound
	}
	facts, err := s.people.ReviewFields(ctx, []ports.MasterDataFieldChange{{
		RequestID: row.ID, StudentID: row.StudentID, Target: row.Target, Field: row.FieldKey,
		OldValue: row.OldValue, NewValue: row.NewValue,
	}})
	if err != nil {
		return nil, err
	}
	scope, err := s.scope(ctx)
	if err != nil {
		return nil, fmt.Errorf("review: resolve reviewer scope: %w", err)
	}
	// The same gate as the open queue: a graduate or a child whose care ended
	// is not offered for a decision (#405 review, #2487).
	fact := facts[row.ID]
	if !scope.Allows(fact.Student) || fact.Student.Alumnus || fact.Student.CareEndedOn(s.today()) {
		return nil, masterdatarequests.ErrReviewNotFound
	}
	return &careplan.MasterDataReviewItem{
		Request: &row, FirstName: fact.FirstName, LastName: fact.LastName,
		BulkEligible: fact.BulkEligible, BulkIneligibleReason: fact.BulkIneligibleReason,
		BulkIneligibleText: fact.BulkIneligibleText, CurrentValueChanged: fact.CurrentValueChanged,
	}, nil
}

// LockBulkRequest takes the open request row FOR UPDATE.
func (s *MasterDataDecisions) LockBulkRequest(ctx context.Context, requestID int64) error {
	_, err := s.lockPendingRequest(ctx, requestID)
	return err
}

// LockBulkStudents establishes one canonical student-lock frontier after all
// request rows have been locked in request-kind/id order. Single decisions
// use the same request-before-student order.
func (s *MasterDataDecisions) LockBulkStudents(ctx context.Context, studentIDs []int64) error {
	for _, studentID := range studentIDs {
		if err := s.lockStudent(ctx, studentID); err != nil {
			return err
		}
	}
	return nil
}

func (s *MasterDataDecisions) lockStudent(ctx context.Context, studentID int64) error {
	_, err := s.records.LockStudent(ctx, studentID)
	return err
}

// ConflictCandidate is the bulk candidate reduced to what the resolver
// validates a group with.
func (s *MasterDataDecisions) ConflictCandidate(ctx context.Context, requestID int64) (*ports.ConflictCandidate, error) {
	item, err := s.bulkCandidate(ctx, requestID)
	if err != nil {
		return nil, err
	}
	return &ports.ConflictCandidate{StudentID: item.Request.StudentID, UpdatedAt: item.Request.UpdatedAt}, nil
}

// LockConflictRequest takes the open request row FOR UPDATE.
func (s *MasterDataDecisions) LockConflictRequest(ctx context.Context, requestID int64) error {
	_, err := s.lockPendingRequest(ctx, requestID)
	return err
}

// DecideConflictRequest takes one verdict of a resolve command through the
// ordinary decision, so every guard and effect of a single decision applies.
func (s *MasterDataDecisions) DecideConflictRequest(ctx context.Context, decision ports.ConflictDecision) error {
	_, err := s.Decide(ctx, masterdatarequests.DecideInput{
		RequestID: decision.RequestID, Approve: decision.Approve, Reason: decision.Reason,
		ReviewedBy: decision.ReviewerID, ExpectedVersion: decision.ExpectedVersion,
	})
	return err
}

// WriteStaffValue writes the staff member's own Stammdaten value for exactly
// the field the rejected wishes fought over. The field is read from the group
// rather than taken from the client, so a resolve can only ever write the
// value the conflict was about.
//
// Unlike an approval this does NOT compare against the request's baseline:
// the staff member is deliberately overriding whatever is there, and they are
// looking at the live value while they type it.
func (s *MasterDataDecisions) WriteStaffValue(ctx context.Context, write ports.StaffValueWrite) error {
	target, field, err := s.conflictGroupField(ctx, write.RequestIDs)
	if err != nil {
		return err
	}
	raw, err := staffValueJSON(write.Value)
	if err != nil {
		return err
	}
	switch target {
	case masterdatarequests.TargetStudent:
		return s.writeStaffStudentValue(ctx, write, field, raw)
	case masterdatarequests.TargetPerson:
		return s.writeStaffPersonValue(ctx, write.StudentID, field, raw)
	case masterdatarequests.TargetDeparture:
		return s.writeStaffDepartureValue(ctx, write, field, raw)
	default:
		return parentrequests.ErrStaffValueUnsupported
	}
}

// conflictGroupField reads the (target, field) the group shares. The requests
// are already rejected at this point, so they are loaded by id rather than as
// open candidates.
func (s *MasterDataDecisions) conflictGroupField(ctx context.Context, requestIDs []int64) (string, string, error) {
	if len(requestIDs) == 0 {
		return "", "", parentrequests.ErrStaffValueUnsupported
	}
	req, err := s.requests.FindStudentDataRequest(ctx, requestIDs[0], false)
	if err != nil {
		return "", "", fmt.Errorf("review: load conflict group field: %w", err)
	}
	return req.Target, req.FieldKey, nil
}

// staffValueJSON unwraps the kind-agnostic {"value": …} envelope the resolve
// route carries into the raw JSON the Stammdaten writers speak.
func staffValueJSON(value map[string]any) (json.RawMessage, error) {
	inner, ok := value["value"]
	if !ok {
		return nil, masterdatarequests.ErrReviewInvalidValue
	}
	raw, err := json.Marshal(inner)
	if err != nil {
		return nil, masterdatarequests.ErrReviewInvalidValue
	}
	return raw, nil
}

func (s *MasterDataDecisions) writeStaffStudentValue(ctx context.Context, write ports.StaffValueWrite, field string, raw json.RawMessage) error {
	if field != masterDataFieldSchoolClass {
		return parentrequests.ErrStaffValueUnsupported
	}
	value, err := decodeNonEmptyString(raw)
	if err != nil {
		return err
	}
	_, err = s.records.UpdateStudent(ctx, write.StudentID, write.ReviewerID, func(ports.MasterDataStudent) (ports.MasterDataStudentWrite, error) {
		return ports.MasterDataStudentWrite{SchoolClass: &value}, nil
	})
	return labelMasterDataWrite(err, "review: load student for staff value", "review: write staff student value", "review: audit staff student value")
}

func (s *MasterDataDecisions) writeStaffDepartureValue(ctx context.Context, write ports.StaffValueWrite, field string, raw json.RawMessage) error {
	if field != masterDataFieldDeparture {
		return parentrequests.ErrStaffValueUnsupported
	}
	modes, err := decodeRequestedDeparture(raw)
	if err != nil {
		return err
	}
	_, err = s.records.UpdateStudent(ctx, write.StudentID, write.ReviewerID, func(ports.MasterDataStudent) (ports.MasterDataStudentWrite, error) {
		return ports.MasterDataStudentWrite{DepartureModes: modes.Normalize()}, nil
	})
	return labelMasterDataWrite(err, "review: load student for staff value", "review: write staff student value", "review: audit staff student value")
}

func (s *MasterDataDecisions) writeStaffPersonValue(ctx context.Context, studentID int64, field string, raw json.RawMessage) error {
	value, err := decodeNonEmptyString(raw)
	if err != nil {
		return err
	}
	student, err := s.records.FindStudent(ctx, studentID)
	if err != nil {
		return fmt.Errorf("review: load student for staff value: %w", err)
	}
	err = s.records.UpdatePerson(ctx, student.PersonID, func(person *ports.MasterDataPerson) error {
		return setMasterDataPersonField(person, field, value, parentrequests.ErrStaffValueUnsupported)
	})
	return labelMasterDataWrite(err, "review: load person for staff value", "review: write staff person value", "review: audit staff person value")
}

func decodeNonEmptyString(raw json.RawMessage) (string, error) {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || value == "" {
		return "", masterdatarequests.ErrReviewInvalidValue
	}
	return value, nil
}
