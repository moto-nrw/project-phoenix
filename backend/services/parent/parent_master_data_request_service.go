package parent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	"github.com/moto-nrw/project-phoenix/tenant"
)

const pendingChangeRequestUniqueIndex = "uniq_student_data_change_requests_pending_field"

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
	usersModels.DataChangeTargetDeparture + "/allowed_departure_modes": true,
}

// SubmitMasterDataChangeRequest records one or more pending Track B change
// requests for staff approval. Fields whose proposed value equals the current
// value are skipped; a field with an already-pending request is rejected.
func (s *service) SubmitMasterDataChangeRequest(ctx context.Context, accountID, studentID int64, changes []MasterDataFieldChange) ([]*usersModels.StudentDataChangeRequest, error) {
	if len(changes) == 0 {
		return nil, ErrMasterDataNoChanges
	}
	for _, c := range changes {
		if !trackBRequestableFields[c.Target+"/"+c.FieldKey] {
			return nil, ErrMasterDataFieldNotEditable
		}
	}

	child, err := s.resolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionMasterDataRequest)
	if err != nil {
		return nil, err
	}

	enabled, err := s.Settings.ResolveBoolForTenant(ctx, child.tenantID, configModels.KeyParentMasterDataRequestEnabled)
	if err != nil {
		return nil, fmt.Errorf("parent: resolve master-data request setting: %w", err)
	}
	if !enabled {
		return nil, ErrMasterDataRequestDisabled
	}

	var created []*usersModels.StudentDataChangeRequest
	txErr := tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		student, loadErr := s.StudentRepo.FindByID(txCtx, studentID)
		if loadErr != nil {
			return loadErr
		}
		person, loadErr := s.PersonRepo.FindByID(txCtx, student.PersonID)
		if loadErr != nil {
			return loadErr
		}

		for _, c := range changes {
			oldRaw, newRaw, changed, fieldErr := trackBFieldState(c.Target, c.FieldKey, person, student, c.Value)
			if fieldErr != nil {
				return fieldErr
			}
			if !changed {
				continue
			}
			has, hasErr := s.ChangeRequestRepo.HasPendingForField(txCtx, studentID, c.Target, c.FieldKey)
			if hasErr != nil {
				return hasErr
			}
			if has {
				return ErrMasterDataDuplicatePending
			}
			row := &usersModels.StudentDataChangeRequest{
				StudentID:   studentID,
				SubmittedBy: accountID,
				Target:      c.Target,
				FieldKey:    c.FieldKey,
				OldValue:    oldRaw,
				NewValue:    newRaw,
				Status:      usersModels.DataChangeStatusPending,
			}
			row.SetTenantID(child.tenantID)
			if createErr := s.ChangeRequestRepo.Create(txCtx, row); createErr != nil {
				if isPendingChangeRequestUniqueViolation(createErr) {
					return ErrMasterDataDuplicatePending
				}
				return createErr
			}
			created = append(created, row)
		}
		if len(created) > 0 {
			// One "Anfrage erstellt" pill PER created row, each referencing its
			// OWN row. A multi-field submit is decided field-by-field
			// (master_data_review.Decide runs per row and emits a decision pill
			// keyed on THAT row's id), so per-row created pills keep every
			// created↔decision ref paired: the thread timeline and the deep-link
			// into the Änderungsanfragen queue resolve the exact row for any
			// field, not just created[0]. Best-effort, after commit.
			capturedTenant := child.tenantID
			refIDs := make([]int64, len(created))
			for i, row := range created {
				refIDs[i] = row.ID
			}
			tenant.RegisterAfterCommit(txCtx, func() {
				if s.Emitter == nil {
					return
				}
				for i := range refIDs {
					refID := refIDs[i]
					s.Emitter.EmitChildEvent(capturedTenant, studentID, accountID, parentmessaging.ChildEvent{
						EventType:      "request_created",
						ActorKind:      usersModels.ParentMessageSenderGuardian,
						ActorAccountID: accountID,
						Body:           "Anfrage: Stammdaten ändern",
						RequestType:    usersModels.ParentMessageRequestMasterData,
						RequestStatus:  usersModels.ParentMessageRequestStatusOpen,
						RefTable:       "users.student_data_change_requests",
						RefID:          &refID,
					})
				}
			})
		}
		return nil
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
		slog.Int64("tenant_id", child.tenantID),
		slog.Int("fields", len(created)),
	)
	return created, nil
}

// ListMyMasterDataRequests returns child-level Track B change requests,
// newest-first. It deliberately excludes Track A guardian contact audit rows so
// one linked parent cannot read another guardian's private email/phone/address
// old/new values.
func (s *service) ListMyMasterDataRequests(ctx context.Context, accountID, studentID int64) ([]*usersModels.StudentDataChangeRequest, error) {
	child, err := s.resolveOwnedChild(ctx, accountID, studentID)
	if err != nil {
		return nil, err
	}
	var out []*usersModels.StudentDataChangeRequest
	txErr := tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		rows, listErr := s.ChangeRequestRepo.ListParentVisibleByStudent(txCtx, studentID, 0)
		if listErr != nil {
			return listErr
		}
		out = rows
		return nil
	})
	if txErr != nil {
		return nil, fmt.Errorf("parent: list master data requests: %w", txErr)
	}
	return out, nil
}

// trackBFieldState validates the proposed value, computes the current value as
// JSON for the audit row, and reports whether the value actually changes.
func trackBFieldState(target, field string, person *usersModels.Person, student *usersModels.Student, value json.RawMessage) (oldRaw, newRaw json.RawMessage, changed bool, err error) {
	switch target {
	case usersModels.DataChangeTargetPerson:
		return trackBPersonFieldState(field, person, value)
	case usersModels.DataChangeTargetDeparture:
		if field != "allowed_departure_modes" {
			return nil, nil, false, ErrMasterDataFieldNotEditable
		}
		return trackBDepartureState(student, value)
	default:
		return nil, nil, false, ErrMasterDataFieldNotEditable
	}
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

func isPendingChangeRequestUniqueViolation(err error) bool {
	return modelBase.IsUniqueViolationOn(err, pendingChangeRequestUniqueIndex)
}
