package ports

import (
	"context"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// GuardianService manages guardian profiles, their student relationships,
// invitations, phone numbers and payment data. The application implements
// it over the retained repositories; the roots and the HTTP layer name it
// here (#3755, #4084).
type GuardianService interface {
	// GuardianProfiles is the retained guardian profile repository.
	GuardianProfiles() GuardianProfileRepository
	// GuardianPhoneNumbers is the retained phone number repository.
	GuardianPhoneNumbers() GuardianPhoneNumberRepository
	// GuardianInvitationReads is the bound invitation port.
	GuardianInvitationReads() GuardianInvitations
	AddGuardiansToStudent(ctx context.Context, studentID int64, guardians []NewStudentGuardian) error
	AddPhoneNumber(ctx context.Context, guardianID int64, req PhoneNumberCreateRequest) (*peopleValues.GuardianProfilePhoneSnapshot, error)
	CreateGuardian(ctx context.Context, req GuardianCreateRequest) (*peopleValues.GuardianProfileSnapshot, error)
	DeleteGuardian(ctx context.Context, id int64, changedByAccountID int64) error
	DeleteGuardianWithLinks(ctx context.Context, id int64, expectedLinkIDs []int64, changedByAccountID int64) error
	DeletePhoneNumber(ctx context.Context, phoneID int64) error
	EvaluateGuardianDelete(ctx context.Context, guardianProfileID int64, force bool, isAdmin bool) (hasLinks bool, err error)
	GetGuardianByID(ctx context.Context, id int64) (*peopleValues.GuardianProfileSnapshot, error)
	GetGuardianDeleteImpact(ctx context.Context, guardianProfileID int64) (*GuardianDeleteImpact, error)
	GetGuardianPaymentMasked(ctx context.Context, guardianProfileID int64, actorAccountID int64, actorRole string) (*GuardianPaymentMasked, error)
	GetGuardianStudents(ctx context.Context, guardianProfileID int64) ([]*StudentWithRelationship, error)
	GetStudentGuardianRelationship(ctx context.Context, relationshipID int64) (*peopleValues.GuardianRelationshipSnapshot, error)
	GetStudentGuardians(ctx context.Context, studentID int64) ([]*GuardianWithRelationship, error)
	GuardianDisplays(ctx context.Context, ids []int64) ([]GuardianDisplay, error)
	LinkGuardianToStudent(ctx context.Context, req StudentGuardianCreateRequest) (*peopleValues.GuardianRelationshipSnapshot, error)
	ListGuardians(ctx context.Context, options *QueryOptions) ([]*peopleValues.GuardianProfileSnapshot, error)
	ListPaymentExportRows(ctx context.Context, actorAccountID int64, actorRole string, format string) ([]GuardianPaymentRow, error)
	ListPaymentOverview(ctx context.Context, actorAccountID int64, actorRole string) ([]GuardianPaymentRow, error)
	RemoveGuardianFromStudent(ctx context.Context, studentID int64, guardianProfileID int64, changedByAccountID int64, mayClearPayer bool) error
	RevealGuardianPayment(ctx context.Context, guardianProfileID int64, actorAccountID int64, actorRole string) (*GuardianPaymentPlain, error)
	SearchGuardiansForPicker(ctx context.Context, searchText string, limit int) ([]*GuardianPickerMatch, error)
	SetPrimaryPhone(ctx context.Context, phoneID int64) error
	SetStudentPayer(ctx context.Context, studentID int64, guardianProfileID *int64, changedByAccountID int64) error
	UpdateGuardian(ctx context.Context, id int64, req GuardianCreateRequest) error
	UpdateGuardianPayment(ctx context.Context, guardianProfileID int64, input GuardianPaymentInput, changedByAccountID int64, note string) error
	UpdatePhoneNumber(ctx context.Context, phoneID int64, req PhoneNumberUpdateRequest) error
	UpdateStudentGuardianRelationship(ctx context.Context, relationshipID int64, req StudentGuardianUpdateRequest) error
	ValidateNewGuardians(ctx context.Context, guardians []NewStudentGuardian) error
}
