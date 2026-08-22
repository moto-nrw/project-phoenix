package parent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/localization"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Stammdaten sentinel errors mapped to stable HTTP codes by the handler.
var (
	// ErrMasterDataEditDisabled means operations.parent_master_data_edit_enabled
	// is off for the child's tenant.
	ErrMasterDataEditDisabled = errors.New("parent: Stammdaten editing disabled for this school")
	// ErrMasterDataFieldNotEditable means the (target, field) pair is not in the
	// Track A direct-edit allowlist (identity and departure go through Track B).
	ErrMasterDataFieldNotEditable = errors.New("parent: this field cannot be edited directly")
	// ErrMasterDataInvalidValue means the submitted value failed validation
	// (bad email, unknown contact method/locale, malformed phone, …).
	ErrMasterDataInvalidValue = errors.New("parent: invalid value for this field")
	// ErrMasterDataRequestDisabled means
	// operations.parent_master_data_request_enabled is off for the tenant.
	// (Track B — see parent_master_data_request_service.go.)
	ErrMasterDataRequestDisabled = errors.New("parent: Stammdaten change requests disabled for this school")
	// ErrMasterDataDuplicatePending means an undecided request already exists
	// for the same field. (Track B.)
	ErrMasterDataDuplicatePending = errors.New("parent: a change request for this field is already pending")
	// ErrMasterDataNoChanges means a Track B submission carried no actual
	// changes (every field equalled its current value).
	ErrMasterDataNoChanges = errors.New("parent: no changes to submit")
)

// trackAEditableFields is the allowlist of direct-edit (Track A) fields keyed by
// "target/field". Identity (name, birthday) and permanent departure data are
// intentionally absent — those require staff approval via Track B.
var trackAEditableFields = map[string]bool{
	usersModels.DataChangeTargetStudent + "/health_info":                      true,
	usersModels.DataChangeTargetGuardianProfile + "/email":                    true,
	usersModels.DataChangeTargetGuardianProfile + "/address_street":           true,
	usersModels.DataChangeTargetGuardianProfile + "/address_city":             true,
	usersModels.DataChangeTargetGuardianProfile + "/address_postal_code":      true,
	usersModels.DataChangeTargetGuardianProfile + "/preferred_contact_method": true,
	usersModels.DataChangeTargetGuardianProfile + "/language_preference":      true,
	usersModels.DataChangeTargetGuardianPhone + "/primary":                    true,
}

const maxMasterDataHealthInfoLen = 2000

// ChildMasterData is the structured Stammdaten view a parent sees for one child.
// It blends the child's own person/student record with the calling guardian's
// contact data (the data they themselves supplied), and lists any pending Track
// B change requests so the UI can show "awaiting approval" badges. Betreuernotizen
// are deliberately excluded — they are staff-internal.
type ChildMasterData struct {
	StudentID int64

	// Identity — Track B editable, shown here read-only.
	FirstName string
	LastName  string
	Birthday  *timezone.Date

	// School-controlled — read-only.
	SchoolClass   string
	Status        string
	EnrolledFrom  *timezone.Date
	EnrolledUntil *timezone.Date

	// Health — Track A editable.
	HealthInfo *string

	// Calling guardian's own contact data — Track A editable.
	GuardianProfileID      int64
	Email                  *string
	AddressStreet          *string
	AddressCity            *string
	AddressPostalCode      *string
	PreferredContactMethod string
	LanguagePreference     string
	PrimaryPhone           *string
	PrimaryPhoneID         *int64

	// Permanent weekday departure config — Track B editable, shown read-only.
	DepartureDays         usersModels.DepartureDays
	AllowedDepartureModes usersModels.AllowedDepartureModes

	// PendingChanges are the child's open Track B requests (awaiting staff).
	PendingChanges []*usersModels.StudentDataChangeRequest
}

// GetChildMasterData assembles the Stammdaten view after verifying the account
// is a guardian of the child.
func (s *service) GetChildMasterData(ctx context.Context, accountID, studentID int64) (*ChildMasterData, error) {
	child, err := s.resolveOwnedChild(ctx, accountID, studentID)
	if err != nil {
		return nil, err
	}

	var out *ChildMasterData
	txErr := tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		data, loadErr := s.loadMasterData(txCtx, child.guardianProfileID, studentID)
		if loadErr != nil {
			return loadErr
		}
		out = data
		return nil
	})
	if txErr != nil {
		return nil, fmt.Errorf("parent: get master data: %w", txErr)
	}
	return out, nil
}

// UpdateMasterDataField applies a single Track A direct edit and returns the
// refreshed view. The live record is updated and an auto_applied audit row is
// written in the same transaction.
func (s *service) UpdateMasterDataField(ctx context.Context, accountID, studentID int64, target, fieldKey string, value json.RawMessage) (*ChildMasterData, error) {
	if !trackAEditableFields[target+"/"+fieldKey] {
		return nil, ErrMasterDataFieldNotEditable
	}

	child, err := s.resolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionMasterDataEdit)
	if err != nil {
		return nil, err
	}
	// A child whose care at this school has ended keeps read access to what
	// happened, but nothing new can be submitted for them (#2487).
	if err := child.requireCareRunning(); err != nil {
		return nil, err
	}

	enabled, err := s.Settings.ResolveBoolForTenant(ctx, child.tenantID, configModels.KeyParentMasterDataEditEnabled)
	if err != nil {
		return nil, fmt.Errorf("parent: resolve master-data edit setting: %w", err)
	}
	if !enabled {
		return nil, ErrMasterDataEditDisabled
	}
	if isTrackAGuardianTarget(target) {
		if err := s.requireGuardianManagementEnabled(ctx, child.tenantID); err != nil {
			return nil, err
		}
	}

	newStr, err := decodeStringValue(value)
	if err != nil {
		return nil, err
	}
	if err := validateTrackAFieldSize(target, fieldKey, newStr); err != nil {
		return nil, err
	}

	var out *ChildMasterData
	txErr := tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		if err := s.requireCareRunningForUpdate(txCtx, studentID); err != nil {
			return err
		}
		oldRaw, newRaw, targetRef, changed, applyErr := s.applyTrackAEdit(txCtx, child.guardianProfileID, studentID, child.tenantID, accountID, target, fieldKey, newStr)
		if applyErr != nil {
			return applyErr
		}
		if changed {
			if recErr := s.recordAutoApplied(txCtx, child.tenantID, studentID, accountID, target, fieldKey, oldRaw, newRaw, targetRef); recErr != nil {
				return recErr
			}
		}
		data, loadErr := s.loadMasterData(txCtx, child.guardianProfileID, studentID)
		if loadErr != nil {
			return loadErr
		}
		out = data

		capturedTenant := child.tenantID
		if changed {
			tenant.RegisterAfterCommit(txCtx, func() {
				s.broadcastStudentUpdated(capturedTenant, studentID)
			})
		}
		return nil
	})
	if txErr != nil {
		return nil, fmt.Errorf("parent: update master data: %w", txErr)
	}

	s.Logger.Info("parent edited master data field",
		slog.Int64("account_id", accountID),
		slog.Int64("student_id", studentID),
		slog.Int64("tenant_id", child.tenantID),
		slog.String("target", target),
		slog.String("field", fieldKey),
	)
	return out, nil
}

// applyTrackAEdit writes the single field to its live record and returns the old
// and new values as JSON (for the audit row) plus the optional target ref id.
func (s *service) applyTrackAEdit(ctx context.Context, guardianProfileID, studentID, tenantID, accountID int64, target, fieldKey, newStr string) (oldRaw, newRaw json.RawMessage, targetRef *int64, changed bool, err error) {
	switch target {
	case usersModels.DataChangeTargetStudent:
		return s.applyStudentEdit(ctx, studentID, accountID, fieldKey, newStr)
	case usersModels.DataChangeTargetGuardianProfile:
		return s.applyGuardianProfileEdit(ctx, guardianProfileID, fieldKey, newStr)
	case usersModels.DataChangeTargetGuardianPhone:
		return s.applyGuardianPhoneEdit(ctx, guardianProfileID, tenantID, newStr)
	default:
		return nil, nil, nil, false, ErrMasterDataFieldNotEditable
	}
}

func isTrackAGuardianTarget(target string) bool {
	return target == usersModels.DataChangeTargetGuardianProfile || target == usersModels.DataChangeTargetGuardianPhone
}

func validateTrackAFieldSize(target, fieldKey, value string) error {
	limit := 0
	switch target + "/" + fieldKey {
	case usersModels.DataChangeTargetStudent + "/health_info":
		limit = maxMasterDataHealthInfoLen
	case usersModels.DataChangeTargetGuardianProfile + "/email":
		limit = maxGuardianEmailLen
	case usersModels.DataChangeTargetGuardianProfile + "/address_street",
		usersModels.DataChangeTargetGuardianProfile + "/address_city",
		usersModels.DataChangeTargetGuardianProfile + "/address_postal_code":
		limit = maxGuardianAddrLen
	case usersModels.DataChangeTargetGuardianProfile + "/preferred_contact_method",
		usersModels.DataChangeTargetGuardianProfile + "/language_preference":
		limit = maxGuardianLabelLen
	case usersModels.DataChangeTargetGuardianPhone + "/primary":
		limit = maxGuardianPhoneLen
	default:
		return ErrMasterDataFieldNotEditable
	}
	if utf8.RuneCountInString(value) > limit {
		return fmt.Errorf("%w: field exceeds %d characters", ErrMasterDataInvalidValue, limit)
	}
	return nil
}

func (s *service) applyStudentEdit(ctx context.Context, studentID, accountID int64, fieldKey, newStr string) (json.RawMessage, json.RawMessage, *int64, bool, error) {
	student, err := s.StudentRepo.FindByIDForUpdate(ctx, studentID)
	if err != nil {
		return nil, nil, nil, false, err
	}
	before := *student
	switch fieldKey {
	case "health_info":
		oldRaw := jsonStringPtr(student.HealthInfo)
		next := strPtrOrNil(newStr)
		newRaw := jsonStringPtr(next)
		if string(oldRaw) == string(newRaw) {
			return oldRaw, newRaw, nil, false, nil
		}
		student.HealthInfo = next
		if err := s.StudentRepo.Update(ctx, student); err != nil {
			return nil, nil, nil, false, err
		}
		if s.StudentAudit != nil {
			if err := s.StudentAudit.RecordChangesForActor(ctx, &before, student, accountID); err != nil {
				return nil, nil, nil, false, err
			}
		}
		return oldRaw, newRaw, nil, true, nil
	default:
		return nil, nil, nil, false, ErrMasterDataFieldNotEditable
	}
}

func (s *service) applyGuardianProfileEdit(ctx context.Context, guardianProfileID int64, fieldKey, newStr string) (json.RawMessage, json.RawMessage, *int64, bool, error) {
	if err := s.GuardianProfileRepo.LockByIDForUpdate(ctx, guardianProfileID); err != nil {
		return nil, nil, nil, false, err
	}
	profile, err := s.GuardianProfileRepo.FindByID(ctx, guardianProfileID)
	if err != nil {
		return nil, nil, nil, false, err
	}
	var oldRaw json.RawMessage
	switch fieldKey {
	case "email":
		oldRaw = jsonStringPtr(profile.Email)
		next := strPtrOrNil(newStr)
		if next != nil {
			addr, parseErr := mail.ParseAddress(*next)
			if parseErr != nil {
				return nil, nil, nil, false, ErrMasterDataInvalidValue
			}
			normalized := addr.Address
			next = &normalized
		}
		profile.Email = next
	case "address_street":
		oldRaw = jsonStringPtr(profile.AddressStreet)
		profile.AddressStreet = strPtrOrNil(newStr)
	case "address_city":
		oldRaw = jsonStringPtr(profile.AddressCity)
		profile.AddressCity = strPtrOrNil(newStr)
	case "address_postal_code":
		oldRaw = jsonStringPtr(profile.AddressPostalCode)
		profile.AddressPostalCode = strPtrOrNil(newStr)
	case "preferred_contact_method":
		if newStr == "" {
			return nil, nil, nil, false, ErrMasterDataInvalidValue
		}
		oldRaw = jsonString(profile.PreferredContactMethod)
		profile.PreferredContactMethod = newStr
	case "language_preference":
		if !localization.IsSupported(newStr) {
			return nil, nil, nil, false, ErrMasterDataInvalidValue
		}
		oldRaw = jsonString(profile.LanguagePreference)
		profile.LanguagePreference = localization.NormalizeLocale(newStr)
	default:
		return nil, nil, nil, false, ErrMasterDataFieldNotEditable
	}

	// Validate() trims/lowercases the email and rejects bad emails / contact
	// methods — turn its failure into the stable invalid-value sentinel.
	if err := profile.Validate(); err != nil {
		return nil, nil, nil, false, fmt.Errorf("%w: %s", ErrMasterDataInvalidValue, err.Error())
	}
	newRaw := s.guardianProfileFieldJSON(profile, fieldKey)
	if string(oldRaw) == string(newRaw) {
		ref := profile.ID
		return oldRaw, newRaw, &ref, false, nil
	}
	if err := s.GuardianProfileRepo.Update(ctx, profile); err != nil {
		if isGuardianEmailUniqueViolation(err) && profile.Email != nil {
			return nil, nil, nil, false, ErrGuardianEmailConflict
		}
		return nil, nil, nil, false, err
	}
	ref := profile.ID
	return oldRaw, newRaw, &ref, true, nil
}

// guardianProfileFieldJSON returns the post-update value of the named field as
// JSON for the audit row.
func (s *service) guardianProfileFieldJSON(profile *usersModels.GuardianProfile, fieldKey string) json.RawMessage {
	switch fieldKey {
	case "email":
		return jsonStringPtr(profile.Email)
	case "address_street":
		return jsonStringPtr(profile.AddressStreet)
	case "address_city":
		return jsonStringPtr(profile.AddressCity)
	case "address_postal_code":
		return jsonStringPtr(profile.AddressPostalCode)
	case "preferred_contact_method":
		return jsonString(profile.PreferredContactMethod)
	case "language_preference":
		return jsonString(profile.LanguagePreference)
	default:
		return json.RawMessage("null")
	}
}

// applyGuardianPhoneEdit upserts the calling guardian's primary phone number. An
// empty value clears it. Multi-phone management is staff-side; the portal edits
// only the primary number.
func (s *service) applyGuardianPhoneEdit(ctx context.Context, guardianProfileID, tenantID int64, newStr string) (json.RawMessage, json.RawMessage, *int64, bool, error) {
	if err := s.GuardianProfileRepo.LockByIDForUpdate(ctx, guardianProfileID); err != nil {
		return nil, nil, nil, false, err
	}
	phones, err := s.GuardianPhoneRepo.FindByGuardianID(ctx, guardianProfileID)
	if err != nil {
		return nil, nil, nil, false, err
	}
	existing := pickPrimaryPhone(phones)

	oldRaw := json.RawMessage("null")
	if existing != nil {
		oldRaw = jsonString(existing.PhoneNumber)
	}

	// Clear: empty value removes the existing primary phone.
	if newStr == "" {
		if existing != nil {
			ref := existing.ID
			if err := s.GuardianPhoneRepo.Delete(ctx, existing.ID); err != nil {
				return nil, nil, nil, false, err
			}
			return oldRaw, json.RawMessage("null"), &ref, true, nil
		}
		return oldRaw, json.RawMessage("null"), nil, false, nil
	}

	if existing != nil {
		existing.PhoneNumber = newStr
		if err := existing.Validate(); err != nil {
			return nil, nil, nil, false, fmt.Errorf("%w: %s", ErrMasterDataInvalidValue, err.Error())
		}
		newRaw := jsonString(existing.PhoneNumber)
		if string(oldRaw) == string(newRaw) {
			ref := existing.ID
			return oldRaw, newRaw, &ref, false, nil
		}
		if err := s.GuardianPhoneRepo.Update(ctx, existing); err != nil {
			return nil, nil, nil, false, err
		}
		ref := existing.ID
		return oldRaw, newRaw, &ref, true, nil
	}

	phone := &usersModels.GuardianPhoneNumber{
		GuardianProfileID: guardianProfileID,
		PhoneNumber:       newStr,
		PhoneType:         usersModels.PhoneTypeMobile,
		IsPrimary:         true,
		Priority:          1,
	}
	phone.SetTenantID(tenantID)
	if err := phone.Validate(); err != nil {
		return nil, nil, nil, false, fmt.Errorf("%w: %s", ErrMasterDataInvalidValue, err.Error())
	}
	if err := s.GuardianPhoneRepo.Create(ctx, phone); err != nil {
		return nil, nil, nil, false, err
	}
	ref := phone.ID
	return oldRaw, jsonString(phone.PhoneNumber), &ref, true, nil
}

// recordAutoApplied inserts the Track A audit row for a direct edit.
func (s *service) recordAutoApplied(ctx context.Context, tenantID, studentID, accountID int64, target, fieldKey string, oldRaw, newRaw json.RawMessage, targetRef *int64) error {
	now := time.Now()
	row := &usersModels.StudentDataChangeRequest{
		StudentID:   studentID,
		SubmittedBy: accountID,
		Target:      target,
		TargetRefID: targetRef,
		FieldKey:    fieldKey,
		OldValue:    oldRaw,
		NewValue:    newRaw,
		Status:      usersModels.DataChangeStatusAutoApplied,
		AppliedAt:   &now,
	}
	row.SetTenantID(tenantID)
	return s.ChangeRequestRepo.Create(ctx, row)
}

// loadMasterData assembles the Stammdaten DTO inside an existing tenant tx.
func (s *service) loadMasterData(ctx context.Context, guardianProfileID, studentID int64) (*ChildMasterData, error) {
	student, err := s.StudentRepo.FindByID(ctx, studentID)
	if err != nil {
		return nil, err
	}
	person, err := s.PersonRepo.FindByID(ctx, student.PersonID)
	if err != nil {
		return nil, err
	}
	profile, err := s.GuardianProfileRepo.FindByID(ctx, guardianProfileID)
	if err != nil {
		return nil, err
	}

	out := &ChildMasterData{
		StudentID:              studentID,
		FirstName:              person.FirstName,
		LastName:               person.LastName,
		Birthday:               person.Birthday,
		SchoolClass:            student.SchoolClass,
		Status:                 string(student.Status),
		EnrolledFrom:           student.EnrolledFrom,
		EnrolledUntil:          student.EnrolledUntil,
		HealthInfo:             student.HealthInfo,
		GuardianProfileID:      profile.ID,
		Email:                  profile.Email,
		AddressStreet:          profile.AddressStreet,
		AddressCity:            profile.AddressCity,
		AddressPostalCode:      profile.AddressPostalCode,
		PreferredContactMethod: profile.PreferredContactMethod,
		LanguagePreference:     profile.LanguagePreference,
		DepartureDays:          student.DepartureDays,
		AllowedDepartureModes:  student.AllowedDepartureModes,
	}

	phones, err := s.GuardianPhoneRepo.FindByGuardianID(ctx, profile.ID)
	if err != nil {
		return nil, err
	}
	if primary := pickPrimaryPhone(phones); primary != nil {
		out.PrimaryPhone = &primary.PhoneNumber
		out.PrimaryPhoneID = &primary.ID
	}

	pending, err := s.ChangeRequestRepo.ListByStudent(ctx, studentID, []string{usersModels.DataChangeStatusPending}, 0)
	if err != nil {
		return nil, err
	}
	out.PendingChanges = pending

	return out, nil
}

// pickPrimaryPhone returns the guardian's primary phone, or the first one when
// none is flagged primary, or nil when the guardian has no phone numbers.
// FindByGuardianID already orders is_primary DESC, so the first row is the best
// candidate — but we still prefer an explicit primary defensively.
func pickPrimaryPhone(phones []*usersModels.GuardianPhoneNumber) *usersModels.GuardianPhoneNumber {
	for _, p := range phones {
		if p.IsPrimary {
			return p
		}
	}
	if len(phones) > 0 {
		return phones[0]
	}
	return nil
}

// --- small value helpers ---

func decodeStringValue(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", ErrMasterDataInvalidValue
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", ErrMasterDataInvalidValue
	}
	return strings.TrimSpace(s), nil
}

func strPtrOrNil(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}

func jsonString(s string) json.RawMessage {
	b, _ := json.Marshal(s)
	return b
}

func jsonStringPtr(s *string) json.RawMessage {
	if s == nil {
		return json.RawMessage("null")
	}
	return jsonString(*s)
}
