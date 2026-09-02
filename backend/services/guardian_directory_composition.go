package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Guardian directory composition (#2663). The People Directory facade owns
// the guardian, phone-number and link capability; the application behind it
// is still the legacy guardian service of the same owner. The provider
// below binds that service onto the facade with the exact transaction and
// error semantics the api/guardians handlers used to apply themselves.

// guardianProviderBinder is what the People Directory module exposes for the
// late binding of its guardian provider.
type guardianProviderBinder interface {
	BindGuardianProvider(peopledirectory.GuardianProvider)
}

// bindGuardianDirectory installs the guardian provider on the People
// Directory. A capability without the binder cannot serve guardians at all,
// so the composition refuses to continue silently.
func (f *Factory) bindGuardianDirectory(persons peopledirectory.Capability, db *bun.DB) {
	binder, ok := persons.(guardianProviderBinder)
	if !ok {
		panic(fmt.Sprintf("guardian directory: people directory %T cannot bind the guardian provider", persons))
	}
	binder.BindGuardianProvider(&guardianDirectoryProvider{guardians: f.Guardian, db: db})
}

type guardianDirectoryProvider struct {
	guardians *usersSvc.GuardianService
	db        *bun.DB
}

// inTenantTx runs fn in the tenant transaction of the context, joining the
// request transaction when one is open. Every write of the old handlers ran
// this way, so a failure rolls the whole request back.
func (p *guardianDirectoryProvider) inTenantTx(ctx context.Context, fn func(context.Context) error) error {
	return tenant.WithTenantTx(ctx, p.db, tenant.FromContext(ctx), func(txCtx context.Context, _ bun.Tx) error {
		return fn(txCtx)
	})
}

func (p *guardianDirectoryProvider) FindGuardian(ctx context.Context, id int64) (peopledirectory.Guardian, error) {
	profile, err := p.guardians.GetGuardianByID(ctx, id)
	if err != nil {
		return peopledirectory.Guardian{}, mapGuardianError(err)
	}
	return toDirectoryGuardian(profile), nil
}

func (p *guardianDirectoryProvider) ListGuardians(ctx context.Context, page, pageSize int) ([]peopledirectory.Guardian, error) {
	profiles, err := p.guardians.ListGuardiansPage(ctx, page, pageSize)
	if err != nil {
		return nil, mapGuardianError(err)
	}
	return toDirectoryGuardians(profiles), nil
}

func (p *guardianDirectoryProvider) ListGuardiansWithoutAccount(ctx context.Context) ([]peopledirectory.Guardian, error) {
	profiles, err := p.guardians.GetGuardiansWithoutAccount(ctx)
	if err != nil {
		return nil, mapGuardianError(err)
	}
	return toDirectoryGuardians(profiles), nil
}

func (p *guardianDirectoryProvider) ListInvitableGuardians(ctx context.Context) ([]peopledirectory.Guardian, error) {
	profiles, err := p.guardians.GetInvitableGuardians(ctx)
	if err != nil {
		return nil, mapGuardianError(err)
	}
	return toDirectoryGuardians(profiles), nil
}

func (p *guardianDirectoryProvider) SearchGuardians(ctx context.Context, text string, limit int) ([]peopledirectory.GuardianMatch, error) {
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

func (p *guardianDirectoryProvider) GuardianDeleteImpact(ctx context.Context, id int64) (peopledirectory.GuardianDeleteImpact, error) {
	impact, err := p.guardians.GetGuardianDeleteImpact(ctx, id)
	if err != nil {
		return peopledirectory.GuardianDeleteImpact{}, mapGuardianError(err)
	}
	return peopledirectory.GuardianDeleteImpact{LinkIDs: impact.LinkIDs, StudentNames: impact.StudentNames}, nil
}

func (p *guardianDirectoryProvider) ListGuardianPhones(ctx context.Context, guardianID int64) ([]peopledirectory.GuardianPhone, error) {
	phones, err := p.guardians.GetGuardianPhoneNumbers(ctx, guardianID)
	if err != nil {
		return nil, mapGuardianError(err)
	}
	return toDirectoryPhones(phones), nil
}

func (p *guardianDirectoryProvider) FindGuardianPhone(ctx context.Context, phoneID int64) (peopledirectory.GuardianPhone, error) {
	phone, err := p.guardians.GetPhoneNumberByID(ctx, phoneID)
	if err != nil {
		return peopledirectory.GuardianPhone{}, mapGuardianPhoneError(err)
	}
	return toDirectoryPhone(phone), nil
}

func (p *guardianDirectoryProvider) ListStudentGuardians(ctx context.Context, studentID int64) ([]peopledirectory.GuardianWithLink, error) {
	rows, err := p.guardians.GetStudentGuardians(ctx, studentID)
	if err != nil {
		return nil, mapGuardianError(err)
	}
	result := make([]peopledirectory.GuardianWithLink, 0, len(rows))
	for _, row := range rows {
		result = append(result, peopledirectory.GuardianWithLink{
			Guardian:          toDirectoryGuardian(row.Profile),
			Link:              toDirectoryLink(row.Relationship),
			InvitationPending: row.InvitationPending,
		})
	}
	return result, nil
}

func (p *guardianDirectoryProvider) ListGuardianStudents(ctx context.Context, guardianID int64) ([]peopledirectory.StudentWithLink, error) {
	rows, err := p.guardians.GetGuardianStudents(ctx, guardianID)
	if err != nil {
		return nil, mapGuardianError(err)
	}
	result := make([]peopledirectory.StudentWithLink, 0, len(rows))
	for _, row := range rows {
		result = append(result, peopledirectory.StudentWithLink{
			Student: toDirectoryStudent(row.Student),
			Link:    toDirectoryLink(row.Relationship),
		})
	}
	return result, nil
}

func (p *guardianDirectoryProvider) FindGuardianLink(ctx context.Context, linkID int64) (peopledirectory.GuardianLink, error) {
	link, err := p.guardians.GetStudentGuardianRelationship(ctx, linkID)
	if err != nil {
		return peopledirectory.GuardianLink{}, mapGuardianLinkError(err)
	}
	return toDirectoryLink(link), nil
}

func (p *guardianDirectoryProvider) GuardianPaymentMasked(ctx context.Context, guardianID int64, actor peopledirectory.GuardianPaymentActor) (peopledirectory.GuardianPayment, error) {
	masked, err := p.guardians.GetGuardianPaymentMasked(ctx, guardianID, actor.AccountID, actor.Role)
	if err != nil {
		return peopledirectory.GuardianPayment{}, mapGuardianPaymentError(err)
	}
	return peopledirectory.GuardianPayment{GuardianProfileID: masked.GuardianProfileID, IBAN: masked.IBANMasked, AccountHolder: masked.AccountHolder}, nil
}

func (p *guardianDirectoryProvider) ListPaymentOverview(ctx context.Context, actor peopledirectory.GuardianPaymentActor) ([]peopledirectory.GuardianPaymentRow, error) {
	rows, err := p.guardians.ListPaymentOverview(ctx, actor.AccountID, actor.Role)
	if err != nil {
		return nil, mapGuardianPaymentError(err)
	}
	return toDirectoryPaymentRows(rows), nil
}

func (p *guardianDirectoryProvider) ListPaymentExportRows(ctx context.Context, actor peopledirectory.GuardianPaymentActor, format string) ([]peopledirectory.GuardianPaymentRow, error) {
	rows, err := p.guardians.ListPaymentExportRows(ctx, actor.AccountID, actor.Role, format)
	if err != nil {
		return nil, mapGuardianPaymentError(err)
	}
	return toDirectoryPaymentRows(rows), nil
}

func (p *guardianDirectoryProvider) CreateGuardian(ctx context.Context, input peopledirectory.GuardianInput) (peopledirectory.Guardian, error) {
	var profile *userModels.GuardianProfile
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

func (p *guardianDirectoryProvider) UpdateGuardian(ctx context.Context, id int64, input peopledirectory.GuardianInput) error {
	return mapGuardianError(p.inTenantTx(ctx, func(txCtx context.Context) error {
		return p.guardians.UpdateGuardian(txCtx, id, toGuardianCreateRequest(input))
	}))
}

func (p *guardianDirectoryProvider) EvaluateGuardianDelete(ctx context.Context, id int64, force, isAdmin bool) (bool, error) {
	hasLinks, err := p.guardians.EvaluateGuardianDelete(ctx, id, force, isAdmin)
	return hasLinks, mapGuardianError(err)
}

func (p *guardianDirectoryProvider) DeleteGuardian(ctx context.Context, input peopledirectory.GuardianDelete) error {
	return mapGuardianError(p.inTenantTx(ctx, func(txCtx context.Context) error {
		if input.WithLinks {
			return p.guardians.DeleteGuardianWithLinks(txCtx, input.GuardianID, input.ExpectedLinkIDs, input.ActorAccountID)
		}
		return p.guardians.DeleteGuardian(txCtx, input.GuardianID, input.ActorAccountID)
	}))
}

func (p *guardianDirectoryProvider) AddGuardianPhone(ctx context.Context, guardianID int64, input peopledirectory.GuardianPhoneInput) (peopledirectory.GuardianPhone, error) {
	var phone *userModels.GuardianPhoneNumber
	err := p.inTenantTx(ctx, func(txCtx context.Context) error {
		var txErr error
		phone, txErr = p.guardians.AddPhoneNumber(txCtx, guardianID, usersSvc.PhoneNumberCreateRequest{
			PhoneNumber: input.PhoneNumber, PhoneType: input.PhoneType, Label: input.Label, IsPrimary: input.IsPrimary,
		})
		return txErr
	})
	if err != nil {
		return peopledirectory.GuardianPhone{}, mapGuardianError(err)
	}
	return toDirectoryPhone(phone), nil
}

func (p *guardianDirectoryProvider) UpdateGuardianPhone(ctx context.Context, phoneID int64, input peopledirectory.GuardianPhoneUpdate) error {
	return mapGuardianPhoneError(p.inTenantTx(ctx, func(txCtx context.Context) error {
		return p.guardians.UpdatePhoneNumber(txCtx, phoneID, usersSvc.PhoneNumberUpdateRequest{
			PhoneNumber: input.PhoneNumber, PhoneType: input.PhoneType, Label: input.Label, IsPrimary: input.IsPrimary, Priority: input.Priority,
		})
	}))
}

func (p *guardianDirectoryProvider) DeleteGuardianPhone(ctx context.Context, phoneID int64) error {
	return mapGuardianPhoneError(p.inTenantTx(ctx, func(txCtx context.Context) error {
		return p.guardians.DeletePhoneNumber(txCtx, phoneID)
	}))
}

func (p *guardianDirectoryProvider) SetPrimaryGuardianPhone(ctx context.Context, phoneID int64) error {
	return mapGuardianPhoneError(p.inTenantTx(ctx, func(txCtx context.Context) error {
		return p.guardians.SetPrimaryPhone(txCtx, phoneID)
	}))
}

func (p *guardianDirectoryProvider) LinkGuardianToStudent(ctx context.Context, input peopledirectory.LinkGuardian) (peopledirectory.GuardianLink, error) {
	var link *userModels.StudentGuardian
	err := p.inTenantTx(ctx, func(txCtx context.Context) error {
		var txErr error
		link, txErr = p.guardians.LinkGuardianToStudent(txCtx, usersSvc.StudentGuardianCreateRequest{
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
	return toDirectoryLink(link), nil
}

func (p *guardianDirectoryProvider) ValidateNewGuardians(ctx context.Context, guardians []peopledirectory.NewStudentGuardian) error {
	return mapGuardianError(p.guardians.ValidateNewGuardians(ctx, toNewStudentGuardians(guardians)))
}

// AddGuardiansToStudent joins the caller's transaction: the student create
// flow and the batch endpoint both open theirs first, and a validation
// failure must roll back everything they wrote.
func (p *guardianDirectoryProvider) AddGuardiansToStudent(ctx context.Context, studentID int64, guardians []peopledirectory.NewStudentGuardian) error {
	return mapGuardianError(p.inTenantTx(ctx, func(txCtx context.Context) error {
		return p.guardians.AddGuardiansToStudent(txCtx, studentID, toNewStudentGuardians(guardians))
	}))
}

func (p *guardianDirectoryProvider) UpdateGuardianLink(ctx context.Context, linkID int64, input peopledirectory.GuardianLinkUpdate) error {
	return mapGuardianLinkError(p.inTenantTx(ctx, func(txCtx context.Context) error {
		return p.guardians.UpdateStudentGuardianRelationship(txCtx, linkID, usersSvc.StudentGuardianUpdateRequest{
			RelationshipType: input.RelationshipType, GuardianRole: input.GuardianRole,
			IsPrimary: input.IsPrimary, IsEmergencyContact: input.IsEmergencyContact, CanPickup: input.CanPickup,
			PickupNotes: input.PickupNotes, EmergencyPriority: input.EmergencyPriority,
		})
	}))
}

func (p *guardianDirectoryProvider) RemoveGuardianFromStudent(ctx context.Context, input peopledirectory.RemoveGuardian) error {
	return mapGuardianError(p.inTenantTx(ctx, func(txCtx context.Context) error {
		return p.guardians.RemoveGuardianFromStudent(txCtx, input.StudentID, input.GuardianProfileID, input.ActorAccountID, input.MayClearPayer)
	}))
}

func (p *guardianDirectoryProvider) RevealGuardianPayment(ctx context.Context, guardianID int64, actor peopledirectory.GuardianPaymentActor) (peopledirectory.GuardianPayment, error) {
	plain, err := p.guardians.RevealGuardianPayment(ctx, guardianID, actor.AccountID, actor.Role)
	if err != nil {
		return peopledirectory.GuardianPayment{}, mapGuardianPaymentError(err)
	}
	return peopledirectory.GuardianPayment{GuardianProfileID: plain.GuardianProfileID, IBAN: plain.IBAN, AccountHolder: plain.AccountHolder}, nil
}

func (p *guardianDirectoryProvider) UpdateGuardianPayment(ctx context.Context, guardianID int64, input peopledirectory.GuardianPaymentInput) error {
	return mapGuardianPaymentError(p.guardians.UpdateGuardianPayment(ctx, guardianID, usersSvc.GuardianPaymentInput{
		IBAN: input.IBAN, AccountHolder: input.AccountHolder,
	}, input.ActorAccountID, input.Note))
}

func (p *guardianDirectoryProvider) SetStudentPayer(ctx context.Context, input peopledirectory.StudentPayer) error {
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
	var validation *usersSvc.ValidationError
	var stillLinked *usersSvc.GuardianStillLinkedError
	switch {
	case errors.As(err, &validation):
		return &peopledirectory.InvalidGuardianError{Reason: validation.Error()}
	case errors.As(err, &stillLinked):
		return &peopledirectory.GuardianStillLinkedError{StudentNames: stillLinked.StudentNames}
	case errors.Is(err, usersSvc.ErrGuardianForceDeleteRequiresAdmin):
		return peopledirectory.ErrGuardianForceDeleteRequiresAdmin
	case errors.Is(err, usersSvc.ErrGuardianDeletePreviewChanged):
		return peopledirectory.ErrGuardianDeletePreviewChanged
	case errors.Is(err, usersSvc.ErrPayerRemovalRequiresFinancial):
		return peopledirectory.ErrPayerRemovalRequiresFinancial
	case errors.Is(err, userModels.ErrGuardianProfileNotFound):
		return fmt.Errorf("%w: %w", peopledirectory.ErrGuardianNotFound, err)
	case errors.Is(err, userModels.ErrStudentGuardianNotFound):
		return fmt.Errorf("%w: %w", peopledirectory.ErrGuardianLinkNotFound, err)
	case errors.Is(err, usersSvc.ErrStudentNotFound):
		return fmt.Errorf("%w: %w", peopledirectory.ErrStudentNotFound, err)
	case usersSvc.IsGuardianLinkConstraintViolation(err):
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
	case errors.Is(err, sql.ErrNoRows), errors.Is(err, userModels.ErrGuardianProfileNotFound):
		return fmt.Errorf("%w: %w", peopledirectory.ErrGuardianNotFound, err)
	case errors.Is(err, usersSvc.ErrStudentNotFound):
		return fmt.Errorf("%w: %w", peopledirectory.ErrStudentNotFound, err)
	case errors.Is(err, usersSvc.ErrGuardianIBANInvalid):
		return peopledirectory.ErrGuardianIBANInvalid
	case errors.Is(err, usersSvc.ErrGuardianAccountHolderTooLong):
		return peopledirectory.ErrGuardianAccountHolderTooLong
	case errors.Is(err, usersSvc.ErrGuardianStudentRequired):
		return peopledirectory.ErrGuardianStudentRequired
	case errors.Is(err, usersSvc.ErrGuardianNotLinkedToStudent):
		return peopledirectory.ErrGuardianNotLinkedToStudent
	case errors.Is(err, usersSvc.ErrGuardianPaymentInvalid):
		return fmt.Errorf("%w: %w", peopledirectory.ErrGuardianPaymentInvalid, err)
	default:
		return err
	}
}

// --- projections ---

func toGuardianCreateRequest(input peopledirectory.GuardianInput) usersSvc.GuardianCreateRequest {
	return usersSvc.GuardianCreateRequest{
		FirstName: input.FirstName, LastName: input.LastName, Email: input.Email,
		AddressStreet: input.AddressStreet, AddressCity: input.AddressCity, AddressPostalCode: input.AddressPostalCode,
		PreferredContactMethod: input.PreferredContactMethod, LanguagePreference: input.LanguagePreference, Notes: input.Notes,
	}
}

func toNewStudentGuardians(inputs []peopledirectory.NewStudentGuardian) []usersSvc.NewStudentGuardian {
	out := make([]usersSvc.NewStudentGuardian, 0, len(inputs))
	for i := range inputs {
		in := inputs[i]
		out = append(out, usersSvc.NewStudentGuardian{
			Profile: usersSvc.GuardianCreateRequest{
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
			Relationship: usersSvc.StudentGuardianRelationship{
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

func toNewGuardianPhones(phones []peopledirectory.NewGuardianPhone) []usersSvc.PhoneNumberCreateRequest {
	if len(phones) == 0 {
		return nil
	}
	out := make([]usersSvc.PhoneNumberCreateRequest, 0, len(phones))
	for i := range phones {
		out = append(out, usersSvc.PhoneNumberCreateRequest{
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

func toDirectoryGuardians(profiles []*userModels.GuardianProfile) []peopledirectory.Guardian {
	result := make([]peopledirectory.Guardian, 0, len(profiles))
	for _, profile := range profiles {
		if profile == nil {
			continue
		}
		result = append(result, toDirectoryGuardian(profile))
	}
	return result
}

func toDirectoryGuardian(profile *userModels.GuardianProfile) peopledirectory.Guardian {
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

func toDirectoryPhones(phones []*userModels.GuardianPhoneNumber) []peopledirectory.GuardianPhone {
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

func toDirectoryPhone(phone *userModels.GuardianPhoneNumber) peopledirectory.GuardianPhone {
	return peopledirectory.GuardianPhone{
		ID: phone.ID, GuardianProfileID: phone.GuardianProfileID, PhoneNumber: phone.PhoneNumber,
		PhoneType: string(phone.PhoneType), Label: phone.Label, IsPrimary: phone.IsPrimary, Priority: phone.Priority,
	}
}

func toDirectoryLink(link *userModels.StudentGuardian) peopledirectory.GuardianLink {
	if link == nil {
		return peopledirectory.GuardianLink{}
	}
	return peopledirectory.GuardianLink{
		ID: link.ID, TenantID: link.GetTenantID(), StudentID: link.StudentID, GuardianProfileID: link.GuardianProfileID,
		RelationshipType: link.RelationshipType, GuardianRole: link.GuardianRole,
		IsPrimary: link.IsPrimary, IsEmergencyContact: link.IsEmergencyContact, CanPickup: link.CanPickup,
		PickupNotes: link.PickupNotes, EmergencyPriority: link.EmergencyPriority, IsPayer: link.IsPayer,
		Permissions: usersSvc.GrantedGuardianPermissions(link),
	}
}

func toDirectoryStudent(student *userModels.Student) peopledirectory.Student {
	if student == nil {
		return peopledirectory.Student{}
	}
	return peopledirectory.Student{
		ID: student.ID, CreatedAt: student.CreatedAt, UpdatedAt: student.UpdatedAt, TenantID: student.GetTenantID(),
		PersonID: student.PersonID, SchoolClass: student.SchoolClass, GroupID: student.GroupID, Status: string(student.Status),
	}
}

func toDirectoryPaymentRows(rows []usersSvc.GuardianPaymentRow) []peopledirectory.GuardianPaymentRow {
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

// --- runtime for the HTTP adapter ---

// GuardianFailureKind classifies an invitation failure for the HTTP layer.
type GuardianFailureKind string

const (
	GuardianFailureInvalidRequest GuardianFailureKind = "invalid_request"
	GuardianFailureForbidden      GuardianFailureKind = "forbidden"
)

// GuardianInvitationSummary is the staff-initiated invitation the deprecated
// per-guardian invite endpoint reports; Token is only exposed to the seeder.
type GuardianInvitationSummary struct {
	ID                int64
	GuardianProfileID int64
	ExpiresAt         time.Time
	EmailSent         bool
	Token             string
}

// PendingGuardianInvitation is one open invitation of the tenant.
type PendingGuardianInvitation struct {
	ID                int64
	GuardianProfileID int64
	CreatedAt         time.Time
	ExpiresAt         time.Time
	EmailSentAt       *time.Time
	EmailError        *string
	EmailRetryCount   int
}

// GuardianInviteInput invites a further guardian to a child by e-mail.
type GuardianInviteInput struct {
	StudentID          int64
	Email              string
	FirstName          string
	LastName           string
	RelationshipType   string
	ActorAccountID     int64
	ConfirmRoleUpgrade bool
}

// GuardianInviteResult echoes what the invite resolved to.
type GuardianInviteResult struct {
	Outcome           string
	GuardianProfileID int64
	InvitationID      *int64
	ExistingRole      string
}

// GuardianPendingApproval is one parent-initiated request awaiting staff.
type GuardianPendingApproval struct {
	InvitationID      int64
	GuardianProfileID int64
	GuardianName      string
	GuardianEmail     string
	StudentID         int64
	StudentName       string
	RequestedByEmail  string
	CreatedAt         time.Time
	ExpiresAt         time.Time
	RoleUpgrade       bool
}

// GuardianExportFile is the rendered Bankverbindungen document.
type GuardianExportFile struct {
	ContentType string
	Filename    string
	Data        []byte
}

// GuardianDirectoryRuntime is the legacy-service side of the guardian HTTP
// adapter: the identity, invitation and document flows that are not part
// of the People Directory capability. Every closure keeps the semantics of
// the handler code it replaced in api/guardians.
type GuardianDirectoryRuntime struct {
	// IsVerifiedStaff reports whether the caller has a staff record; the
	// legacy handlers treated any lookup failure as "not staff".
	IsVerifiedStaff func(context.Context) bool
	MarkRollback    func(context.Context)

	SendInvitation         func(ctx context.Context, guardianID, actorAccountID int64) (GuardianInvitationSummary, error)
	ListPendingInvitations func(context.Context) ([]PendingGuardianInvitation, error)

	InviteGuardianToStudent    func(context.Context, GuardianInviteInput) (GuardianInviteResult, error)
	ListPendingApprovals       func(context.Context) ([]GuardianPendingApproval, error)
	PendingInvitationStudentID func(context.Context, int64) (int64, error)
	ApproveInvitation          func(ctx context.Context, invitationID, actorAccountID int64) error
	RejectInvitation           func(ctx context.Context, invitationID, actorAccountID int64) error

	// RenderPaymentExport renders the Bankverbindungen list; format is one
	// of pdf, docx or xlsx, validated by the adapter beforehand.
	RenderPaymentExport func(rows []peopledirectory.GuardianPaymentRow, format string) (GuardianExportFile, error)
}

// exportConfidentialityNote is stamped on every page of the exported list.
const exportConfidentialityNote = "Vertraulich. Enthält Bankverbindungen. Bitte nicht per E-Mail weitergeben."

// NewGuardianDirectoryRuntime composes the closures over the service factory.
func (f *Factory) NewGuardianDirectoryRuntime(db *bun.DB) GuardianDirectoryRuntime {
	return GuardianDirectoryRuntime{
		IsVerifiedStaff: func(ctx context.Context) bool {
			staff, err := f.UserContext.GetCurrentStaff(ctx)
			return err == nil && staff != nil
		},
		MarkRollback: tenant.MarkRollback,
		SendInvitation: func(ctx context.Context, guardianID, actorAccountID int64) (GuardianInvitationSummary, error) {
			var summary GuardianInvitationSummary
			err := tenant.WithTenantTx(ctx, db, tenant.FromContext(ctx), func(txCtx context.Context, _ bun.Tx) error {
				invitation, err := f.Guardian.SendInvitation(txCtx, usersSvc.GuardianInvitationRequest{GuardianProfileID: guardianID, CreatedBy: actorAccountID}) //nolint:staticcheck // deprecated twin stays until audit A-13 deletes the whole flow
				if err != nil {
					return err
				}
				summary = GuardianInvitationSummary{
					ID: invitation.ID, GuardianProfileID: invitation.GuardianProfileID, ExpiresAt: invitation.ExpiresAt,
					EmailSent: invitation.EmailSentAt != nil, Token: invitation.Token,
				}
				return nil
			})
			return summary, err
		},
		ListPendingInvitations: func(ctx context.Context) ([]PendingGuardianInvitation, error) {
			invitations, err := f.Guardian.GetPendingInvitations(ctx)
			if err != nil {
				return nil, err
			}
			result := make([]PendingGuardianInvitation, 0, len(invitations))
			for _, invitation := range invitations {
				result = append(result, PendingGuardianInvitation{
					ID: invitation.ID, GuardianProfileID: invitation.GuardianProfileID, CreatedAt: invitation.CreatedAt,
					ExpiresAt: invitation.ExpiresAt, EmailSentAt: invitation.EmailSentAt, EmailError: invitation.EmailError,
					EmailRetryCount: invitation.EmailRetryCount,
				})
			}
			return result, nil
		},
		InviteGuardianToStudent: func(ctx context.Context, input GuardianInviteInput) (GuardianInviteResult, error) {
			result, err := f.GuardianInvitation.InviteToStudent(ctx, authSvc.InviteToStudentRequest{
				StudentID: input.StudentID, Email: input.Email, FirstName: input.FirstName, LastName: input.LastName,
				RelationshipType: input.RelationshipType, CreatedBy: input.ActorAccountID,
				RequireApproval: false, ConfirmRoleUpgrade: input.ConfirmRoleUpgrade,
			})
			if err != nil {
				tenant.MarkRollback(ctx)
				return GuardianInviteResult{}, err
			}
			return GuardianInviteResult{
				Outcome: string(result.Outcome), GuardianProfileID: result.GuardianProfileID,
				InvitationID: result.InvitationID, ExistingRole: result.ExistingRole,
			}, nil
		},
		ListPendingApprovals: func(ctx context.Context) ([]GuardianPendingApproval, error) {
			views, err := f.GuardianInvitation.ListPendingApprovalsDetailed(ctx)
			if err != nil {
				return nil, err
			}
			result := make([]GuardianPendingApproval, 0, len(views))
			for _, view := range views {
				result = append(result, GuardianPendingApproval{
					InvitationID: view.InvitationID, GuardianProfileID: view.GuardianProfileID,
					GuardianName: view.GuardianName, GuardianEmail: view.GuardianEmail,
					StudentID: view.StudentID, StudentName: view.StudentName, RequestedByEmail: view.RequestedByEmail,
					CreatedAt: view.CreatedAt, ExpiresAt: view.ExpiresAt, RoleUpgrade: view.RoleUpgrade,
				})
			}
			return result, nil
		},
		PendingInvitationStudentID: f.GuardianInvitation.PendingInvitationStudentID,
		ApproveInvitation: func(ctx context.Context, invitationID, actorAccountID int64) error {
			if err := f.GuardianInvitation.ApproveInvitation(ctx, invitationID, actorAccountID); err != nil {
				tenant.MarkRollback(ctx)
				return err
			}
			return nil
		},
		RejectInvitation: func(ctx context.Context, invitationID, actorAccountID int64) error {
			if err := f.GuardianInvitation.RejectInvitation(ctx, invitationID, actorAccountID); err != nil {
				tenant.MarkRollback(ctx)
				return err
			}
			return nil
		},
		RenderPaymentExport: func(rows []peopledirectory.GuardianPaymentRow, format string) (GuardianExportFile, error) {
			if f.ListExport == nil {
				return GuardianExportFile{}, errors.New("list export service is not configured")
			}
			doc := listexport.Document{
				Title:       "Bankverbindungen",
				Subtitle:    paymentExportSubtitle(rows),
				GeneratedAt: time.Now(),
				Footer:      exportConfidentialityNote,
				Columns: []listexport.Column{
					{ID: listexport.ColumnStudentName, Label: "Kind"},
					{ID: listexport.ColumnStudentClass, Label: "Klasse"},
					{ID: listexport.ColumnContactName, Label: "Kontoinhaber"},
					{ID: listexport.ColumnIBAN, Label: "IBAN"},
				},
				Rows: buildPaymentExportRows(rows),
			}
			file, err := f.ListExport.Render(doc, listexport.Format(format), doc.Title)
			if err != nil {
				return GuardianExportFile{}, err
			}
			return GuardianExportFile{ContentType: file.ContentType, Filename: file.Filename, Data: file.Data}, nil
		},
	}
}

// ClassifyGuardianInvitationFailure maps the invitation sentinels: a
// school-managed social worker contact is forbidden, everything else is bad
// input.
func ClassifyGuardianInvitationFailure(err error) GuardianFailureKind {
	if errors.Is(err, authSvc.ErrInviteSocialWorkerManaged) {
		return GuardianFailureForbidden
	}
	return GuardianFailureInvalidRequest
}

// paymentExportSubtitle states how complete the list is. A bank list that
// silently omits its gaps reads as finished when it is not.
func paymentExportSubtitle(rows []peopledirectory.GuardianPaymentRow) string {
	withIBAN := 0
	for _, row := range rows {
		if row.HasIBAN() {
			withIBAN++
		}
	}
	if withIBAN == len(rows) {
		return fmt.Sprintf("%d Kinder, alle mit Bankverbindung", len(rows))
	}
	return fmt.Sprintf("%d Kinder, davon %d mit Bankverbindung", len(rows), withIBAN)
}

// buildPaymentExportRows renders one line per child. Children without a payer
// or without an IBAN keep their row and say what is missing, so the list can
// be used as a to-do rather than read as complete.
func buildPaymentExportRows(rows []peopledirectory.GuardianPaymentRow) []listexport.Row {
	out := make([]listexport.Row, 0, len(rows))
	for _, row := range rows {
		holder := row.AccountHolder
		iban := row.IBAN
		switch {
		case row.GuardianProfileID == nil:
			holder = "Nicht zugeordnet"
			iban = "Fehlt"
		case iban == "":
			iban = "Fehlt"
		}
		out = append(out, listexport.Row{
			Values: map[listexport.ColumnID]string{
				listexport.ColumnStudentName:  row.StudentName,
				listexport.ColumnStudentClass: row.SchoolClass,
				listexport.ColumnContactName:  holder,
				listexport.ColumnIBAN:         iban,
			},
		})
	}
	return out
}
