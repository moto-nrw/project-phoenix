package care

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/careplan/excusedrequests"

	"github.com/moto-nrw/project-phoenix/modules/careplan/parentrequests"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// MasterDataFieldChange is one proposed Track B change submitted by a parent.
type MasterDataFieldChange struct {
	Target   string
	FieldKey string
	Value    json.RawMessage
}

// trackBRequestableFields is the allowlist of approval-required (Track B)
// fields. These are the high-stakes / OGS-coupled fields a parent may only
// propose, not apply directly.
var trackBRequestableFields = map[string]bool{
	DataChangeTargetPerson + "/first_name":                 true,
	DataChangeTargetPerson + "/last_name":                  true,
	DataChangeTargetPerson + "/birthday":                   true,
	DataChangeTargetStudent + "/school_class":              true,
	DataChangeTargetDeparture + "/allowed_departure_modes": true,
}

// SubmitMasterDataChangeRequest records one or more pending Track B change
// requests for staff approval. Fields whose proposed value equals the current
// value are skipped; a field with an already-pending request is rejected.
func (s *Service) SubmitMasterDataChangeRequest(ctx context.Context, accountID, studentID int64, changes []MasterDataFieldChange, recipientGuardianProfileIDs []int64) ([]*StudentDataChangeRequest, error) {
	if len(changes) == 0 {
		return nil, ErrMasterDataNoChanges
	}
	for _, c := range changes {
		if !trackBRequestableFields[c.Target+"/"+c.FieldKey] {
			return nil, ErrMasterDataFieldNotEditable
		}
	}

	child, err := s.ResolvePermittedChild(ctx, accountID, studentID, GuardianPermissionMasterDataRequest)
	if err != nil {
		return nil, err
	}
	// A child whose care at this school has ended keeps read access to what
	// happened, but nothing new can be submitted for them (#2487).
	if err := child.RequireCareRunning(); err != nil {
		return nil, err
	}

	enabled, err := s.Settings.ResolveBoolForTenant(ctx, child.TenantID, KeyParentMasterDataRequestEnabled)
	if err != nil {
		return nil, fmt.Errorf("parent: resolve master-data request setting: %w", err)
	}
	if !enabled {
		return nil, ErrMasterDataRequestDisabled
	}

	submission := masterDataSubmission{accountID: accountID, studentID: studentID, tenantID: child.TenantID, recipients: recipientGuardianProfileIDs}
	var created []*StudentDataChangeRequest
	txErr := s.Runtime.WithinTenant(ctx, child.TenantID, func(txCtx context.Context) error {
		rows, submitErr := s.submitMasterDataInTx(txCtx, submission, changes)
		created = rows
		return submitErr
	})
	if txErr != nil {
		return nil, fmt.Errorf("parent: submit master data change request: %w", txErr)
	}
	if len(created) == 0 {
		return nil, ErrMasterDataNoChanges
	}

	s.Logger.Info("parent submitted master data change request",
		slog.Int64("account_id", accountID),
		slog.Int64("student_id", studentID),
		slog.Int64("tenant_id", child.TenantID),
		slog.Int("fields", len(created)),
	)
	return created, nil
}

// masterDataSubmission is one Track B submission of a guardian for a child.
type masterDataSubmission struct {
	accountID  int64
	studentID  int64
	tenantID   int64
	recipients []int64
}

// submitMasterDataInTx records one pending request per changed field.
func (s *Service) submitMasterDataInTx(ctx context.Context, submission masterDataSubmission, changes []MasterDataFieldChange) ([]*StudentDataChangeRequest, error) {
	// The permission check above used a snapshot. Acquire the same student
	// row lock as a care exit and re-check its interval before inserting any
	// pending request, so an exit cannot commit in the gap before this write.
	student, err := s.StudentRepo.FindByIDForUpdate(ctx, submission.studentID)
	if err != nil {
		return nil, err
	}
	if student.CareEndedOn(s.todayDate()) {
		return nil, ErrChildCareEnded
	}
	person, err := s.PersonRepo.FindByID(ctx, student.PersonID)
	if err != nil {
		return nil, err
	}

	var created []*StudentDataChangeRequest
	for _, c := range changes {
		oldRaw, newRaw, changed, fieldErr := trackBFieldState(c.Target, c.FieldKey, MasterDataPersonValues(person), MasterDataStudentValues(student), c.Value, s.DeparturePolicy)
		if fieldErr != nil {
			return nil, fieldErr
		}
		if !changed {
			continue
		}
		row, err := s.submitMasterDataField(ctx, submission, c, oldRaw, newRaw)
		if err != nil {
			return nil, err
		}
		created = append(created, row)
	}
	if len(created) > 0 {
		if err := s.emitMasterDataRequestPills(ctx, submission, created); err != nil {
			return nil, err
		}
	}
	return created, nil
}

// submitMasterDataField records one pending field request through Care Plan,
// its ledger event and the family's share choice, in the caller's unit of
// work.
func (s *Service) submitMasterDataField(ctx context.Context, submission masterDataSubmission, c MasterDataFieldChange, oldRaw, newRaw json.RawMessage) (*StudentDataChangeRequest, error) {
	has, err := s.ChangeRequestRepo.HasPendingForField(ctx, submission.studentID, c.Target, c.FieldKey)
	if err != nil {
		return nil, err
	}
	if has {
		return nil, ErrMasterDataDuplicatePending
	}
	row := &StudentDataChangeRequest{
		StudentID:   submission.studentID,
		SubmittedBy: submission.accountID,
		Target:      c.Target,
		FieldKey:    c.FieldKey,
		OldValue:    oldRaw,
		NewValue:    newRaw,
		Status:      DataChangeStatusPending,
	}
	row.TenantID = submission.tenantID
	if err := s.createDataRequest(ctx, row); err != nil {
		if s.isPendingChangeRequestUniqueViolation(err) {
			return nil, ErrMasterDataDuplicatePending
		}
		return nil, err
	}
	if err := RecordRequestEvent(ctx, s.ParentRequestEvents, RequestLedgerEntry{
		StudentID:      submission.studentID,
		RequestType:    ParentRequestTypeMasterData,
		RequestID:      row.ID,
		EventType:      ParentRequestEventSubmitted,
		ActorAccountID: submission.accountID,
		UpdatedAt:      row.UpdatedAt,
		Payload:        dataRequestEventPayload(c.Target, c.FieldKey),
	}); err != nil {
		return nil, err
	}
	// Same transaction as the request row: a refused share rolls the whole
	// submit back rather than leaving requests nobody the family picked can
	// see (#2267). A multi-field submit shares every row it created — the
	// guardian chose recipients for the submission, not for one field of it.
	if err := s.RequestSharing.ShareRequestInTx(
		ctx, submission.accountID, submission.studentID, RequestShareMasterData, row.ID, submission.recipients,
	); err != nil {
		return nil, err
	}
	return row, nil
}

// emitMasterDataRequestPills posts one "Anfrage erstellt" pill PER created
// row, each referencing its OWN row. A multi-field submit is decided
// field-by-field (master_data_review.Decide runs per row and emits a decision
// pill keyed on THAT row's id), so per-row created pills keep every
// created↔decision ref paired: the thread timeline and the deep-link into the
// Änderungsanfragen queue resolve the exact row for any field, not just
// created[0]. Best-effort, after commit.
func (s *Service) emitMasterDataRequestPills(ctx context.Context, submission masterDataSubmission, created []*StudentDataChangeRequest) error {
	refIDs := make([]int64, len(created))
	for i, row := range created {
		refIDs[i] = row.ID
	}
	if s.Emitter == nil {
		return nil
	}
	for i := range refIDs {
		refID := refIDs[i]
		if err := s.Emitter.EmitChildEvent(ctx, submission.tenantID, submission.studentID, submission.accountID, ChildEvent{
			EventType:      "request_created",
			ActorKind:      ParentMessageSenderGuardian,
			ActorAccountID: submission.accountID,
			Body:           "Anfrage: Stammdaten ändern",
			RequestType:    ParentMessageRequestMasterData,
			RequestStatus:  ParentMessageRequestStatusOpen,
			RefTable:       "users.student_data_change_requests",
			RefID:          &refID,
		}); err != nil {
			return err
		}
	}
	return nil
}

// ListMyMasterDataRequests returns child-level Track B change requests,
// newest-first. It deliberately excludes Track A guardian contact audit rows so
// one linked parent cannot read another guardian's private email/phone/address
// old/new values.
func (s *Service) ListMyMasterDataRequests(ctx context.Context, accountID, studentID int64) ([]*StudentDataChangeRequest, error) {
	child, err := s.ResolveOwnedChild(ctx, accountID, studentID)
	if err != nil {
		return nil, err
	}
	var out []*StudentDataChangeRequest
	txErr := s.Runtime.WithinTenant(ctx, child.TenantID, func(txCtx context.Context) error {
		rows, listErr := s.ChangeRequestRepo.ListParentVisibleByStudent(txCtx, studentID, 0)
		if listErr != nil {
			return listErr
		}
		visibility, visibilityErr := s.RequestSharing.LoadRequestShareVisibility(txCtx, studentID)
		if visibilityErr != nil {
			return visibilityErr
		}
		out = make([]*StudentDataChangeRequest, 0, len(rows))
		for _, row := range rows {
			if row != nil && visibility.Allows(RequestShareMasterData, row.ID, accountID, row.SubmittedBy) {
				out = append(out, row)
			}
		}
		return nil
	})
	if txErr != nil {
		return nil, fmt.Errorf("parent: list master data requests: %w", txErr)
	}
	return out, nil
}

// trackBFieldState validates the proposed value, computes the current value as
// JSON for the audit row, and reports whether the value actually changes.
// EditMasterDataRequest rewrites the proposed value of the caller's own
// still-pending Stammdaten request (#2267, story 37). It replaces the withdraw
// flow, so the request keeps its id, its share and its history. Target and
// field stay fixed — changing those is a different request.
func (s *Service) EditMasterDataRequest(
	ctx context.Context, accountID, studentID, requestID int64, newValue json.RawMessage, expectedVersion string,
) (*StudentDataChangeRequest, error) {
	child, err := s.ResolvePermittedChild(ctx, accountID, studentID, GuardianPermissionMasterDataRequest)
	if err != nil {
		return nil, err
	}
	if err := child.RequireCareRunning(); err != nil {
		return nil, err
	}
	var out *StudentDataChangeRequest
	txErr := s.Runtime.WithinTenant(ctx, child.TenantID, func(txCtx context.Context) error {
		row, editErr := s.editMasterDataRequestInTx(txCtx, accountID, studentID, requestID, newValue, expectedVersion)
		if editErr != nil {
			return editErr
		}
		out = row
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}
	return out, nil
}

func (s *Service) editMasterDataRequestInTx(
	ctx context.Context, accountID, studentID, requestID int64, newValue json.RawMessage, expectedVersion string,
) (*StudentDataChangeRequest, error) {
	req, err := s.ChangeRequestRepo.FindByIDForUpdate(ctx, requestID)
	if err != nil {
		return nil, err
	}
	// A request that is not the caller's own is reported as missing, never as
	// forbidden: a stranger must not learn that the id exists.
	if req.SubmittedBy != accountID || req.StudentID != studentID {
		return nil, ErrMasterDataRequestNotFound
	}
	if req.Status != DataChangeStatusPending {
		return nil, ErrMasterDataRequestNotPending
	}
	if expectedVersion != "" && excusedrequests.ParentRequestVersion(req.UpdatedAt) != expectedVersion {
		return nil, parentrequests.ErrStale
	}
	student, err := s.StudentRepo.FindByIDForUpdate(ctx, studentID)
	if err != nil {
		return nil, err
	}
	if student.CareEndedOn(s.todayDate()) {
		return nil, ErrChildCareEnded
	}
	person, err := s.PersonRepo.FindByID(ctx, student.PersonID)
	if err != nil {
		return nil, err
	}
	// Same validator the create path runs, against the CURRENT record: an edit
	// may not propose a value the submit path would have refused, and one that
	// meanwhile equals the live value is no request at all.
	_, newRaw, changed, err := trackBFieldState(req.Target, req.FieldKey, MasterDataPersonValues(person), MasterDataStudentValues(student), newValue, s.DeparturePolicy)
	if err != nil {
		return nil, err
	}
	if !changed {
		return nil, ErrMasterDataNoChanges
	}
	if err := s.updatePendingDataRequest(ctx, req.ID, newRaw); err != nil {
		return nil, err
	}
	row, err := s.ChangeRequestRepo.FindByID(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	if err := RecordRequestEvent(ctx, s.ParentRequestEvents, RequestLedgerEntry{
		StudentID:      row.StudentID,
		RequestType:    ParentRequestTypeMasterData,
		RequestID:      row.ID,
		EventType:      ParentRequestEventGuardianEdit,
		ActorAccountID: accountID,
		UpdatedAt:      row.UpdatedAt,
		Payload:        dataRequestEventPayload(row.Target, row.FieldKey),
	}); err != nil {
		return nil, err
	}
	return row, nil
}

func trackBFieldState(target, field string, person *MasterDataPerson, student *MasterDataStudent, value json.RawMessage, policy DeparturePolicy) (oldRaw, newRaw json.RawMessage, changed bool, err error) {
	switch target {
	case DataChangeTargetPerson:
		return trackBPersonFieldState(field, person, value)
	case DataChangeTargetStudent:
		return trackBStudentFieldState(field, student, value)
	case DataChangeTargetDeparture:
		if field != "allowed_departure_modes" {
			return nil, nil, false, ErrMasterDataFieldNotEditable
		}
		return trackBDepartureState(student, value, policy)
	default:
		return nil, nil, false, ErrMasterDataFieldNotEditable
	}
}

func trackBStudentFieldState(field string, student *MasterDataStudent, value json.RawMessage) (json.RawMessage, json.RawMessage, bool, error) {
	if field != "school_class" {
		return nil, nil, false, ErrMasterDataFieldNotEditable
	}
	newClass, err := decodeStringValue(value)
	if err != nil || newClass == "" {
		return nil, nil, false, ErrMasterDataInvalidValue
	}
	return jsonString(student.SchoolClass), jsonString(newClass), newClass != student.SchoolClass, nil
}

func trackBPersonFieldState(field string, person *MasterDataPerson, value json.RawMessage) (json.RawMessage, json.RawMessage, bool, error) {
	newStr, decErr := decodeStringValue(value)
	if decErr != nil {
		return nil, nil, false, decErr
	}
	switch field {
	case "first_name", "last_name":
		if newStr == "" {
			return nil, nil, false, ErrMasterDataInvalidValue
		}
		old := person.FirstName
		if field == "last_name" {
			old = person.LastName
		}
		return jsonString(old), jsonString(newStr), newStr != old, nil
	case "birthday":
		if newStr == "" {
			return nil, nil, false, ErrMasterDataInvalidValue
		}
		if _, parseErr := calendar.ParseDate(newStr); parseErr != nil {
			return nil, nil, false, ErrMasterDataInvalidValue
		}
		oldStr := ""
		if person.Birthday != nil {
			oldStr = person.Birthday.String()
		}
		return jsonStringPtr(birthdayPtr(person)), jsonString(newStr), newStr != oldStr, nil
	default:
		return nil, nil, false, ErrMasterDataFieldNotEditable
	}
}

func birthdayPtr(person *MasterDataPerson) *string {
	if person.Birthday == nil {
		return nil
	}
	s := person.Birthday.String()
	return &s
}

func trackBDepartureState(student *MasterDataStudent, value json.RawMessage, policy DeparturePolicy) (json.RawMessage, json.RawMessage, bool, error) {
	next, err := policy.Inspect(value, true)
	if err != nil {
		return nil, nil, false, ErrMasterDataInvalidValue
	}
	currentRaw, err := json.Marshal(student.AllowedDepartureModes)
	if err != nil {
		return nil, nil, false, ErrMasterDataInvalidValue
	}
	current, err := policy.Inspect(currentRaw, false)
	if err != nil {
		return nil, nil, false, ErrMasterDataInvalidValue
	}
	if next.Accompanied || current.Accompanied {
		return nil, nil, false, ErrMasterDataInvalidValue
	}
	return current.Normalized, next.Normalized, !bytes.Equal(next.Normalized, current.Normalized), nil
}

// isPendingChangeRequestUniqueViolation reports Care Plan's refusal of a
// second pending request for the same field (the partial unique index on
// users.student_data_change_requests).
func (s *Service) isPendingChangeRequestUniqueViolation(err error) bool {
	return errors.Is(err, s.CarePlanPolicy.Error(CarePlanStudentDataRequestFieldPending))
}
