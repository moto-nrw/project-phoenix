package compose

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

// Guardian directory composition (#2663). The People Directory facade owns
// the guardian, phone-number and link capability; the application behind it
// is still the legacy guardian service of the same owner.

// GuardianProviderBinder is what the People Directory module exposes for the
// late binding of its guardian provider.
type GuardianProviderBinder interface {
	BindGuardianProvider(peopledirectory.GuardianProvider)
}

// BindGuardianDirectory installs the guardian provider on the People
// Directory. A capability without the binder cannot serve guardians at all,
// so the composition refuses to continue silently.
func BindGuardianDirectory(persons peopledirectory.Capability, provider *GuardianDirectoryProvider) {
	binder, ok := persons.(GuardianProviderBinder)
	if !ok {
		panic(fmt.Sprintf("guardian directory: people directory %T cannot bind the guardian provider", persons))
	}
	binder.BindGuardianProvider(provider)
}

// GuardianDirectoryProvider binds the legacy guardian service onto the facade
// with the exact transaction and error semantics the api/guardians handlers
// used to apply themselves (#2663). PermissionGranted is the Security
// Runtime's stored-value rule of a parent-portal permission.
type GuardianDirectoryProvider struct {
	Guardians         peoplePorts.GuardianService
	Runtime           LegacyRepositoryRuntime
	PermissionGranted func(value any) bool
}

// inTenantTx runs fn in the tenant transaction of the context, joining the
// request transaction when one is open. Every write of the old handlers ran
// this way, so a failure rolls the whole request back.
func (p *GuardianDirectoryProvider) inTenantTx(ctx context.Context, fn func(context.Context) error) error {
	return p.Runtime.InTenantTx(ctx, fn)
}

func (p *GuardianDirectoryProvider) FindGuardian(ctx context.Context, id int64) (peopledirectory.Guardian, error) {
	profile, err := p.Guardians.GetGuardianByID(ctx, id)
	if err != nil {
		return peopledirectory.Guardian{}, mapGuardianError(err)
	}
	return toDirectoryGuardian(profile), nil
}

func (p *GuardianDirectoryProvider) FindGuardianByEmail(ctx context.Context, email string) (peopledirectory.Guardian, error) {
	profile, err := p.Guardians.GuardianProfiles().FindByEmail(ctx, email)
	if err != nil {
		return peopledirectory.Guardian{}, mapGuardianError(err)
	}
	return toDirectoryGuardian(profile), nil
}

func (p *GuardianDirectoryProvider) ListGuardians(ctx context.Context, page, pageSize int) ([]peopledirectory.Guardian, error) {
	profiles, err := p.Guardians.ListGuardians(ctx, peoplePorts.NewQueryOptions().WithPagination(page, pageSize))
	if err != nil {
		return nil, mapGuardianError(err)
	}
	return toDirectoryGuardians(profiles), nil
}

func (p *GuardianDirectoryProvider) ListGuardiansWithoutAccount(ctx context.Context) ([]peopledirectory.Guardian, error) {
	profiles, err := p.Guardians.GetGuardiansWithoutAccount(ctx)
	if err != nil {
		return nil, mapGuardianError(err)
	}
	return toDirectoryGuardians(profiles), nil
}

func (p *GuardianDirectoryProvider) ListInvitableGuardians(ctx context.Context) ([]peopledirectory.Guardian, error) {
	profiles, err := p.Guardians.GetInvitableGuardians(ctx)
	if err != nil {
		return nil, mapGuardianError(err)
	}
	return toDirectoryGuardians(profiles), nil
}

func (p *GuardianDirectoryProvider) SearchGuardians(ctx context.Context, text string, limit int) ([]peopledirectory.GuardianMatch, error) {
	matches, err := p.Guardians.SearchGuardiansForPicker(ctx, text, limit)
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

func (p *GuardianDirectoryProvider) GuardianDeleteImpact(ctx context.Context, id int64) (peopledirectory.GuardianDeleteImpact, error) {
	impact, err := p.Guardians.GetGuardianDeleteImpact(ctx, id)
	if err != nil {
		return peopledirectory.GuardianDeleteImpact{}, mapGuardianError(err)
	}
	return peopledirectory.GuardianDeleteImpact{LinkIDs: impact.LinkIDs, StudentNames: impact.StudentNames}, nil
}

func (p *GuardianDirectoryProvider) ListGuardianPhones(ctx context.Context, guardianID int64) ([]peopledirectory.GuardianPhone, error) {
	phones, err := p.Guardians.GetGuardianPhoneNumbers(ctx, guardianID)
	if err != nil {
		return nil, mapGuardianError(err)
	}
	return toDirectoryPhones(phones), nil
}

func (p *GuardianDirectoryProvider) FindGuardianPhone(ctx context.Context, phoneID int64) (peopledirectory.GuardianPhone, error) {
	phone, err := p.Guardians.GetPhoneNumberByID(ctx, phoneID)
	if err != nil {
		return peopledirectory.GuardianPhone{}, mapGuardianPhoneError(err)
	}
	return toDirectoryPhone(phone), nil
}

func (p *GuardianDirectoryProvider) ListStudentGuardians(ctx context.Context, studentID int64) ([]peopledirectory.GuardianWithLink, error) {
	rows, err := p.Guardians.GetStudentGuardians(ctx, studentID)
	if err != nil {
		return nil, mapGuardianError(err)
	}
	result := make([]peopledirectory.GuardianWithLink, 0, len(rows))
	for _, row := range rows {
		result = append(result, peopledirectory.GuardianWithLink{
			Guardian:          toDirectoryGuardian(row.Profile),
			Link:              toDirectoryLink(row.Relationship, p.PermissionGranted),
			InvitationPending: row.InvitationPending,
		})
	}
	return result, nil
}

func (p *GuardianDirectoryProvider) ListGuardianStudents(ctx context.Context, guardianID int64) ([]peopledirectory.StudentWithLink, error) {
	rows, err := p.Guardians.GetGuardianStudents(ctx, guardianID)
	if err != nil {
		return nil, mapGuardianError(err)
	}
	result := make([]peopledirectory.StudentWithLink, 0, len(rows))
	for _, row := range rows {
		result = append(result, peopledirectory.StudentWithLink{
			Student: toDirectoryStudent(row.Student),
			Link:    toDirectoryLink(row.Relationship, p.PermissionGranted),
		})
	}
	return result, nil
}

func (p *GuardianDirectoryProvider) FindGuardianLink(ctx context.Context, linkID int64) (peopledirectory.GuardianLink, error) {
	link, err := p.Guardians.GetStudentGuardianRelationship(ctx, linkID)
	if err != nil {
		return peopledirectory.GuardianLink{}, mapGuardianLinkError(err)
	}
	return toDirectoryLink(link, p.PermissionGranted), nil
}

func (p *GuardianDirectoryProvider) GuardianPaymentMasked(ctx context.Context, guardianID int64, actor peopledirectory.GuardianPaymentActor) (peopledirectory.GuardianPayment, error) {
	masked, err := p.Guardians.GetGuardianPaymentMasked(ctx, guardianID, actor.AccountID, actor.Role)
	if err != nil {
		return peopledirectory.GuardianPayment{}, mapGuardianPaymentError(err)
	}
	return peopledirectory.GuardianPayment{GuardianProfileID: masked.GuardianProfileID, IBAN: masked.IBANMasked, AccountHolder: masked.AccountHolder}, nil
}

func (p *GuardianDirectoryProvider) ListPaymentOverview(ctx context.Context, actor peopledirectory.GuardianPaymentActor) ([]peopledirectory.GuardianPaymentRow, error) {
	rows, err := p.Guardians.ListPaymentOverview(ctx, actor.AccountID, actor.Role)
	if err != nil {
		return nil, mapGuardianPaymentError(err)
	}
	return toDirectoryPaymentRows(rows), nil
}

func (p *GuardianDirectoryProvider) ListPaymentExportRows(ctx context.Context, actor peopledirectory.GuardianPaymentActor, format string) ([]peopledirectory.GuardianPaymentRow, error) {
	rows, err := p.Guardians.ListPaymentExportRows(ctx, actor.AccountID, actor.Role, format)
	if err != nil {
		return nil, mapGuardianPaymentError(err)
	}
	return toDirectoryPaymentRows(rows), nil
}

func (p *GuardianDirectoryProvider) CreateGuardian(ctx context.Context, input peopledirectory.GuardianInput) (peopledirectory.Guardian, error) {
	var profile *users.GuardianProfile
	err := p.inTenantTx(ctx, func(txCtx context.Context) error {
		var txErr error
		profile, txErr = p.Guardians.CreateGuardian(txCtx, toGuardianCreateRequest(input))
		return txErr
	})
	if err != nil {
		return peopledirectory.Guardian{}, mapGuardianError(err)
	}
	return toDirectoryGuardian(profile), nil
}

func (p *GuardianDirectoryProvider) UpdateGuardian(ctx context.Context, id int64, input peopledirectory.GuardianInput) error {
	return mapGuardianError(p.inTenantTx(ctx, func(txCtx context.Context) error {
		return p.Guardians.UpdateGuardian(txCtx, id, toGuardianCreateRequest(input))
	}))
}

func (p *GuardianDirectoryProvider) EvaluateGuardianDelete(ctx context.Context, id int64, force, isAdmin bool) (bool, error) {
	hasLinks, err := p.Guardians.EvaluateGuardianDelete(ctx, id, force, isAdmin)
	return hasLinks, mapGuardianError(err)
}

func (p *GuardianDirectoryProvider) DeleteGuardian(ctx context.Context, input peopledirectory.GuardianDelete) error {
	return mapGuardianError(p.inTenantTx(ctx, func(txCtx context.Context) error {
		if input.WithLinks {
			return p.Guardians.DeleteGuardianWithLinks(txCtx, input.GuardianID, input.ExpectedLinkIDs, input.ActorAccountID)
		}
		return p.Guardians.DeleteGuardian(txCtx, input.GuardianID, input.ActorAccountID)
	}))
}

func (p *GuardianDirectoryProvider) AddGuardianPhone(ctx context.Context, guardianID int64, input peopledirectory.GuardianPhoneInput) (peopledirectory.GuardianPhone, error) {
	var phone *users.GuardianPhoneNumber
	err := p.inTenantTx(ctx, func(txCtx context.Context) error {
		var txErr error
		phone, txErr = p.Guardians.AddPhoneNumber(txCtx, guardianID, peoplePorts.PhoneNumberCreateRequest{
			PhoneNumber: input.PhoneNumber, PhoneType: input.PhoneType, Label: input.Label, IsPrimary: input.IsPrimary,
		})
		return txErr
	})
	if err != nil {
		return peopledirectory.GuardianPhone{}, mapGuardianError(err)
	}
	return toDirectoryPhone(phone), nil
}

func (p *GuardianDirectoryProvider) UpdateGuardianPhone(ctx context.Context, phoneID int64, input peopledirectory.GuardianPhoneUpdate) error {
	return mapGuardianPhoneError(p.inTenantTx(ctx, func(txCtx context.Context) error {
		return p.Guardians.UpdatePhoneNumber(txCtx, phoneID, peoplePorts.PhoneNumberUpdateRequest{
			PhoneNumber: input.PhoneNumber, PhoneType: input.PhoneType, Label: input.Label, IsPrimary: input.IsPrimary, Priority: input.Priority,
		})
	}))
}

func (p *GuardianDirectoryProvider) DeleteGuardianPhone(ctx context.Context, phoneID int64) error {
	return mapGuardianPhoneError(p.inTenantTx(ctx, func(txCtx context.Context) error {
		return p.Guardians.DeletePhoneNumber(txCtx, phoneID)
	}))
}

func (p *GuardianDirectoryProvider) SetPrimaryGuardianPhone(ctx context.Context, phoneID int64) error {
	return mapGuardianPhoneError(p.inTenantTx(ctx, func(txCtx context.Context) error {
		return p.Guardians.SetPrimaryPhone(txCtx, phoneID)
	}))
}

func (p *GuardianDirectoryProvider) LinkGuardianToStudent(ctx context.Context, input peopledirectory.LinkGuardian) (peopledirectory.GuardianLink, error) {
	var link *users.StudentGuardian
	err := p.inTenantTx(ctx, func(txCtx context.Context) error {
		var txErr error
		link, txErr = p.Guardians.LinkGuardianToStudent(txCtx, peoplePorts.StudentGuardianCreateRequest{
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
	return toDirectoryLink(link, p.PermissionGranted), nil
}

func (p *GuardianDirectoryProvider) ValidateNewGuardians(ctx context.Context, guardians []peopledirectory.NewStudentGuardian) error {
	return mapGuardianError(p.Guardians.ValidateNewGuardians(ctx, toNewStudentGuardians(guardians)))
}

// AddGuardiansToStudent joins the caller's transaction: the student create
// flow and the batch endpoint both open theirs first, and a validation
// failure must roll back everything they wrote.
func (p *GuardianDirectoryProvider) AddGuardiansToStudent(ctx context.Context, studentID int64, guardians []peopledirectory.NewStudentGuardian) error {
	return mapGuardianError(p.inTenantTx(ctx, func(txCtx context.Context) error {
		return p.Guardians.AddGuardiansToStudent(txCtx, studentID, toNewStudentGuardians(guardians))
	}))
}

func (p *GuardianDirectoryProvider) UpdateGuardianLink(ctx context.Context, linkID int64, input peopledirectory.GuardianLinkUpdate) error {
	return mapGuardianLinkError(p.inTenantTx(ctx, func(txCtx context.Context) error {
		return p.Guardians.UpdateStudentGuardianRelationship(txCtx, linkID, peoplePorts.StudentGuardianUpdateRequest{
			RelationshipType: input.RelationshipType, GuardianRole: input.GuardianRole,
			IsPrimary: input.IsPrimary, IsEmergencyContact: input.IsEmergencyContact, CanPickup: input.CanPickup,
			PickupNotes: input.PickupNotes, EmergencyPriority: input.EmergencyPriority,
		})
	}))
}

func (p *GuardianDirectoryProvider) RemoveGuardianFromStudent(ctx context.Context, input peopledirectory.RemoveGuardian) error {
	return mapGuardianError(p.inTenantTx(ctx, func(txCtx context.Context) error {
		return p.Guardians.RemoveGuardianFromStudent(txCtx, input.StudentID, input.GuardianProfileID, input.ActorAccountID, input.MayClearPayer)
	}))
}

func (p *GuardianDirectoryProvider) RevealGuardianPayment(ctx context.Context, guardianID int64, actor peopledirectory.GuardianPaymentActor) (peopledirectory.GuardianPayment, error) {
	plain, err := p.Guardians.RevealGuardianPayment(ctx, guardianID, actor.AccountID, actor.Role)
	if err != nil {
		return peopledirectory.GuardianPayment{}, mapGuardianPaymentError(err)
	}
	return peopledirectory.GuardianPayment{GuardianProfileID: plain.GuardianProfileID, IBAN: plain.IBAN, AccountHolder: plain.AccountHolder}, nil
}

func (p *GuardianDirectoryProvider) UpdateGuardianPayment(ctx context.Context, guardianID int64, input peopledirectory.GuardianPaymentInput) error {
	return mapGuardianPaymentError(p.Guardians.UpdateGuardianPayment(ctx, guardianID, peoplePorts.GuardianPaymentInput{
		IBAN: input.IBAN, AccountHolder: input.AccountHolder,
	}, input.ActorAccountID, input.Note))
}

func (p *GuardianDirectoryProvider) SetStudentPayer(ctx context.Context, input peopledirectory.StudentPayer) error {
	return mapGuardianPaymentError(p.Guardians.SetStudentPayer(ctx, input.StudentID, input.GuardianProfileID, input.ActorAccountID))
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
	case errors.Is(err, users.ErrGuardianProfileNotFound):
		return fmt.Errorf("%w: %w", peopledirectory.ErrGuardianNotFound, err)
	case errors.Is(err, users.ErrStudentGuardianNotFound):
		return fmt.Errorf("%w: %w", peopledirectory.ErrGuardianLinkNotFound, err)
	case usersRepo.IsConstraintViolation(err): // a child link still references the profile
		return fmt.Errorf("%w: %w", peopledirectory.ErrGuardianLinkConflict, err)
	case strings.Contains(err.Error(), "not found"):
		// The legacy service reports missing rows through wrapped messages
		// ("guardian profile not found: ...", "student not found: ...").
		return fmt.Errorf("%w: %w", peopledirectory.ErrGuardianNotFound, err)
	default:
		return err
	}
}

func mapGuardianPhoneError(err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "phone number not found") {
		return fmt.Errorf("%w: %w", peopledirectory.ErrGuardianPhoneNotFound, err)
	}
	return mapGuardianError(err)
}

func mapGuardianLinkError(err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "relationship not found") {
		return fmt.Errorf("%w: %w", peopledirectory.ErrGuardianLinkNotFound, err)
	}
	return mapGuardianError(err)
}

func mapGuardianPaymentError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, sql.ErrNoRows), errors.Is(err, users.ErrGuardianProfileNotFound):
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

func toDirectoryGuardians(profiles []*users.GuardianProfile) []peopledirectory.Guardian {
	result := make([]peopledirectory.Guardian, 0, len(profiles))
	for _, profile := range profiles {
		if profile == nil {
			continue
		}
		result = append(result, toDirectoryGuardian(profile))
	}
	return result
}

func toDirectoryGuardian(profile *users.GuardianProfile) peopledirectory.Guardian {
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

func toDirectoryPhones(phones []*users.GuardianPhoneNumber) []peopledirectory.GuardianPhone {
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

func toDirectoryPhone(phone *users.GuardianPhoneNumber) peopledirectory.GuardianPhone {
	return peopledirectory.GuardianPhone{
		ID: phone.ID, GuardianProfileID: phone.GuardianProfileID, PhoneNumber: phone.PhoneNumber,
		PhoneType: string(phone.PhoneType), Label: phone.Label, IsPrimary: phone.IsPrimary, Priority: phone.Priority,
	}
}

func toDirectoryLink(link *users.StudentGuardian, granted func(any) bool) peopledirectory.GuardianLink {
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

func toDirectoryStudent(student *users.Student) peopledirectory.Student {
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
func grantedGuardianPermissions(link *users.StudentGuardian, granted func(any) bool) []string {
	if link == nil {
		return nil
	}
	_, _, _, _, _, permissions := link.GuardianAuthorizationData()
	names := make([]string, 0, len(permissions))
	for name, value := range permissions {
		if granted(value) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}
