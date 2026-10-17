package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/users"
)

// GuardianService manages guardian profiles, their student relationships,
// invitations, phone numbers and payment data. The application implements
// it over the retained repositories; the roots and the HTTP layer name it
// here (#3755, #4084).
type GuardianService interface {
	// GuardianProfiles is the retained guardian profile repository.
	GuardianProfiles() GuardianProfileRepository
	AddGuardiansToStudent(ctx context.Context, studentID int64, guardians []NewStudentGuardian) error
	AddPhoneNumber(ctx context.Context, guardianID int64, req PhoneNumberCreateRequest) (*users.GuardianPhoneNumber, error)
	CreateGuardian(ctx context.Context, req GuardianCreateRequest) (*users.GuardianProfile, error)
	DeleteGuardian(ctx context.Context, id int64, changedByAccountID int64) error
	DeleteGuardianWithLinks(ctx context.Context, id int64, expectedLinkIDs []int64, changedByAccountID int64) error
	DeletePhoneNumber(ctx context.Context, phoneID int64) error
	EvaluateGuardianDelete(ctx context.Context, guardianProfileID int64, force bool, isAdmin bool) (hasLinks bool, err error)
	GetGuardianByID(ctx context.Context, id int64) (*users.GuardianProfile, error)
	GetGuardianDeleteImpact(ctx context.Context, guardianProfileID int64) (*GuardianDeleteImpact, error)
	GetGuardianPaymentMasked(ctx context.Context, guardianProfileID int64, actorAccountID int64, actorRole string) (*GuardianPaymentMasked, error)
	GetGuardianPhoneNumbers(ctx context.Context, guardianID int64) ([]*users.GuardianPhoneNumber, error)
	GetGuardianStudents(ctx context.Context, guardianProfileID int64) ([]*StudentWithRelationship, error)
	GetGuardiansWithoutAccount(ctx context.Context) ([]*users.GuardianProfile, error)
	GetInvitableGuardians(ctx context.Context) ([]*users.GuardianProfile, error)
	GetPendingInvitations(ctx context.Context) ([]GuardianInvitationRecord, error)
	GetPhoneNumberByID(ctx context.Context, phoneID int64) (*users.GuardianPhoneNumber, error)
	GetStudentGuardianRelationship(ctx context.Context, relationshipID int64) (*users.StudentGuardian, error)
	GetStudentGuardians(ctx context.Context, studentID int64) ([]*GuardianWithRelationship, error)
	GuardianDisplays(ctx context.Context, ids []int64) ([]GuardianDisplay, error)
	LinkGuardianToStudent(ctx context.Context, req StudentGuardianCreateRequest) (*users.StudentGuardian, error)
	ListGuardians(ctx context.Context, options *QueryOptions) ([]*users.GuardianProfile, error)
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
