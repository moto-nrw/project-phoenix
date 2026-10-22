package care

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

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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
	// ErrMasterDataRequestNotFound means the Track B request does not exist or
	// is not the caller's own. A stranger must not learn that the id exists.
	ErrMasterDataRequestNotFound = errors.New("parent: Stammdaten change request not found")
	// ErrMasterDataRequestNotPending means the Track B request was decided
	// meanwhile and can no longer be edited.
	ErrMasterDataRequestNotPending = errors.New("parent: Stammdaten change request is not pending")
)

// trackAEditableFields is the allowlist of direct-edit (Track A) fields keyed by
// "target/field". Identity (name, birthday) and permanent departure data are
// intentionally absent — those require staff approval via Track B.
var trackAEditableFields = map[string]bool{
	DataChangeTargetStudent + "/health_info":                      true,
	DataChangeTargetGuardianProfile + "/email":                    true,
	DataChangeTargetGuardianProfile + "/address_street":           true,
	DataChangeTargetGuardianProfile + "/address_city":             true,
	DataChangeTargetGuardianProfile + "/address_postal_code":      true,
	DataChangeTargetGuardianProfile + "/preferred_contact_method": true,
	DataChangeTargetGuardianProfile + "/language_preference":      true,
	DataChangeTargetGuardianPhone + "/primary":                    true,
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
	Birthday  *calendar.Date

	// School-controlled — read-only.
	SchoolClass   string
	Status        string
	EnrolledFrom  *calendar.Date
	EnrolledUntil *calendar.Date

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
	DepartureDays         DepartureDays
	AllowedDepartureModes AllowedDepartureModes

	// PendingChanges are the child's open Track B requests (awaiting staff).
	PendingChanges []*StudentDataChangeRequest
}

// GetChildMasterData assembles the Stammdaten view after verifying the account
// is a guardian of the child.
func (s *Service) GetChildMasterData(ctx context.Context, accountID, studentID int64) (*ChildMasterData, error) {
	child, err := s.ResolveOwnedChild(ctx, accountID, studentID)
	if err != nil {
		return nil, err
	}

	var out *ChildMasterData
	txErr := s.Runtime.WithinTenant(ctx, child.TenantID, func(txCtx context.Context) error {
		data, loadErr := s.loadMasterData(txCtx, accountID, child.GuardianProfileID, studentID)
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
func (s *Service) UpdateMasterDataField(ctx context.Context, accountID, studentID int64, target, fieldKey string, value json.RawMessage) (*ChildMasterData, error) {
	if !trackAEditableFields[target+"/"+fieldKey] {
		return nil, ErrMasterDataFieldNotEditable
	}

	child, err := s.ResolvePermittedChild(ctx, accountID, studentID, GuardianPermissionMasterDataEdit)
	if err != nil {
		return nil, err
	}
	// A child whose care at this school has ended keeps read access to
	// what happened, but nothing new can be submitted for them (#2487).
	if err := child.RequireCareRunning(); err != nil {
		return nil, err
	}
	if err := s.requireTrackAEditEnabled(ctx, child.TenantID, target); err != nil {
		return nil, err
	}

	newStr, err := decodeStringValue(value)
	if err != nil {
		return nil, err
	}
	if err := validateTrackAFieldSize(target, fieldKey, newStr); err != nil {
		return nil, err
	}

	edit := trackAEdit{accountID: accountID, studentID: studentID, target: target, fieldKey: fieldKey, value: newStr}
	var out *ChildMasterData
	txErr := s.Runtime.WithinTenant(ctx, child.TenantID, func(txCtx context.Context) error {
		data, applyErr := s.applyTrackAInTx(txCtx, child, edit)
		out = data
		return applyErr
	})
	if txErr != nil {
		return nil, fmt.Errorf("parent: update master data: %w", txErr)
	}

	s.Logger.Info("parent edited master data field",
		slog.Int64("account_id", accountID),
		slog.Int64("student_id", studentID),
		slog.Int64("tenant_id", child.TenantID),
		slog.String("target", target),
		slog.String("field", fieldKey),
	)
	return out, nil
}

// requireTrackAEditEnabled checks the school's direct-edit switch, and the
// guardian-management switch for the guardian's own contact fields.
func (s *Service) requireTrackAEditEnabled(ctx context.Context, tenantID int64, target string) error {
	enabled, err := s.Settings.ResolveBoolForTenant(ctx, tenantID, KeyParentMasterDataEditEnabled)
	if err != nil {
		return fmt.Errorf("parent: resolve master-data edit setting: %w", err)
	}
	if !enabled {
		return ErrMasterDataEditDisabled
	}
	if isTrackAGuardianTarget(target) {
		return s.requireGuardianManagementEnabled(ctx, tenantID)
	}
	return nil
}

// trackAEdit is one direct Stammdaten edit.
type trackAEdit struct {
	accountID int64
	studentID int64
	target    string
	fieldKey  string
	value     string
}

// applyTrackAInTx writes the field through its owner, records the
// auto-applied audit row and reads the refreshed view, in one unit of work.
func (s *Service) applyTrackAInTx(ctx context.Context, child *Child, edit trackAEdit) (*ChildMasterData, error) {
	if err := s.RequireCareRunningForUpdate(ctx, edit.studentID); err != nil {
		return nil, err
	}
	oldRaw, newRaw, targetRef, changed, err := s.applyTrackAEdit(ctx, child.GuardianProfileID, edit.studentID, child.TenantID, edit.accountID, edit.target, edit.fieldKey, edit.value)
	if err != nil {
		return nil, err
	}
	if changed {
		if err := s.recordAutoApplied(ctx, child.TenantID, edit.studentID, edit.accountID, edit.target, edit.fieldKey, oldRaw, newRaw, targetRef); err != nil {
			return nil, err
		}
	}
	data, err := s.loadMasterData(ctx, edit.accountID, child.GuardianProfileID, edit.studentID)
	if err != nil {
		return nil, err
	}
	capturedTenant := child.TenantID
	if changed {
		s.Runtime.RegisterAfterCommit(ctx, func() {
			s.broadcastStudentUpdated(capturedTenant, edit.studentID)
		})
	}
	return data, nil
}

// applyTrackAEdit writes the single field to its live record and returns the old
// and new values as JSON (for the audit row) plus the optional target ref id.
func (s *Service) applyTrackAEdit(ctx context.Context, guardianProfileID, studentID, tenantID, accountID int64, target, fieldKey, newStr string) (oldRaw, newRaw json.RawMessage, targetRef *int64, changed bool, err error) {
	switch target {
	case DataChangeTargetStudent:
		return s.applyStudentEdit(ctx, studentID, accountID, fieldKey, newStr)
	case DataChangeTargetGuardianProfile:
		return s.applyGuardianProfileEdit(ctx, guardianProfileID, fieldKey, newStr)
	case DataChangeTargetGuardianPhone:
		return s.applyGuardianPhoneEdit(ctx, guardianProfileID, tenantID, newStr)
	default:
		return nil, nil, nil, false, ErrMasterDataFieldNotEditable
	}
}

func isTrackAGuardianTarget(target string) bool {
	return target == DataChangeTargetGuardianProfile || target == DataChangeTargetGuardianPhone
}

func validateTrackAFieldSize(target, fieldKey, value string) error {
	limit := 0
	switch target + "/" + fieldKey {
	case DataChangeTargetStudent + "/health_info":
		limit = maxMasterDataHealthInfoLen
	case DataChangeTargetGuardianProfile + "/email":
		limit = maxGuardianEmailLen
	case DataChangeTargetGuardianProfile + "/address_street",
		DataChangeTargetGuardianProfile + "/address_city",
		DataChangeTargetGuardianProfile + "/address_postal_code":
		limit = maxGuardianAddrLen
	case DataChangeTargetGuardianProfile + "/preferred_contact_method",
		DataChangeTargetGuardianProfile + "/language_preference":
		limit = maxGuardianLabelLen
	case DataChangeTargetGuardianPhone + "/primary":
		limit = maxGuardianPhoneLen
	default:
		return ErrMasterDataFieldNotEditable
	}
	if utf8.RuneCountInString(value) > limit {
		return fmt.Errorf("%w: field exceeds %d characters", ErrMasterDataInvalidValue, limit)
	}
	return nil
}

func (s *Service) applyStudentEdit(ctx context.Context, studentID, accountID int64, fieldKey, newStr string) (json.RawMessage, json.RawMessage, *int64, bool, error) {
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
		if err := s.Students.SetStudentHealthInfo(ctx, student.ID, student.HealthInfo); err != nil {
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

func (s *Service) applyGuardianProfileEdit(ctx context.Context, guardianProfileID int64, fieldKey, newStr string) (json.RawMessage, json.RawMessage, *int64, bool, error) {
	if err := s.GuardianProfileRepo.LockByIDForUpdate(ctx, guardianProfileID); err != nil {
		return nil, nil, nil, false, err
	}
	profile, err := s.GuardianProfileRepo.FindByID(ctx, guardianProfileID)
	if err != nil {
		return nil, nil, nil, false, err
	}
	oldRaw, err := s.setGuardianProfileField(profile, fieldKey, newStr)
	if err != nil {
		return nil, nil, nil, false, err
	}

	// Validate() trims/lowercases the email and rejects bad emails / contact
	// methods — turn its failure into the stable invalid-value sentinel.
	if err := s.GuardianProfileRepo.NormalizeProfile(profile); err != nil {
		return nil, nil, nil, false, fmt.Errorf("%w: %s", ErrMasterDataInvalidValue, err.Error())
	}
	newRaw := s.guardianProfileFieldJSON(guardianContactRecord(profile), fieldKey)
	ref := profile.ID
	if string(oldRaw) == string(newRaw) {
		return oldRaw, newRaw, &ref, false, nil
	}
	if err := s.updateGuardianProfile(ctx, profile); err != nil {
		if errors.Is(err, ErrGuardianEmailConflict) && profile.Email != nil {
			return nil, nil, nil, false, ErrGuardianEmailConflict
		}
		return nil, nil, nil, false, err
	}
	return oldRaw, newRaw, &ref, true, nil
}

// setGuardianProfileField applies one Track A guardian field to the profile
// and returns its previous value as JSON.
func (s *Service) setGuardianProfileField(profile *GuardianProfile, fieldKey, newStr string) (json.RawMessage, error) {
	var oldRaw json.RawMessage
	switch fieldKey {
	case "email":
		oldRaw = jsonStringPtr(profile.Email)
		next := strPtrOrNil(newStr)
		if next != nil {
			addr, parseErr := mail.ParseAddress(*next)
			if parseErr != nil {
				return nil, ErrMasterDataInvalidValue
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
			return nil, ErrMasterDataInvalidValue
		}
		oldRaw = jsonString(profile.PreferredContactMethod)
		profile.PreferredContactMethod = newStr
	case "language_preference":
		if !s.Locales.IsSupported(newStr) {
			return nil, ErrMasterDataInvalidValue
		}
		oldRaw = jsonString(profile.LanguagePreference)
		profile.LanguagePreference = s.Locales.Normalize(newStr)
	default:
		return nil, ErrMasterDataFieldNotEditable
	}
	return oldRaw, nil
}

// guardianProfileFieldJSON returns the post-update value of the named field as
// JSON for the audit row.
func (s *Service) guardianProfileFieldJSON(profile GuardianContactRecord, fieldKey string) json.RawMessage {
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
func (s *Service) applyGuardianPhoneEdit(ctx context.Context, guardianProfileID, tenantID int64, newStr string) (json.RawMessage, json.RawMessage, *int64, bool, error) {
	if err := s.GuardianProfileRepo.LockByIDForUpdate(ctx, guardianProfileID); err != nil {
		return nil, nil, nil, false, err
	}
	phones, err := s.GuardianPhoneRepo.FindByGuardianID(ctx, guardianProfileID)
	if err != nil {
		return nil, nil, nil, false, err
	}
	existing := pickPrimaryPhone(phones, func(phone *GuardianPhoneNumber) bool { return phone.IsPrimary })

	oldRaw := json.RawMessage("null")
	if existing != nil {
		oldRaw = jsonString(existing.PhoneNumber)
	}

	// Clear: empty value removes the existing primary phone.
	if newStr == "" {
		if existing == nil {
			return oldRaw, json.RawMessage("null"), nil, false, nil
		}
		ref := existing.ID
		if err := s.Guardians.DeleteGuardianPhoneRecord(ctx, existing.ID); err != nil {
			return nil, nil, nil, false, err
		}
		return oldRaw, json.RawMessage("null"), &ref, true, nil
	}
	if existing != nil {
		return s.changePrimaryPhone(ctx, existing, oldRaw, newStr)
	}
	return s.addPrimaryPhone(ctx, guardianProfileID, tenantID, oldRaw, newStr)
}

// changePrimaryPhone rewrites the number of the existing primary phone.
func (s *Service) changePrimaryPhone(ctx context.Context, existing *GuardianPhoneNumber, oldRaw json.RawMessage, newStr string) (json.RawMessage, json.RawMessage, *int64, bool, error) {
	existing.PhoneNumber = newStr
	if err := s.GuardianPhoneRepo.NormalizePhone(existing); err != nil {
		return nil, nil, nil, false, fmt.Errorf("%w: %s", ErrMasterDataInvalidValue, err.Error())
	}
	newRaw := jsonString(existing.PhoneNumber)
	ref := existing.ID
	if string(oldRaw) == string(newRaw) {
		return oldRaw, newRaw, &ref, false, nil
	}
	if err := s.Guardians.SetGuardianPhoneNumber(ctx, existing.ID, existing.PhoneNumber); err != nil {
		return nil, nil, nil, false, err
	}
	return oldRaw, newRaw, &ref, true, nil
}

// addPrimaryPhone creates the guardian's first primary phone.
func (s *Service) addPrimaryPhone(ctx context.Context, guardianProfileID, tenantID int64, oldRaw json.RawMessage, newStr string) (json.RawMessage, json.RawMessage, *int64, bool, error) {
	phone := &GuardianPhoneNumber{
		GuardianProfileID: guardianProfileID,
		PhoneNumber:       newStr,
		PhoneType:         s.GuardianPhoneRepo.PhoneTypeOrMobile(""),
		IsPrimary:         true,
		Priority:          1,
	}
	phone.TenantID = tenantID
	if err := s.GuardianPhoneRepo.NormalizePhone(phone); err != nil {
		return nil, nil, nil, false, fmt.Errorf("%w: %s", ErrMasterDataInvalidValue, err.Error())
	}
	id, err := s.Guardians.AddGuardianPhoneRecord(ctx, guardianProfileID, GuardianPhoneRecord{
		PhoneNumber: phone.PhoneNumber, PhoneType: string(phone.PhoneType), Label: phone.Label,
		IsPrimary: phone.IsPrimary, Priority: phone.Priority,
	})
	if err != nil {
		return nil, nil, nil, false, err
	}
	phone.ID = id
	ref := phone.ID
	return oldRaw, jsonString(phone.PhoneNumber), &ref, true, nil
}

// recordAutoApplied inserts the Track A audit row for a direct edit.
func (s *Service) recordAutoApplied(ctx context.Context, tenantID, studentID, accountID int64, target, fieldKey string, oldRaw, newRaw json.RawMessage, targetRef *int64) error {
	now := time.Now()
	row := &StudentDataChangeRequest{
		StudentID:   studentID,
		SubmittedBy: accountID,
		Target:      target,
		TargetRefID: targetRef,
		FieldKey:    fieldKey,
		OldValue:    oldRaw,
		NewValue:    newRaw,
		Status:      DataChangeStatusAutoApplied,
		AppliedAt:   &now,
	}
	row.TenantID = tenantID
	return s.createDataRequest(ctx, row)
}

// loadMasterData assembles the Stammdaten DTO inside an existing tenant tx.
func (s *Service) loadMasterData(ctx context.Context, accountID, guardianProfileID, studentID int64) (*ChildMasterData, error) {
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
	if primary := pickPrimaryPhone(phones, func(phone *GuardianPhoneNumber) bool { return phone.IsPrimary }); primary != nil {
		out.PrimaryPhone = &primary.PhoneNumber
		out.PrimaryPhoneID = &primary.ID
	}
	out.PendingChanges, err = s.visiblePendingDataRequests(ctx, accountID, studentID)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// visiblePendingDataRequests returns the child's pending Track B requests the
// caller may see under the family's sharing choices.
func (s *Service) visiblePendingDataRequests(ctx context.Context, accountID, studentID int64) ([]*StudentDataChangeRequest, error) {
	pending, err := s.ChangeRequestRepo.ListByStudent(ctx, studentID, []string{DataChangeStatusPending}, 0)
	if err != nil {
		return nil, err
	}
	visibility, err := s.RequestSharing.LoadRequestShareVisibility(ctx, studentID)
	if err != nil {
		return nil, err
	}
	visible := make([]*StudentDataChangeRequest, 0, len(pending))
	for _, row := range pending {
		if row != nil && visibility.Allows(RequestShareMasterData, row.ID, accountID, row.SubmittedBy) {
			visible = append(visible, row)
		}
	}
	return visible, nil
}

func pickPrimaryPhone[T any](phones []*T, isPrimary func(*T) bool) *T {
	for _, p := range phones {
		if isPrimary(p) {
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
