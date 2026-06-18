package users

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// Staff-review sentinel errors, mapped to HTTP codes by the handler.
var (
	// ErrReviewNotFound means no change request matched the id in the tenant.
	ErrReviewNotFound = errors.New("users: change request not found")
	// ErrReviewNotPending means the request was already decided (lost race) or
	// is a Track A audit row that cannot be decided.
	ErrReviewNotPending = errors.New("users: change request is not pending")
	// ErrReviewInvalidTarget means the request's target/field is not one the
	// review service knows how to apply.
	ErrReviewInvalidTarget = errors.New("users: change request target is not applicable")
	// ErrReviewInvalidValue means the stored new_value could not be decoded when
	// applying an approval.
	ErrReviewInvalidValue = errors.New("users: change request value is invalid")
)

// MasterDataReviewItem is one pending request enriched with the child's name for
// the staff queue.
type MasterDataReviewItem struct {
	Request   *userModels.StudentDataChangeRequest
	FirstName string
	LastName  string
}

// MasterDataReviewDecideInput carries a staff decision on one change request.
type MasterDataReviewDecideInput struct {
	RequestID  int64
	Approve    bool
	Reason     string
	ReviewedBy int64
}

// MasterDataReviewService is the staff-facing review queue for parent Track B
// Stammdaten change requests. It runs inside the tenant transaction established
// by the request middleware, so all repo calls are tenant-scoped via RLS.
type MasterDataReviewService interface {
	// ListPending returns every pending change request for the current tenant,
	// newest-first, enriched with the child's name.
	ListPending(ctx context.Context) ([]*MasterDataReviewItem, error)
	// Decide approves (and applies) or rejects one pending request and returns
	// the refreshed row.
	Decide(ctx context.Context, input MasterDataReviewDecideInput) (*userModels.StudentDataChangeRequest, error)
}

type masterDataReviewService struct {
	changeRequestRepo userModels.StudentDataChangeRequestRepository
	studentRepo       userModels.StudentRepository
	personRepo        userModels.PersonRepository
	logger            *slog.Logger
}

// NewMasterDataReviewService wires the staff review service.
func NewMasterDataReviewService(
	changeRequestRepo userModels.StudentDataChangeRequestRepository,
	studentRepo userModels.StudentRepository,
	personRepo userModels.PersonRepository,
	logger *slog.Logger,
) MasterDataReviewService {
	if logger == nil {
		logger = slog.Default()
	}
	return &masterDataReviewService{
		changeRequestRepo: changeRequestRepo,
		studentRepo:       studentRepo,
		personRepo:        personRepo,
		logger:            logger,
	}
}

func (s *masterDataReviewService) ListPending(ctx context.Context) ([]*MasterDataReviewItem, error) {
	rows, err := s.changeRequestRepo.ListPendingForTenant(ctx)
	if err != nil {
		return nil, fmt.Errorf("review: list pending: %w", err)
	}
	if len(rows) == 0 {
		return []*MasterDataReviewItem{}, nil
	}

	studentIDs := make([]int64, 0, len(rows))
	seen := make(map[int64]struct{}, len(rows))
	for _, r := range rows {
		if _, ok := seen[r.StudentID]; !ok {
			seen[r.StudentID] = struct{}{}
			studentIDs = append(studentIDs, r.StudentID)
		}
	}
	students, err := s.studentRepo.FindByIDs(ctx, studentIDs)
	if err != nil {
		return nil, fmt.Errorf("review: load students: %w", err)
	}
	personIDs := make([]int64, 0, len(students))
	for _, st := range students {
		personIDs = append(personIDs, st.PersonID)
	}
	persons, err := s.personRepo.FindByIDs(ctx, personIDs)
	if err != nil {
		return nil, fmt.Errorf("review: load persons: %w", err)
	}

	items := make([]*MasterDataReviewItem, 0, len(rows))
	for _, r := range rows {
		item := &MasterDataReviewItem{Request: r}
		if st, ok := students[r.StudentID]; ok {
			if p, ok := persons[st.PersonID]; ok {
				item.FirstName = p.FirstName
				item.LastName = p.LastName
			}
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *masterDataReviewService) Decide(ctx context.Context, input MasterDataReviewDecideInput) (*userModels.StudentDataChangeRequest, error) {
	if input.RequestID <= 0 {
		return nil, ErrReviewNotFound
	}
	req, err := s.changeRequestRepo.FindByID(ctx, input.RequestID)
	if err != nil {
		return nil, ErrReviewNotFound
	}
	if req.Status != userModels.DataChangeStatusPending {
		return nil, ErrReviewNotPending
	}

	var reason *string
	if trimmed := input.Reason; trimmed != "" {
		reason = &trimmed
	}

	if !input.Approve {
		if err := s.changeRequestRepo.Decide(ctx, req.ID, userModels.DataChangeStatusRejected, reason, input.ReviewedBy, false); err != nil {
			return nil, fmt.Errorf("review: reject: %w", err)
		}
		s.logger.Info("staff rejected master data change",
			slog.Int64("request_id", req.ID),
			slog.Int64("student_id", req.StudentID),
			slog.Int64("reviewed_by", input.ReviewedBy),
		)
		return s.changeRequestRepo.FindByID(ctx, req.ID)
	}

	if err := s.applyApprovedChange(ctx, req); err != nil {
		return nil, err
	}
	if err := s.changeRequestRepo.Decide(ctx, req.ID, userModels.DataChangeStatusApproved, reason, input.ReviewedBy, true); err != nil {
		return nil, fmt.Errorf("review: approve: %w", err)
	}
	s.logger.Info("staff approved master data change",
		slog.Int64("request_id", req.ID),
		slog.Int64("student_id", req.StudentID),
		slog.String("target", req.Target),
		slog.String("field", req.FieldKey),
		slog.Int64("reviewed_by", input.ReviewedBy),
	)
	return s.changeRequestRepo.FindByID(ctx, req.ID)
}

// applyApprovedChange writes the request's new_value to the live record.
func (s *masterDataReviewService) applyApprovedChange(ctx context.Context, req *userModels.StudentDataChangeRequest) error {
	switch req.Target {
	case userModels.DataChangeTargetPerson:
		return s.applyPersonChange(ctx, req)
	case userModels.DataChangeTargetDeparture:
		return s.applyDepartureChange(ctx, req)
	default:
		return ErrReviewInvalidTarget
	}
}

func (s *masterDataReviewService) applyPersonChange(ctx context.Context, req *userModels.StudentDataChangeRequest) error {
	student, err := s.studentRepo.FindByID(ctx, req.StudentID)
	if err != nil {
		return fmt.Errorf("review: load student: %w", err)
	}
	person, err := s.personRepo.FindByID(ctx, student.PersonID)
	if err != nil {
		return fmt.Errorf("review: load person: %w", err)
	}
	var value string
	if err := json.Unmarshal(req.NewValue, &value); err != nil {
		return ErrReviewInvalidValue
	}
	switch req.FieldKey {
	case "first_name":
		person.FirstName = value
	case "last_name":
		person.LastName = value
	case "birthday":
		d, parseErr := timezone.ParseDate(value)
		if parseErr != nil {
			return ErrReviewInvalidValue
		}
		person.Birthday = &d
	default:
		return ErrReviewInvalidTarget
	}
	if err := s.personRepo.Update(ctx, person); err != nil {
		return fmt.Errorf("review: update person: %w", err)
	}
	return nil
}

func (s *masterDataReviewService) applyDepartureChange(ctx context.Context, req *userModels.StudentDataChangeRequest) error {
	if req.FieldKey != "allowed_departure_modes" {
		return ErrReviewInvalidTarget
	}
	var modes userModels.AllowedDepartureModes
	if err := json.Unmarshal(req.NewValue, &modes); err != nil {
		return ErrReviewInvalidValue
	}
	student, err := s.studentRepo.FindByIDForUpdate(ctx, req.StudentID)
	if err != nil {
		return fmt.Errorf("review: load student: %w", err)
	}
	// Setting AllowedDepartureModes makes StudentRepository.Update persist the
	// derived departure_days / pickup_days / bus_days columns too.
	student.AllowedDepartureModes = modes.Normalize()
	if err := s.studentRepo.Update(ctx, student); err != nil {
		return fmt.Errorf("review: update student departure: %w", err)
	}
	return nil
}
