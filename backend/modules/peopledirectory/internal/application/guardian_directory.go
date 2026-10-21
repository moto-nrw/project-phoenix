package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain/userscontract"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/transactionruntime"
)

// Guardian directory composition (#2663). The People Directory facade owns
// the guardian, phone-number and link capability; the application behind it
// is still the legacy guardian service of the same owner.

// GuardianDirectory binds the legacy guardian service onto the facade
// with the exact transaction and error semantics the api/guardians handlers
// used to apply themselves (#2663). PermissionGranted is the Security
// Runtime's stored-value rule of a parent-portal permission.
type GuardianDirectory struct {
	guardians         peoplePorts.GuardianService
	runtime           peoplePorts.TenantTxRunner
	permissionGranted func(value any) bool
}

// NewGuardianDirectory binds the guardian provider over the legacy guardian
// service.
func NewGuardianDirectory(guardians peoplePorts.GuardianService, runtime peoplePorts.TenantTxRunner, permissionGranted func(value any) bool) *GuardianDirectory {
	return &GuardianDirectory{guardians: guardians, runtime: runtime, permissionGranted: permissionGranted}
}

// inTenantTx runs fn in the tenant transaction of the context, joining the
// request transaction when one is open. Every write of the old handlers ran
// this way, so a failure rolls the whole request back.
func (p *GuardianDirectory) inTenantTx(ctx context.Context, fn func(context.Context) error) error {
	return p.runtime.InTenantTx(ctx, fn)
}

func (p *GuardianDirectory) FindGuardian(ctx context.Context, id int64) (peopledirectory.Guardian, error) {
	profile, err := p.guardians.GetGuardianByID(ctx, id)
	if err != nil {
		return peopledirectory.Guardian{}, mapGuardianError(err)
	}
	return toDirectoryGuardian(profile), nil
}

func (p *GuardianDirectory) FindGuardianByEmail(ctx context.Context, email string) (peopledirectory.Guardian, error) {
	profile, err := p.guardians.GuardianProfiles().FindByEmail(ctx, email)
	if err != nil {
		return peopledirectory.Guardian{}, mapGuardianError(err)
	}
	return toDirectoryGuardian(profile), nil
}

func (p *GuardianDirectory) ListGuardians(ctx context.Context, page, pageSize int) ([]peopledirectory.Guardian, error) {
	profiles, err := p.guardians.ListGuardians(ctx, peoplePorts.NewQueryOptions().WithPagination(page, pageSize))
	if err != nil {
		return nil, mapGuardianError(err)
	}
	return toDirectoryGuardians(profiles), nil
}

func (p *GuardianDirectory) ListGuardiansWithoutAccount(ctx context.Context) ([]peopledirectory.Guardian, error) {
	profiles, err := p.guardians.GuardianProfiles().FindWithoutAccount(ctx)
	if err != nil {
		return nil, mapGuardianError(err)
	}
	return toDirectoryGuardians(profiles), nil
}

func (p *GuardianDirectory) ListInvitableGuardians(ctx context.Context) ([]peopledirectory.Guardian, error) {
	profiles, err := p.guardians.GuardianProfiles().FindInvitable(ctx)
	if err != nil {
		return nil, mapGuardianError(err)
	}
	return toDirectoryGuardians(profiles), nil
}

func (p *GuardianDirectory) SearchGuardians(ctx context.Context, text string, limit int) ([]peopledirectory.GuardianMatch, error) {
	matches, err := p.guardians.SearchGuardiansForPicker(ctx, text, limit)
	if err != nil {
		return nil, mapGuardianError(err)
	}
	result := make([]peopledirectory.GuardianMatch, 0, len(matches))
	for _, match := range matches {
		result = append(result, peopledirectory.GuardianMatch{
			Guardian:            toDirectoryGuardian(match.Profile),
			LinkedChildrenCount: len(match.Children),
		})
	}
	return result, nil
}

func (p *GuardianDirectory) GuardianDeleteImpact(ctx context.Context, id int64) (peopledirectory.GuardianDeleteImpact, error) {
	impact, err := p.guardians.GetGuardianDeleteImpact(ctx, id)
	if err != nil {
		return peopledirectory.GuardianDeleteImpact{}, mapGuardianError(err)
	}
	return peopledirectory.GuardianDeleteImpact{LinkIDs: impact.LinkIDs, StudentNames: impact.StudentNames}, nil
}

func (p *GuardianDirectory) ListGuardianPhones(ctx context.Context, guardianID int64) ([]peopledirectory.GuardianPhone, error) {
	phones, err := p.guardians.GuardianPhoneNumbers().FindByGuardianID(ctx, guardianID)
	if err != nil {
		return nil, mapGuardianError(err)
	}
	return toDirectoryPhones(phones), nil
}

func (p *GuardianDirectory) FindGuardianPhone(ctx context.Context, phoneID int64) (peopledirectory.GuardianPhone, error) {
	phone, err := p.guardians.GuardianPhoneNumbers().FindByID(ctx, phoneID)
	if err != nil {
		return peopledirectory.GuardianPhone{}, mapGuardianPhoneError(err)
	}
	return toDirectoryPhone(phone), nil
}

func (p *GuardianDirectory) ListStudentGuardians(ctx context.Context, studentID int64) ([]peopledirectory.GuardianWithLink, error) {
	rows, err := p.guardians.GetStudentGuardians(ctx, studentID)
	if err != nil {
		return nil, mapGuardianError(err)
	}
	result := make([]peopledirectory.GuardianWithLink, 0, len(rows))
	for _, row := range rows {
		result = append(result, peopledirectory.GuardianWithLink{
			Guardian:          toDirectoryGuardian(row.Profile),
			Link:              toDirectoryLink(row.Relationship, p.permissionGranted),
			InvitationPending: row.InvitationPending,
		})
	}
	return result, nil
}

func (p *GuardianDirectory) ListGuardianStudents(ctx context.Context, guardianID int64) ([]peopledirectory.StudentWithLink, error) {
	rows, err := p.guardians.GetGuardianStudents(ctx, guardianID)
	if err != nil {
		return nil, mapGuardianError(err)
	}
	result := make([]peopledirectory.StudentWithLink, 0, len(rows))
	for _, row := range rows {
		result = append(result, peopledirectory.StudentWithLink{
			Student: ToDirectoryStudent(row.Student),
			Link:    toDirectoryLink(row.Relationship, p.permissionGranted),
		})
	}
	return result, nil
}

func (p *GuardianDirectory) FindGuardianLink(ctx context.Context, linkID int64) (peopledirectory.GuardianLink, error) {
	link, err := p.guardians.GetStudentGuardianRelationship(ctx, linkID)
	if err != nil {
		return peopledirectory.GuardianLink{}, mapGuardianLinkError(err)
	}
	return toDirectoryLink(link, p.permissionGranted), nil
}

func (p *GuardianDirectory) GuardianPaymentMasked(ctx context.Context, guardianID int64, actor peopledirectory.GuardianPaymentActor) (peopledirectory.GuardianPayment, error) {
	masked, err := p.guardians.GetGuardianPaymentMasked(ctx, guardianID, actor.AccountID, actor.Role)
	if err != nil {
		return peopledirectory.GuardianPayment{}, mapGuardianPaymentError(err)
	}
	return peopledirectory.GuardianPayment{GuardianProfileID: masked.GuardianProfileID, IBAN: masked.IBANMasked, AccountHolder: masked.AccountHolder}, nil
}

func (p *GuardianDirectory) ListPaymentOverview(ctx context.Context, actor peopledirectory.GuardianPaymentActor) ([]peopledirectory.GuardianPaymentRow, error) {
	rows, err := p.guardians.ListPaymentOverview(ctx, actor.AccountID, actor.Role)
	if err != nil {
		return nil, mapGuardianPaymentError(err)
	}
	return toDirectoryPaymentRows(rows), nil
}

func (p *GuardianDirectory) ListPaymentExportRows(ctx context.Context, actor peopledirectory.GuardianPaymentActor, format string) ([]peopledirectory.GuardianPaymentRow, error) {
	rows, err := p.guardians.ListPaymentExportRows(ctx, actor.AccountID, actor.Role, format)
	if err != nil {
		return nil, mapGuardianPaymentError(err)
	}
	return toDirectoryPaymentRows(rows), nil
}

func (p *GuardianDirectory) CreateGuardian(ctx context.Context, input peopledirectory.GuardianInput) (peopledirectory.Guardian, error) {
	var profile *peopledirectory.GuardianProfileSnapshot
	err := p.inTenantTx(ctx, func(txCtx context.Context) error {
		var txErr error
		profile, txErr = p.guardians.CreateGuardian(txCtx, toGuardianCreateRequest(input))
		return txErr
	})
	if err != nil {
		return peopledirectory.Guardian{}, mapGuardianError(err)
	}
	return toDirectoryGuardian(profile), nil
}

func (p *GuardianDirectory) UpdateGuardian(ctx context.Context, id int64, input peopledirectory.GuardianInput) error {
	return mapGuardianError(p.inTenantTx(ctx, func(txCtx context.Context) error {
		return p.guardians.UpdateGuardian(txCtx, id, toGuardianCreateRequest(input))
	}))
}

func (p *GuardianDirectory) EvaluateGuardianDelete(ctx context.Context, id int64, force, isAdmin bool) (bool, error) {
	hasLinks, err := p.guardians.EvaluateGuardianDelete(ctx, id, force, isAdmin)
	return hasLinks, mapGuardianError(err)
}

func (p *GuardianDirectory) DeleteGuardian(ctx context.Context, input peopledirectory.GuardianDelete) error {
	return mapGuardianError(p.inTenantTx(ctx, func(txCtx context.Context) error {
		if input.WithLinks {
			return p.guardians.DeleteGuardianWithLinks(txCtx, input.GuardianID, input.ExpectedLinkIDs, input.ActorAccountID)
		}
		return p.guardians.DeleteGuardian(txCtx, input.GuardianID, input.ActorAccountID)
	}))
}

func (p *GuardianDirectory) AddGuardianPhone(ctx context.Context, guardianID int64, input peopledirectory.GuardianPhoneInput) (peopledirectory.GuardianPhone, error) {
	var phone *peopledirectory.GuardianProfilePhoneSnapshot
	err := p.inTenantTx(ctx, func(txCtx context.Context) error {
		var txErr error
		phone, txErr = p.guardians.AddPhoneNumber(txCtx, guardianID, peoplePorts.PhoneNumberCreateRequest{
			PhoneNumber: input.PhoneNumber, PhoneType: input.PhoneType, Label: input.Label, IsPrimary: input.IsPrimary,
		})
		return txErr
	})
	if err != nil {
		return peopledirectory.GuardianPhone{}, mapGuardianError(err)
	}
	return toDirectoryPhone(phone), nil
}

func (p *GuardianDirectory) UpdateGuardianPhone(ctx context.Context, phoneID int64, input peopledirectory.GuardianPhoneUpdate) error {
	return mapGuardianPhoneError(p.inTenantTx(ctx, func(txCtx context.Context) error {
		return p.guardians.UpdatePhoneNumber(txCtx, phoneID, peoplePorts.PhoneNumberUpdateRequest{
			PhoneNumber: input.PhoneNumber, PhoneType: input.PhoneType, Label: input.Label, IsPrimary: input.IsPrimary, Priority: input.Priority,
		})
	}))
}

func (p *GuardianDirectory) DeleteGuardianPhone(ctx context.Context, phoneID int64) error {
	return mapGuardianPhoneError(p.inTenantTx(ctx, func(txCtx context.Context) error {
		return p.guardians.DeletePhoneNumber(txCtx, phoneID)
	}))
}

func (p *GuardianDirectory) SetPrimaryGuardianPhone(ctx context.Context, phoneID int64) error {
	return mapGuardianPhoneError(p.inTenantTx(ctx, func(txCtx context.Context) error {
		return p.guardians.SetPrimaryPhone(txCtx, phoneID)
	}))
}

func (p *GuardianDirectory) LinkGuardianToStudent(ctx context.Context, input peopledirectory.LinkGuardian) (peopledirectory.GuardianLink, error) {
	var link *peopledirectory.GuardianRelationshipSnapshot
	err := p.inTenantTx(ctx, func(txCtx context.Context) error {
		var txErr error
		link, txErr = p.guardians.LinkGuardianToStudent(txCtx, peoplePorts.StudentGuardianCreateRequest{
			StudentID: input.StudentID, GuardianProfileID: input.GuardianProfileID,
			RelationshipType: input.RelationshipType, GuardianRole: input.GuardianRole,
			IsPrimary: input.IsPrimary, IsEmergencyContact: input.IsEmergencyContact, CanPickup: input.CanPickup,
			PickupNotes: input.PickupNotes, EmergencyPriority: input.EmergencyPriority,
		})
		return txErr
	})
	if err != nil {
		return peopledirectory.GuardianLink{}, mapGuardianError(err)
	}
	return toDirectoryLink(link, p.permissionGranted), nil
}

func (p *GuardianDirectory) ValidateNewGuardians(ctx context.Context, guardians []peopledirectory.NewStudentGuardian) error {
	return mapGuardianError(p.guardians.ValidateNewGuardians(ctx, toNewStudentGuardians(guardians)))
}

// AddGuardiansToStudent joins the caller's transaction: the student create
// flow and the batch endpoint both open theirs first, and a validation
// failure must roll back everything they wrote.
func (p *GuardianDirectory) AddGuardiansToStudent(ctx context.Context, studentID int64, guardians []peopledirectory.NewStudentGuardian) error {
	return mapGuardianError(p.inTenantTx(ctx, func(txCtx context.Context) error {
		return p.guardians.AddGuardiansToStudent(txCtx, studentID, toNewStudentGuardians(guardians))
	}))
}

func (p *GuardianDirectory) UpdateGuardianLink(ctx context.Context, linkID int64, input peopledirectory.GuardianLinkUpdate) error {
	return mapGuardianLinkError(p.inTenantTx(ctx, func(txCtx context.Context) error {
		return p.guardians.UpdateStudentGuardianRelationship(txCtx, linkID, peoplePorts.StudentGuardianUpdateRequest{
			RelationshipType: input.RelationshipType, GuardianRole: input.GuardianRole,
			IsPrimary: input.IsPrimary, IsEmergencyContact: input.IsEmergencyContact, CanPickup: input.CanPickup,
			PickupNotes: input.PickupNotes, EmergencyPriority: input.EmergencyPriority,
		})
	}))
}

func (p *GuardianDirectory) RemoveGuardianFromStudent(ctx context.Context, input peopledirectory.RemoveGuardian) error {
	return mapGuardianError(p.inTenantTx(ctx, func(txCtx context.Context) error {
		return p.guardians.RemoveGuardianFromStudent(txCtx, input.StudentID, input.GuardianProfileID, input.ActorAccountID, input.MayClearPayer)
	}))
}

func (p *GuardianDirectory) RevealGuardianPayment(ctx context.Context, guardianID int64, actor peopledirectory.GuardianPaymentActor) (peopledirectory.GuardianPayment, error) {
	plain, err := p.guardians.RevealGuardianPayment(ctx, guardianID, actor.AccountID, actor.Role)
	if err != nil {
		return peopledirectory.GuardianPayment{}, mapGuardianPaymentError(err)
	}
	return peopledirectory.GuardianPayment{GuardianProfileID: plain.GuardianProfileID, IBAN: plain.IBAN, AccountHolder: plain.AccountHolder}, nil
}

func (p *GuardianDirectory) UpdateGuardianPayment(ctx context.Context, guardianID int64, input peopledirectory.GuardianPaymentInput) error {
	return mapGuardianPaymentError(p.guardians.UpdateGuardianPayment(ctx, guardianID, peoplePorts.GuardianPaymentInput{
		IBAN: input.IBAN, AccountHolder: input.AccountHolder,
	}, input.ActorAccountID, input.Note))
}

func (p *GuardianDirectory) SetStudentPayer(ctx context.Context, input peopledirectory.StudentPayer) error {
	return mapGuardianPaymentError(p.guardians.SetStudentPayer(ctx, input.StudentID, input.GuardianProfileID, input.ActorAccountID))
}

// --- error mapping ---

// mapGuardianError translates the legacy service errors onto the facade
// sentinels. Unknown errors pass through unchanged so a read failure keeps
// its cause and is rendered as a 500.
func mapGuardianError(err error) error {
	if err == nil {
		return nil
	}
	validation, isValidation := peoplePorts.ValidationFailure(err)
	var stillLinked *peoplePorts.GuardianStillLinkedError
	switch {
	case isValidation && errors.Is(err, peoplePorts.ErrGuardianEmailInUse):
		// Still invalid input for every caller; the sentinel adds the code.
		return fmt.Errorf("%w: %w", peopledirectory.ErrGuardianEmailTaken,
			&peopledirectory.InvalidGuardianError{Reason: validation.Error()})
	case isValidation:
		return &peopledirectory.InvalidGuardianError{Reason: validation.Error()}
	case errors.As(err, &stillLinked):
		return &peopledirectory.GuardianStillLinkedError{StudentNames: stillLinked.StudentNames}
	case errors.Is(err, peoplePorts.ErrGuardianForceDeleteRequiresAdmin):
		return peopledirectory.ErrGuardianForceDeleteRequiresAdmin
	case errors.Is(err, peoplePorts.ErrGuardianDeletePreviewChanged):
		return peopledirectory.ErrGuardianDeletePreviewChanged
	case errors.Is(err, peoplePorts.ErrPayerRemovalRequiresFinancial):
		return peopledirectory.ErrPayerRemovalRequiresFinancial
	case errors.Is(err, peopledirectory.ErrGuardianProfileNotFound):
		return fmt.Errorf("%w: %w", peopledirectory.ErrGuardianNotFound, err)
	case errors.Is(err, peopledirectory.ErrStudentGuardianNotFound):
		return fmt.Errorf("%w: %w", peopledirectory.ErrGuardianLinkNotFound, err)
	case peoplePorts.IsConstraintViolation(err): // a child link still references the profile
		return fmt.Errorf("%w: %w", peopledirectory.ErrGuardianLinkConflict, err)
	case errors.Is(err, peopledirectory.ErrGuardianNotFound) || errors.Is(err, peopledirectory.ErrGuardianPhoneNotFound) || errors.Is(err, peopledirectory.ErrGuardianLinkNotFound) || errors.Is(err, peopledirectory.ErrStudentNotFound) || errors.Is(err, transactionruntime.ErrNoRows) || errors.Is(err, userscontract.ErrStudentNotFound):
		// Lookup failures retain their existing text and expose an identity.
		return fmt.Errorf("%w: %w", peopledirectory.ErrGuardianNotFound, err)
	default:
		return err
	}
}

func mapGuardianPhoneError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, peopledirectory.ErrGuardianPhoneNotFound) {
		return fmt.Errorf("%w: %w", peopledirectory.ErrGuardianPhoneNotFound, err)
	}
	return mapGuardianError(err)
}

func mapGuardianLinkError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, peopledirectory.ErrGuardianLinkNotFound) {
		return fmt.Errorf("%w: %w", peopledirectory.ErrGuardianLinkNotFound, err)
	}
	return mapGuardianError(err)
}

func mapGuardianPaymentError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, transactionruntime.ErrNoRows), errors.Is(err, peopledirectory.ErrGuardianProfileNotFound):
		return fmt.Errorf("%w: %w", peopledirectory.ErrGuardianNotFound, err)
	case errors.Is(err, peoplePorts.ErrGuardianIBANInvalid):
		return peopledirectory.ErrGuardianIBANInvalid
	case errors.Is(err, peoplePorts.ErrGuardianAccountHolderTooLong):
		return peopledirectory.ErrGuardianAccountHolderTooLong
	case errors.Is(err, peoplePorts.ErrGuardianStudentRequired):
		return peopledirectory.ErrGuardianStudentRequired
	case errors.Is(err, peoplePorts.ErrGuardianNotLinkedToStudent):
		return peopledirectory.ErrGuardianNotLinkedToStudent
	case errors.Is(err, peoplePorts.ErrGuardianPaymentInvalid):
		return fmt.Errorf("%w: %w", peopledirectory.ErrGuardianPaymentInvalid, err)
	default:
		return err
	}
}

// --- projections ---

func toGuardianCreateRequest(input peopledirectory.GuardianInput) peoplePorts.GuardianCreateRequest {
	return peoplePorts.GuardianCreateRequest{
		FirstName: input.FirstName, LastName: input.LastName, Email: input.Email,
		AddressStreet: input.AddressStreet, AddressCity: input.AddressCity, AddressPostalCode: input.AddressPostalCode,
		PreferredContactMethod: input.PreferredContactMethod, LanguagePreference: input.LanguagePreference, Notes: input.Notes,
	}
}

func toNewStudentGuardians(inputs []peopledirectory.NewStudentGuardian) []peoplePorts.NewStudentGuardian {
	out := make([]peoplePorts.NewStudentGuardian, 0, len(inputs))
	for i := range inputs {
		in := inputs[i]
		out = append(out, peoplePorts.NewStudentGuardian{
			Profile: peoplePorts.GuardianCreateRequest{
				FirstName:              strings.TrimSpace(in.FirstName),
				LastName:               strings.TrimSpace(in.LastName),
				Email:                  trimToNil(in.Email),
				AddressStreet:          trimToNil(in.AddressStreet),
				AddressCity:            trimToNil(in.AddressCity),
				AddressPostalCode:      trimToNil(in.AddressPostalCode),
				PreferredContactMethod: in.PreferredContactMethod,
				LanguagePreference:     in.LanguagePreference,
				Notes:                  trimToNil(in.Notes),
			},
			Relationship: peoplePorts.StudentGuardianRelationship{
				RelationshipType:   in.RelationshipType,
				GuardianRole:       in.GuardianRole,
				IsPrimary:          in.IsPrimary,
				IsEmergencyContact: in.IsEmergencyContact,
				CanPickup:          in.CanPickup,
				PickupNotes:        trimToNil(in.PickupNotes),
				EmergencyPriority:  in.EmergencyPriority,
			},
			PhoneNumbers:      toNewGuardianPhones(in.PhoneNumbers),
			ExistingProfileID: in.GuardianProfileID,
		})
	}
	return out
}

func toNewGuardianPhones(phones []peopledirectory.NewGuardianPhone) []peoplePorts.PhoneNumberCreateRequest {
	if len(phones) == 0 {
		return nil
	}
	out := make([]peoplePorts.PhoneNumberCreateRequest, 0, len(phones))
	for i := range phones {
		out = append(out, peoplePorts.PhoneNumberCreateRequest{
			PhoneNumber: strings.TrimSpace(phones[i].PhoneNumber),
			PhoneType:   phones[i].PhoneType,
			Label:       trimToNil(phones[i].Label),
			IsPrimary:   phones[i].IsPrimary,
		})
	}
	return out
}

// trimToNil trims the value and returns nil for an empty result, the shape
// the optional guardian columns take.
func trimToNil(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func toDirectoryGuardians(profiles []*peopledirectory.GuardianProfileSnapshot) []peopledirectory.Guardian {
	result := make([]peopledirectory.Guardian, 0, len(profiles))
	for _, profile := range profiles {
		if profile == nil {
			continue
		}
		result = append(result, toDirectoryGuardian(profile))
	}
	return result
}

func toDirectoryGuardian(profile *peopledirectory.GuardianProfileSnapshot) peopledirectory.Guardian {
	if profile == nil {
		return peopledirectory.Guardian{}
	}
	return peopledirectory.Guardian{
		ID: profile.ID, TenantID: profile.GetTenantID(),
		FirstName: profile.FirstName, LastName: profile.LastName, Email: profile.Email,
		AddressStreet: profile.AddressStreet, AddressCity: profile.AddressCity, AddressPostalCode: profile.AddressPostalCode,
		PreferredContactMethod: profile.PreferredContactMethod, LanguagePreference: profile.LanguagePreference,
		Notes: profile.Notes, HasAccount: profile.HasAccount, AccountID: profile.AccountID,
		PhoneNumbers: toDirectoryPhones(profile.PhoneNumbers),
	}
}

func toDirectoryPhones(phones []*peopledirectory.GuardianProfilePhoneSnapshot) []peopledirectory.GuardianPhone {
	if len(phones) == 0 {
		return nil
	}
	result := make([]peopledirectory.GuardianPhone, 0, len(phones))
	for _, phone := range phones {
		if phone == nil {
			continue
		}
		result = append(result, toDirectoryPhone(phone))
	}
	return result
}

func toDirectoryPhone(phone *peopledirectory.GuardianProfilePhoneSnapshot) peopledirectory.GuardianPhone {
	return peopledirectory.GuardianPhone{
		ID: phone.ID, GuardianProfileID: phone.GuardianProfileID, PhoneNumber: phone.PhoneNumber,
		PhoneType: string(phone.PhoneType), Label: phone.Label, IsPrimary: phone.IsPrimary, Priority: phone.Priority,
	}
}

func toDirectoryLink(link *peopledirectory.GuardianRelationshipSnapshot, granted func(any) bool) peopledirectory.GuardianLink {
	if link == nil {
		return peopledirectory.GuardianLink{}
	}
	return peopledirectory.GuardianLink{
		ID: link.ID, TenantID: link.GetTenantID(), StudentID: link.StudentID, GuardianProfileID: link.GuardianProfileID,
		RelationshipType: link.RelationshipType, GuardianRole: link.GuardianRole,
		IsPrimary: link.IsPrimary, IsEmergencyContact: link.IsEmergencyContact, CanPickup: link.CanPickup,
		PickupNotes: link.PickupNotes, EmergencyPriority: link.EmergencyPriority, IsPayer: link.IsPayer,
		Permissions: grantedGuardianPermissions(link, granted),
	}
}

// ToDirectoryStudent maps a retained student row onto the public student.
func ToDirectoryStudent(student *peopledirectory.StudentState) peopledirectory.Student {
	if student == nil {
		return peopledirectory.Student{}
	}
	return peopledirectory.Student{
		ID: student.ID, CreatedAt: student.CreatedAt, UpdatedAt: student.UpdatedAt, TenantID: student.GetTenantID(),
		PersonID: student.PersonID, SchoolClass: student.SchoolClass, GroupID: student.GroupID, Status: string(student.Status),
	}
}

func toDirectoryPaymentRows(rows []peoplePorts.GuardianPaymentRow) []peopledirectory.GuardianPaymentRow {
	result := make([]peopledirectory.GuardianPaymentRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, peopledirectory.GuardianPaymentRow{
			StudentID: row.StudentID, StudentName: row.StudentName, SchoolClass: row.SchoolClass,
			GuardianProfileID: row.GuardianProfileID, GuardianName: row.GuardianName, RelationshipType: row.RelationshipType,
			AccountHolder: row.AccountHolder, IBAN: row.IBAN, IBANMasked: row.IBANMasked,
		})
	}
	return result
}

// grantedGuardianPermissions lists the parents-portal permission names a
// student-guardian link grants, in stable order, under the stored-value rule.
func grantedGuardianPermissions(link *peopledirectory.GuardianRelationshipSnapshot, granted func(any) bool) []string {
	if link == nil {
		return nil
	}
	var permissions map[string]any
	if json.Unmarshal(link.Permissions, &permissions) != nil {
		return nil
	}
	names := make([]string, 0, len(permissions))
	for name, value := range permissions {
		if granted(value) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}
