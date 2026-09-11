package users

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// Stammdaten side of the conflict resolver (#2267, stories 6-10). The
// coordinator owns the group, this file owns the payload — the same split the
// bulk-approval ports use.

var _ ParentRequestConflictPort = (*masterDataReviewService)(nil)

func (s *masterDataReviewService) ConflictCandidate(
	ctx context.Context, requestID int64,
) (*ParentRequestConflictCandidate, error) {
	item, err := s.GetBulkCandidate(ctx, requestID)
	if err != nil {
		return nil, err
	}
	return &ParentRequestConflictCandidate{
		StudentID: item.Request.StudentID,
		UpdatedAt: item.Request.UpdatedAt,
	}, nil
}

func (s *masterDataReviewService) LockConflictRequest(ctx context.Context, requestID int64) error {
	return s.LockBulkRequest(ctx, requestID)
}

func (s *masterDataReviewService) DecideConflictRequest(
	ctx context.Context, decision ParentRequestConflictDecision,
) error {
	_, err := s.Decide(ctx, MasterDataReviewDecideInput{
		RequestID:       decision.RequestID,
		Approve:         decision.Approve,
		Reason:          decision.Reason,
		ReviewedBy:      decision.ReviewerID,
		ExpectedVersion: decision.ExpectedVersion,
	})
	return err
}

// WriteStaffValue writes the staff member's own Stammdaten value for exactly
// the field the rejected wishes fought over. The field is read from the group
// rather than taken from the client, so a resolve can only ever write the
// value the conflict was about.
//
// Unlike an approval this does NOT compare against the request's old_value:
// the staff member is deliberately overriding whatever is there, and they are
// looking at the live value while they type it.
func (s *masterDataReviewService) WriteStaffValue(
	ctx context.Context, write ParentRequestStaffValueWrite,
) error {
	target, field, err := s.conflictGroupField(ctx, write.RequestIDs)
	if err != nil {
		return err
	}
	raw, err := staffValueJSON(write.Value)
	if err != nil {
		return err
	}
	switch target {
	case userModels.DataChangeTargetStudent:
		return s.writeStaffStudentValue(ctx, write, field, raw)
	case userModels.DataChangeTargetPerson:
		return s.writeStaffPersonValue(ctx, write.StudentID, field, raw)
	case userModels.DataChangeTargetDeparture:
		return s.writeStaffDepartureValue(ctx, write, field, raw)
	default:
		return ErrStaffValueUnsupported
	}
}

// conflictGroupField reads the (target, field) the group shares. The requests
// are already rejected at this point, so they are loaded by id rather than as
// pending candidates.
func (s *masterDataReviewService) conflictGroupField(
	ctx context.Context, requestIDs []int64,
) (string, string, error) {
	if len(requestIDs) == 0 {
		return "", "", ErrStaffValueUnsupported
	}
	req, err := s.changeRequestRepo.FindByID(ctx, requestIDs[0])
	if err != nil {
		return "", "", fmt.Errorf("review: load conflict group field: %w", err)
	}
	if req == nil {
		return "", "", ErrReviewNotFound
	}
	return req.Target, req.FieldKey, nil
}

// staffValueJSON unwraps the domain-agnostic {"value": …} envelope the resolve
// route carries into the raw JSON the Stammdaten writers already speak.
func staffValueJSON(value map[string]any) (json.RawMessage, error) {
	inner, ok := value["value"]
	if !ok {
		return nil, ErrReviewInvalidValue
	}
	raw, err := json.Marshal(inner)
	if err != nil {
		return nil, ErrReviewInvalidValue
	}
	return raw, nil
}

func (s *masterDataReviewService) writeStaffStudentValue(
	ctx context.Context, write ParentRequestStaffValueWrite, field string, raw json.RawMessage,
) error {
	if field != "school_class" {
		return ErrStaffValueUnsupported
	}
	value, err := decodeNonEmptyString(raw)
	if err != nil {
		return err
	}
	student, err := s.studentRepo.FindByIDForUpdate(ctx, write.StudentID)
	if err != nil {
		return fmt.Errorf("review: load student for staff value: %w", err)
	}
	before := *student
	student.SchoolClass = value
	return s.persistStaffStudentValue(ctx, &before, student, write.ReviewerID)
}

func (s *masterDataReviewService) writeStaffDepartureValue(
	ctx context.Context, write ParentRequestStaffValueWrite, field string, raw json.RawMessage,
) error {
	if field != "allowed_departure_modes" {
		return ErrStaffValueUnsupported
	}
	modes, err := decodeDepartureModes(raw)
	if err != nil {
		return err
	}
	if modes.HasMode(userModels.DepartureAccompanied) {
		return ErrReviewInvalidValue
	}
	student, err := s.studentRepo.FindByIDForUpdate(ctx, write.StudentID)
	if err != nil {
		return fmt.Errorf("review: load student for staff value: %w", err)
	}
	before := *student
	// Setting AllowedDepartureModes makes StudentRepository.Update persist the
	// derived departure_days / pickup_days / bus_days columns too.
	student.AllowedDepartureModes = modes.Normalize()
	return s.persistStaffStudentValue(ctx, &before, student, write.ReviewerID)
}

func (s *masterDataReviewService) persistStaffStudentValue(
	ctx context.Context, before, student *userModels.Student, reviewerID int64,
) error {
	if err := s.studentRepo.Update(ctx, student); err != nil {
		return fmt.Errorf("review: write staff student value: %w", err)
	}
	if s.studentAudit == nil {
		return nil
	}
	if err := s.studentAudit.RecordChangesForActor(ctx, before, student, reviewerID); err != nil {
		return fmt.Errorf("review: audit staff student value: %w", err)
	}
	return nil
}

func (s *masterDataReviewService) writeStaffPersonValue(
	ctx context.Context, studentID int64, field string, raw json.RawMessage,
) error {
	value, err := decodeNonEmptyString(raw)
	if err != nil {
		return err
	}
	student, err := s.studentRepo.FindByID(ctx, studentID)
	if err != nil {
		return fmt.Errorf("review: load student for staff value: %w", err)
	}
	person, err := s.personRepo.FindByIDForUpdate(ctx, student.PersonID)
	if err != nil {
		return fmt.Errorf("review: load person for staff value: %w", err)
	}
	switch field {
	case "first_name":
		person.FirstName = value
	case "last_name":
		person.LastName = value
	case "birthday":
		birthday, parseErr := timezone.ParseDate(value)
		if parseErr != nil {
			return ErrReviewInvalidValue
		}
		person.Birthday = &birthday
	default:
		return ErrStaffValueUnsupported
	}
	if err := s.personRepo.Update(ctx, person); err != nil {
		return fmt.Errorf("review: write staff person value: %w", err)
	}
	return nil
}

func decodeNonEmptyString(raw json.RawMessage) (string, error) {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", ErrReviewInvalidValue
	}
	if value == "" {
		return "", ErrReviewInvalidValue
	}
	return value, nil
}
