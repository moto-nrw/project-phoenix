package care

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/parentrequests"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	"github.com/moto-nrw/project-phoenix/tenant"
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
	usersModels.DataChangeTargetPerson + "/first_name":                 true,
	usersModels.DataChangeTargetPerson + "/last_name":                  true,
	usersModels.DataChangeTargetPerson + "/birthday":                   true,
	usersModels.DataChangeTargetStudent + "/school_class":              true,
	usersModels.DataChangeTargetDeparture + "/allowed_departure_modes": true,
}

// SubmitMasterDataChangeRequest records one or more pending Track B change
// requests for staff approval. Fields whose proposed value equals the current
// value are skipped; a field with an already-pending request is rejected.
func (s *Service) SubmitMasterDataChangeRequest(ctx context.Context, accountID, studentID int64, changes []MasterDataFieldChange, recipientGuardianProfileIDs []int64) ([]*usersModels.StudentDataChangeRequest, error) {
	if len(changes) == 0 {
		return nil, ErrMasterDataNoChanges
	}
	for _, c := range changes {
		if !trackBRequestableFields[c.Target+"/"+c.FieldKey] {
			return nil, ErrMasterDataFieldNotEditable
		}
	}

	child, err := s.ResolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionMasterDataRequest)
	if err != nil {
		return nil, err
	}
	// A child whose care at this school has ended keeps read access to what
	// happened, but nothing new can be submitted for them (#2487).
	if err := child.RequireCareRunning(); err != nil {
		return nil, err
	}

	enabled, err := s.Settings.ResolveBoolForTenant(ctx, child.TenantID, configModels.KeyParentMasterDataRequestEnabled)
	if err != nil {
		return nil, fmt.Errorf("parent: resolve master-data request setting: %w", err)
	}
	if !enabled {
		return nil, ErrMasterDataRequestDisabled
	}

	submission := masterDataSubmission{accountID: accountID, studentID: studentID, tenantID: child.TenantID, recipients: recipientGuardianProfileIDs}
	var created []*usersModels.StudentDataChangeRequest
	txErr := InTenant(ctx, child.TenantID, func(txCtx context.Context) error {
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
func (s *Service) submitMasterDataInTx(ctx context.Context, submission masterDataSubmission, changes []MasterDataFieldChange) ([]*usersModels.StudentDataChangeRequest, error) {
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

	var created []*usersModels.StudentDataChangeRequest
	for _, c := range changes {
		oldRaw, newRaw, changed, fieldErr := trackBFieldState(c.Target, c.FieldKey, person, student, c.Value)
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
		s.emitMasterDataRequestPills(ctx, submission, created)
	}
	return created, nil
}

// submitMasterDataField records one pending field request through Care Plan,
// its ledger event and the family's share choice, in the caller's unit of
// work.
func (s *Service) submitMasterDataField(ctx context.Context, submission masterDataSubmission, c MasterDataFieldChange, oldRaw, newRaw json.RawMessage) (*usersModels.StudentDataChangeRequest, error) {
	has, err := s.ChangeRequestRepo.HasPendingForField(ctx, submission.studentID, c.Target, c.FieldKey)
	if err != nil {
		return nil, err
	}
	if has {
		return nil, ErrMasterDataDuplicatePending
	}
	row := &usersModels.StudentDataChangeRequest{
		StudentID:   submission.studentID,
		SubmittedBy: submission.accountID,
		Target:      c.Target,
		FieldKey:    c.FieldKey,
		OldValue:    oldRaw,
		NewValue:    newRaw,
		Status:      usersModels.DataChangeStatusPending,
	}
	row.SetTenantID(submission.tenantID)
	if err := s.createDataRequest(ctx, row); err != nil {
		if isPendingChangeRequestUniqueViolation(err) {
			return nil, ErrMasterDataDuplicatePending
		}
		return nil, err
	}
	if err := RecordRequestEvent(ctx, s.ParentRequestEvents, RequestLedgerEntry{
		StudentID:      submission.studentID,
		RequestType:    usersModels.ParentRequestTypeMasterData,
		RequestID:      row.ID,
		EventType:      usersModels.ParentRequestEventSubmitted,
		ActorAccountID: submission.accountID,
		UpdatedAt:      row.UpdatedAt,
		Payload:        map[string]any{"target": c.Target, "field": c.FieldKey},
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
func (s *Service) emitMasterDataRequestPills(ctx context.Context, submission masterDataSubmission, created []*usersModels.StudentDataChangeRequest) {
	refIDs := make([]int64, len(created))
	for i, row := range created {
		refIDs[i] = row.ID
	}
	tenant.RegisterAfterCommit(ctx, func() {
		if s.Emitter == nil {
			return
		}
		for i := range refIDs {
			refID := refIDs[i]
			s.Emitter.EmitChildEvent(submission.tenantID, submission.studentID, submission.accountID, parentmessaging.ChildEvent{
				EventType:      "request_created",
				ActorKind:      usersModels.ParentMessageSenderGuardian,
				ActorAccountID: submission.accountID,
				Body:           "Anfrage: Stammdaten ändern",
				RequestType:    usersModels.ParentMessageRequestMasterData,
				RequestStatus:  usersModels.ParentMessageRequestStatusOpen,
				RefTable:       "users.student_data_change_requests",
				RefID:          &refID,
			})
		}
	})
}

// ListMyMasterDataRequests returns child-level Track B change requests,
// newest-first. It deliberately excludes Track A guardian contact audit rows so
// one linked parent cannot read another guardian's private email/phone/address
// old/new values.
func (s *Service) ListMyMasterDataRequests(ctx context.Context, accountID, studentID int64) ([]*usersModels.StudentDataChangeRequest, error) {
	child, err := s.ResolveOwnedChild(ctx, accountID, studentID)
	if err != nil {
		return nil, err
	}
	var out []*usersModels.StudentDataChangeRequest
	txErr := InTenant(ctx, child.TenantID, func(txCtx context.Context) error {
		rows, listErr := s.ChangeRequestRepo.ListParentVisibleByStudent(txCtx, studentID, 0)
		if listErr != nil {
			return listErr
		}
		visibility, visibilityErr := s.RequestSharing.LoadRequestShareVisibility(txCtx, studentID)
		if visibilityErr != nil {
			return visibilityErr
		}
		out = make([]*usersModels.StudentDataChangeRequest, 0, len(rows))
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
) (*usersModels.StudentDataChangeRequest, error) {
	child, err := s.ResolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionMasterDataRequest)
	if err != nil {
		return nil, err
	}
	if err := child.RequireCareRunning(); err != nil {
		return nil, err
	}
	var out *usersModels.StudentDataChangeRequest
	txErr := InTenant(ctx, child.TenantID, func(txCtx context.Context) error {
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
) (*usersModels.StudentDataChangeRequest, error) {
	req, err := s.ChangeRequestRepo.FindByIDForUpdate(ctx, requestID)
	if err != nil {
		return nil, err
	}
	// A request that is not the caller's own is reported as missing, never as
	// forbidden: a stranger must not learn that the id exists.
	if req.SubmittedBy != accountID || req.StudentID != studentID {
		return nil, usersModels.ErrChangeRequestNotFound
	}
	if req.Status != usersModels.DataChangeStatusPending {
		return nil, usersModels.ErrChangeRequestNotPending
	}
	if expectedVersion != "" && careplan.ParentRequestVersion(req.UpdatedAt) != expectedVersion {
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
	_, newRaw, changed, err := trackBFieldState(req.Target, req.FieldKey, person, student, newValue)
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
		RequestType:    usersModels.ParentRequestTypeMasterData,
		RequestID:      row.ID,
		EventType:      usersModels.ParentRequestEventGuardianEdit,
		ActorAccountID: accountID,
		UpdatedAt:      row.UpdatedAt,
		Payload:        map[string]any{"target": row.Target, "field": row.FieldKey},
	}); err != nil {
		return nil, err
	}
	return row, nil
}

func trackBFieldState(target, field string, person *usersModels.Person, student *usersModels.Student, value json.RawMessage) (oldRaw, newRaw json.RawMessage, changed bool, err error) {
	switch target {
	case usersModels.DataChangeTargetPerson:
		return trackBPersonFieldState(field, person, value)
	case usersModels.DataChangeTargetStudent:
		return trackBStudentFieldState(field, student, value)
	case usersModels.DataChangeTargetDeparture:
		if field != "allowed_departure_modes" {
			return nil, nil, false, ErrMasterDataFieldNotEditable
		}
		return trackBDepartureState(student, value)
	default:
		return nil, nil, false, ErrMasterDataFieldNotEditable
	}
}

func trackBStudentFieldState(field string, student *usersModels.Student, value json.RawMessage) (json.RawMessage, json.RawMessage, bool, error) {
	if field != "school_class" {
		return nil, nil, false, ErrMasterDataFieldNotEditable
	}
	newClass, err := decodeStringValue(value)
	if err != nil || newClass == "" {
		return nil, nil, false, ErrMasterDataInvalidValue
	}
	return jsonString(student.SchoolClass), jsonString(newClass), newClass != student.SchoolClass, nil
}

func trackBPersonFieldState(field string, person *usersModels.Person, value json.RawMessage) (json.RawMessage, json.RawMessage, bool, error) {
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
		if _, parseErr := timezone.ParseDate(newStr); parseErr != nil {
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

func birthdayPtr(person *usersModels.Person) *string {
	if person.Birthday == nil {
		return nil
	}
	s := person.Birthday.String()
	return &s
}

func trackBDepartureState(student *usersModels.Student, value json.RawMessage) (json.RawMessage, json.RawMessage, bool, error) {
	var modes usersModels.AllowedDepartureModes
	if err := json.Unmarshal(value, &modes); err != nil {
		return nil, nil, false, ErrMasterDataInvalidValue
	}
	if err := modes.Validate(); err != nil {
		return nil, nil, false, ErrMasterDataInvalidValue
	}
	if modes.HasMode(usersModels.DepartureAccompanied) ||
		student.AllowedDepartureModes.HasMode(usersModels.DepartureAccompanied) {
		return nil, nil, false, ErrMasterDataInvalidValue
	}
	normalized := modes.Normalize()
	current := student.AllowedDepartureModes.Normalize()
	newRaw, _ := json.Marshal(normalized)
	oldRaw, _ := json.Marshal(current)
	return oldRaw, newRaw, !bytes.Equal(newRaw, oldRaw), nil
}

// isPendingChangeRequestUniqueViolation reports Care Plan's refusal of a
// second pending request for the same field (the partial unique index on
// users.student_data_change_requests).
func isPendingChangeRequestUniqueViolation(err error) bool {
	return errors.Is(err, careplan.ErrStudentDataRequestFieldPending)
}
