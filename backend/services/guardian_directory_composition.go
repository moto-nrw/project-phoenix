package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/transactionruntime"
)

// Guardian directory composition (#2663). The People Directory facade owns
// the guardian, phone-number and link capability; the application behind it
// is still the legacy guardian service of the same owner. The provider
// below binds that service onto the facade with the exact transaction and
// error semantics the api/guardians handlers used to apply themselves.

// GuardianProviderBinder is what the People Directory module exposes for the
// late binding of its guardian provider.
type GuardianProviderBinder interface {
	BindGuardianProvider(peopledirectory.GuardianProvider)
}

// bindGuardianDirectory installs the guardian provider on the People
// Directory. A capability without the binder cannot serve guardians at all,
// so the composition refuses to continue silently.
func (f *Factory) bindGuardianDirectory(persons peopledirectory.Capability, runtime peopleCompose.LegacyRepositoryRuntime) {
	binder, ok := persons.(GuardianProviderBinder)
	if !ok {
		panic(fmt.Sprintf("guardian directory: people directory %T cannot bind the guardian provider", persons))
	}
	binder.BindGuardianProvider(&GuardianDirectoryProvider{Guardians: f.Guardian, Runtime: runtime})
}

type GuardianDirectoryProvider struct {
	Guardians *peopleCompose.GuardianService
	Runtime   peopleCompose.LegacyRepositoryRuntime
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
	profile, err := p.Guardians.GuardianProfileRepo.FindByEmail(ctx, email)
	if err != nil {
		return peopledirectory.Guardian{}, mapGuardianError(err)
	}
	return toDirectoryGuardian(profile), nil
}

func (p *GuardianDirectoryProvider) ListGuardians(ctx context.Context, page, pageSize int) ([]peopledirectory.Guardian, error) {
	profiles, err := p.Guardians.ListGuardiansPage(ctx, page, pageSize)
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
			Link:              toDirectoryLink(row.Relationship),
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
			Link:    toDirectoryLink(row.Relationship),
		})
	}
	return result, nil
}

func (p *GuardianDirectoryProvider) FindGuardianLink(ctx context.Context, linkID int64) (peopledirectory.GuardianLink, error) {
	link, err := p.Guardians.GetStudentGuardianRelationship(ctx, linkID)
	if err != nil {
		return peopledirectory.GuardianLink{}, mapGuardianLinkError(err)
	}
	return toDirectoryLink(link), nil
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
	var profile *userModels.GuardianProfile
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
	var phone *userModels.GuardianPhoneNumber
	err := p.inTenantTx(ctx, func(txCtx context.Context) error {
		var txErr error
		phone, txErr = p.Guardians.AddPhoneNumber(txCtx, guardianID, peopleCompose.PhoneNumberCreateRequest{
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
		return p.Guardians.UpdatePhoneNumber(txCtx, phoneID, peopleCompose.PhoneNumberUpdateRequest{
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
	var link *userModels.StudentGuardian
	err := p.inTenantTx(ctx, func(txCtx context.Context) error {
		var txErr error
		link, txErr = p.Guardians.LinkGuardianToStudent(txCtx, peopleCompose.StudentGuardianCreateRequest{
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
		return p.Guardians.UpdateStudentGuardianRelationship(txCtx, linkID, peopleCompose.StudentGuardianUpdateRequest{
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
	return mapGuardianPaymentError(p.Guardians.UpdateGuardianPayment(ctx, guardianID, peopleCompose.GuardianPaymentInput{
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
	validation, isValidation := peopleCompose.ValidationFailure(err)
	var stillLinked *peopleCompose.GuardianStillLinkedError
	switch {
	case isValidation && errors.Is(err, peopleCompose.ErrGuardianEmailInUse):
		// Still invalid input for every caller; the sentinel adds the code.
		return fmt.Errorf("%w: %w", peopledirectory.ErrGuardianEmailTaken,
			&peopledirectory.InvalidGuardianError{Reason: validation.Error()})
	case isValidation:
		return &peopledirectory.InvalidGuardianError{Reason: validation.Error()}
	case errors.As(err, &stillLinked):
		return &peopledirectory.GuardianStillLinkedError{StudentNames: stillLinked.StudentNames}
	case errors.Is(err, peopleCompose.ErrGuardianForceDeleteRequiresAdmin):
		return peopledirectory.ErrGuardianForceDeleteRequiresAdmin
	case errors.Is(err, peopleCompose.ErrGuardianDeletePreviewChanged):
		return peopledirectory.ErrGuardianDeletePreviewChanged
	case errors.Is(err, peopleCompose.ErrPayerRemovalRequiresFinancial):
		return peopledirectory.ErrPayerRemovalRequiresFinancial
	case errors.Is(err, userModels.ErrGuardianProfileNotFound):
		return fmt.Errorf("%w: %w", peopledirectory.ErrGuardianNotFound, err)
	case errors.Is(err, userModels.ErrStudentGuardianNotFound):
		return fmt.Errorf("%w: %w", peopledirectory.ErrGuardianLinkNotFound, err)
	case peopleCompose.IsGuardianLinkConstraintViolation(err):
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
	case errors.Is(err, transactionruntime.ErrNoRows), errors.Is(err, userModels.ErrGuardianProfileNotFound):
		return fmt.Errorf("%w: %w", peopledirectory.ErrGuardianNotFound, err)
	case errors.Is(err, peopleCompose.ErrGuardianIBANInvalid):
		return peopledirectory.ErrGuardianIBANInvalid
	case errors.Is(err, peopleCompose.ErrGuardianAccountHolderTooLong):
		return peopledirectory.ErrGuardianAccountHolderTooLong
	case errors.Is(err, peopleCompose.ErrGuardianStudentRequired):
		return peopledirectory.ErrGuardianStudentRequired
	case errors.Is(err, peopleCompose.ErrGuardianNotLinkedToStudent):
		return peopledirectory.ErrGuardianNotLinkedToStudent
	case errors.Is(err, peopleCompose.ErrGuardianPaymentInvalid):
		return fmt.Errorf("%w: %w", peopledirectory.ErrGuardianPaymentInvalid, err)
	default:
		return err
	}
}

// --- projections ---

func toGuardianCreateRequest(input peopledirectory.GuardianInput) peopleCompose.GuardianCreateRequest {
	return peopleCompose.GuardianCreateRequest{
		FirstName: input.FirstName, LastName: input.LastName, Email: input.Email,
		AddressStreet: input.AddressStreet, AddressCity: input.AddressCity, AddressPostalCode: input.AddressPostalCode,
		PreferredContactMethod: input.PreferredContactMethod, LanguagePreference: input.LanguagePreference, Notes: input.Notes,
	}
}

func toNewStudentGuardians(inputs []peopledirectory.NewStudentGuardian) []peopleCompose.NewStudentGuardian {
	out := make([]peopleCompose.NewStudentGuardian, 0, len(inputs))
	for i := range inputs {
		in := inputs[i]
		out = append(out, peopleCompose.NewStudentGuardian{
			Profile: peopleCompose.GuardianCreateRequest{
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
			Relationship: peopleCompose.StudentGuardianRelationship{
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

func toNewGuardianPhones(phones []peopledirectory.NewGuardianPhone) []peopleCompose.PhoneNumberCreateRequest {
	if len(phones) == 0 {
		return nil
	}
	out := make([]peopleCompose.PhoneNumberCreateRequest, 0, len(phones))
	for i := range phones {
		out = append(out, peopleCompose.PhoneNumberCreateRequest{
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
		Permissions: grantedGuardianPermissions(link),
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

func toDirectoryPaymentRows(rows []peopleCompose.GuardianPaymentRow) []peopledirectory.GuardianPaymentRow {
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
	GuardianFailureConflict       GuardianFailureKind = "conflict"
	GuardianFailureForbidden      GuardianFailureKind = "forbidden"
	GuardianFailureInternal       GuardianFailureKind = "internal"
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
	Token             string
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

// The bulk invitation (#3378) passes Identity & Access' own contract through.
type (
	GuardianBulkInviteRequest = identityaccess.BulkInviteRequest
	GuardianBulkInviteResult  = identityaccess.BulkInviteResult
)

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

	InviteGuardianToStudent func(context.Context, GuardianInviteInput) (GuardianInviteResult, error)
	// BulkInviteGuardians invites the guardians of many children (#3378); a
	// failed run rolls the request's transaction back.
	BulkInviteGuardians        func(context.Context, GuardianBulkInviteRequest) (*GuardianBulkInviteResult, error)
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

// GuardianDirectoryInvitations is the Identity & Access half the guardian
// routes drive: the invitation lifecycle and the relative access flows.
type GuardianDirectoryInvitations interface {
	identityaccess.GuardianInvitations
	identityaccess.GuardianRelativeAccess
}

// NewGuardianDirectoryRuntime composes the closures over the service factory
// and the Identity & Access the root composed (#4006): invitations serves
// the invitation writes, verifiedStaff tells a verified staff caller.
// runtime is the People Directory's tenant runtime the invitation writes run
// in.
func (f *Factory) NewGuardianDirectoryRuntime(invitations GuardianDirectoryInvitations, verifiedStaff func(context.Context) (bool, error), runtime peopleCompose.LegacyRepositoryRuntime) GuardianDirectoryRuntime {
	return GuardianDirectoryRuntime{
		IsVerifiedStaff: func(ctx context.Context) bool {
			staff, err := verifiedStaff(ctx)
			return err == nil && staff
		},
		MarkRollback: runtime.MarkRollback,
		SendInvitation: func(ctx context.Context, guardianID, actorAccountID int64) (GuardianInvitationSummary, error) {
			var summary GuardianInvitationSummary
			err := runtime.InTenantTx(ctx, func(txCtx context.Context) error {
				invitation, err := invitations.CreateGuardianInvitation(txCtx, identityaccess.GuardianInvitationRequest{
					GuardianProfileID: guardianID, CreatedBy: actorAccountID,
				})
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
					ID: invitation.ID, GuardianProfileID: invitation.GuardianProfileID, Token: invitation.Token, CreatedAt: invitation.CreatedAt,
					ExpiresAt: invitation.ExpiresAt, EmailSentAt: invitation.EmailSentAt, EmailError: invitation.EmailError,
				})
			}
			return result, nil
		},
		InviteGuardianToStudent: func(ctx context.Context, input GuardianInviteInput) (GuardianInviteResult, error) {
			result, err := invitations.InviteToStudent(ctx, identityaccess.InviteToStudentRequest{
				StudentID: input.StudentID, Email: input.Email, FirstName: input.FirstName, LastName: input.LastName,
				RelationshipType: input.RelationshipType, CreatedBy: input.ActorAccountID,
				RequireApproval: false, ConfirmRoleUpgrade: input.ConfirmRoleUpgrade,
			})
			if err != nil {
				runtime.MarkRollback(ctx)
				return GuardianInviteResult{}, err
			}
			return GuardianInviteResult{
				Outcome: string(result.Outcome), GuardianProfileID: result.GuardianProfileID,
				InvitationID: result.InvitationID, ExistingRole: result.ExistingRole,
			}, nil
		},
		BulkInviteGuardians: func(ctx context.Context, req GuardianBulkInviteRequest) (*GuardianBulkInviteResult, error) {
			result, err := invitations.BulkInviteToStudents(ctx, req)
			if err != nil {
				runtime.MarkRollback(ctx)
			}
			return result, err
		},
		ListPendingApprovals: func(ctx context.Context) ([]GuardianPendingApproval, error) {
			views, err := invitations.ListPendingApprovalsDetailed(ctx)
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
		PendingInvitationStudentID: invitations.PendingInvitationStudentID,
		ApproveInvitation: func(ctx context.Context, invitationID, actorAccountID int64) error {
			if err := invitations.ApproveInvitation(ctx, invitationID, actorAccountID); err != nil {
				runtime.MarkRollback(ctx)
				return err
			}
			return nil
		},
		RejectInvitation: func(ctx context.Context, invitationID, actorAccountID int64) error {
			if err := invitations.RejectInvitation(ctx, invitationID, actorAccountID); err != nil {
				runtime.MarkRollback(ctx)
				return err
			}
			return nil
		},
		RenderPaymentExport: func(rows []peopledirectory.GuardianPaymentRow, format string) (GuardianExportFile, error) {
			if f.ListExport == nil {
				return GuardianExportFile{}, errors.New("list export service is not configured")
			}
			doc := lists.Document{
				Title:       "Bankverbindungen",
				Subtitle:    paymentExportSubtitle(rows),
				GeneratedAt: time.Now(),
				Footer:      exportConfidentialityNote,
				Columns: []lists.Column{
					{ID: lists.ColumnStudentName, Label: "Kind"},
					{ID: lists.ColumnStudentClass, Label: "Klasse"},
					{ID: lists.ColumnContactName, Label: "Kontoinhaber"},
					{ID: lists.ColumnIBAN, Label: "IBAN"},
				},
				Rows: buildPaymentExportRows(rows),
			}
			file, err := f.ListExport.Render(doc, lists.Format(format), doc.Title)
			if err != nil {
				return GuardianExportFile{}, err
			}
			return GuardianExportFile{ContentType: file.ContentType, Filename: file.Filename, Data: file.Data}, nil
		},
	}
}

// ClassifyGuardianInvitationFailure maps the invitation sentinels. Unknown
// failures come from persistence or delivery and must not be reported as bad
// client input.
func ClassifyGuardianInvitationFailure(err error) GuardianFailureKind {
	var validation *identityaccess.GuardianInvitationValidationError
	switch {
	case errors.Is(err, userModels.ErrGuardianAccountConflict):
		return GuardianFailureConflict
	case errors.As(err, &validation),
		errors.Is(err, identityaccess.ErrGuardianInvitationNotFound),
		errors.Is(err, identityaccess.ErrGuardianInvitationExpired):
		return GuardianFailureInvalidRequest
	case errors.Is(err, identityaccess.ErrInviteSocialWorkerManaged):
		return GuardianFailureForbidden
	default:
		return GuardianFailureInternal
	}
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
func buildPaymentExportRows(rows []peopledirectory.GuardianPaymentRow) []lists.Row {
	out := make([]lists.Row, 0, len(rows))
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
		out = append(out, lists.Row{
			Values: map[lists.ColumnID]string{
				lists.ColumnStudentName:  row.StudentName,
				lists.ColumnStudentClass: row.SchoolClass,
				lists.ColumnContactName:  holder,
				lists.ColumnIBAN:         iban,
			},
		})
	}
	return out
}
