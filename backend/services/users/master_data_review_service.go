package users

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
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
	// ErrReviewStaleValue means the live value no longer matches the request's
	// old_value baseline, so approval would overwrite a newer staff/import edit.
	ErrReviewStaleValue = errors.New("users: change request baseline changed")
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
	broadcaster       realtime.Broadcaster
	logger            *slog.Logger
}

// NewMasterDataReviewService wires the staff review service.
func NewMasterDataReviewService(
	changeRequestRepo userModels.StudentDataChangeRequestRepository,
	studentRepo userModels.StudentRepository,
	personRepo userModels.PersonRepository,
	logger *slog.Logger,
	broadcasters ...realtime.Broadcaster,
) MasterDataReviewService {
	if logger == nil {
		logger = slog.Default()
	}
	var broadcaster realtime.Broadcaster
	if len(broadcasters) > 0 {
		broadcaster = broadcasters[0]
	}
	return &masterDataReviewService{
		changeRequestRepo: changeRequestRepo,
		studentRepo:       studentRepo,
		personRepo:        personRepo,
		broadcaster:       broadcaster,
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
	req, err := s.changeRequestRepo.FindPendingByIDForUpdate(ctx, input.RequestID)
	if err != nil {
		if errors.Is(err, userModels.ErrChangeRequestNotFound) {
			return nil, ErrReviewNotFound
		}
		if errors.Is(err, userModels.ErrChangeRequestNotPending) {
			return nil, ErrReviewNotPending
		}
		return nil, fmt.Errorf("review: find pending request: %w", err)
	}

	var reason *string
	if trimmed := input.Reason; trimmed != "" {
		reason = &trimmed
	}

	if !input.Approve {
		if err := s.changeRequestRepo.Decide(ctx, req.ID, userModels.DataChangeStatusRejected, reason, input.ReviewedBy, false); err != nil {
			if errors.Is(err, userModels.ErrChangeRequestNotPending) {
				return nil, ErrReviewNotPending
			}
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
		if errors.Is(err, userModels.ErrChangeRequestNotPending) {
			return nil, ErrReviewNotPending
		}
		return nil, fmt.Errorf("review: approve: %w", err)
	}
	s.logger.Info("staff approved master data change",
		slog.Int64("request_id", req.ID),
		slog.Int64("student_id", req.StudentID),
		slog.String("target", req.Target),
		slog.String("field", req.FieldKey),
		slog.Int64("reviewed_by", input.ReviewedBy),
	)
	s.deferStudentUpdated(ctx, req.StudentID)
	return s.changeRequestRepo.FindByID(ctx, req.ID)
}

func (s *masterDataReviewService) deferStudentUpdated(ctx context.Context, studentID int64) {
	if s.broadcaster == nil {
		return
	}
	tenantID := tenant.FromContext(ctx)
	tenant.RegisterAfterCommit(ctx, func() {
		if tenantID <= 0 {
			s.logger.Warn("review: skipping student_updated broadcast without tenant",
				slog.Int64("student_id", studentID),
			)
			return
		}
		source := "master_data_review"
		event := realtime.NewEvent(realtime.EventStudentUpdated, "", realtime.EventData{Source: &source})
		if err := s.broadcaster.BroadcastToTenant(tenantID, event); err != nil {
			s.logger.Warn("review: failed to broadcast student update",
				slog.Int64("tenant_id", tenantID),
				slog.Int64("student_id", studentID),
				slog.String("error", err.Error()),
			)
		}
	})
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
	var value string
	if err := json.Unmarshal(req.NewValue, &value); err != nil {
		return ErrReviewInvalidValue
	}
	var parsedBirthday *timezone.Date
	if req.FieldKey == "birthday" {
		d, parseErr := timezone.ParseDate(value)
		if parseErr != nil {
			return ErrReviewInvalidValue
		}
		parsedBirthday = &d
	}

	student, err := s.studentRepo.FindByID(ctx, req.StudentID)
	if err != nil {
		return fmt.Errorf("review: load student: %w", err)
	}
	person, err := s.personRepo.FindByIDForUpdate(ctx, student.PersonID)
	if err != nil {
		return fmt.Errorf("review: load person: %w", err)
	}

	currentRaw, err := personFieldRaw(person, req.FieldKey)
	if err != nil {
		return err
	}
	if !jsonRawEqual(currentRaw, req.OldValue) {
		return ErrReviewStaleValue
	}

	switch req.FieldKey {
	case "first_name":
		person.FirstName = value
	case "last_name":
		person.LastName = value
	case "birthday":
		person.Birthday = parsedBirthday
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
	modes, err := decodeDepartureModes(req.NewValue)
	if err != nil {
		return err
	}
	if modes.HasMode(userModels.DepartureAccompanied) {
		return ErrReviewInvalidValue
	}
	student, err := s.studentRepo.FindByIDForUpdate(ctx, req.StudentID)
	if err != nil {
		return fmt.Errorf("review: load student: %w", err)
	}
	oldModes, err := decodeDepartureModes(req.OldValue)
	if err != nil {
		return err
	}
	if !departureModesEqual(student.AllowedDepartureModes.Normalize(), oldModes.Normalize()) {
		return ErrReviewStaleValue
	}
	// Setting AllowedDepartureModes makes StudentRepository.Update persist the
	// derived departure_days / pickup_days / bus_days columns too.
	student.AllowedDepartureModes = modes.Normalize()
	if err := s.studentRepo.Update(ctx, student); err != nil {
		return fmt.Errorf("review: update student departure: %w", err)
	}
	return nil
}

func personFieldRaw(person *userModels.Person, field string) (json.RawMessage, error) {
	switch field {
	case "first_name":
		return jsonString(person.FirstName), nil
	case "last_name":
		return jsonString(person.LastName), nil
	case "birthday":
		if person.Birthday == nil {
			return json.RawMessage("null"), nil
		}
		return jsonString(person.Birthday.String()), nil
	default:
		return nil, ErrReviewInvalidTarget
	}
}

func decodeDepartureModes(raw json.RawMessage) (userModels.AllowedDepartureModes, error) {
	var modes userModels.AllowedDepartureModes
	if err := json.Unmarshal(raw, &modes); err != nil {
		return nil, ErrReviewInvalidValue
	}
	if modes == nil {
		modes = userModels.AllowedDepartureModes{}
	}
	if err := modes.Validate(); err != nil {
		return nil, ErrReviewInvalidValue
	}
	return modes, nil
}

func departureModesEqual(a, b userModels.AllowedDepartureModes) bool {
	for _, day := range userModels.PickupDayOrder {
		left := a[day]
		right := b[day]
		if len(left) != len(right) {
			return false
		}
		for i := range left {
			if left[i] != right[i] {
				return false
			}
		}
	}
	return true
}

func jsonString(s string) json.RawMessage {
	b, _ := json.Marshal(s)
	return b
}

func jsonRawEqual(a, b json.RawMessage) bool {
	var left any
	var right any
	if err := json.Unmarshal(a, &left); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &right); err != nil {
		return false
	}
	return reflect.DeepEqual(left, right)
}
