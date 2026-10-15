package api

import (
	"context"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/compose/vocabulary"
	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
)

// The rows the root's Enrollment bindings read and write, declared where they
// are used (Rule 1, #4012): the People Directory rows and the Audit Platform
// trails the service graph hands in. Each port names only the
// operations a binding calls.

type enrollmentPersonRows interface {
	Create(ctx context.Context, person *userModels.Person) error
	FindByID(ctx context.Context, id any) (*userModels.Person, error)
	Update(ctx context.Context, person *userModels.Person) error
	FindByAccountID(ctx context.Context, accountID int64) (*userModels.Person, error)
}

type enrollmentStaffRows interface {
	FindByPersonID(ctx context.Context, personID int64) (*userModels.Staff, error)
}

type enrollmentStudentRows interface {
	FindByID(ctx context.Context, id any) (*userModels.Student, error)
}

type enrollmentGuardianProfileRows interface {
	Create(ctx context.Context, profile *userModels.GuardianProfile) error
	Update(ctx context.Context, profile *userModels.GuardianProfile) error
	FindByAccountID(ctx context.Context, accountID int64) (*userModels.GuardianProfile, error)
	FindByEmail(ctx context.Context, email string) (*userModels.GuardianProfile, error)
	FindByEmails(ctx context.Context, emails []string) ([]*userModels.GuardianProfile, error)
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*userModels.GuardianProfile, error)
	FindLoginReadyPortalProfilesByIDs(ctx context.Context, ids []int64) (map[int64]*userModels.GuardianProfile, error)
	LinkAccount(ctx context.Context, profileID int64, accountID int64) error
}

type enrollmentStudentGuardianRows interface {
	Create(ctx context.Context, relationship *userModels.StudentGuardian) error
	Update(ctx context.Context, relationship *userModels.StudentGuardian) error
	Delete(ctx context.Context, id any) error
	FindByStudentID(ctx context.Context, studentID int64) ([]*userModels.StudentGuardian, error)
	FindByStudentAndGuardianForUpdate(ctx context.Context, studentID, guardianProfileID int64) (*userModels.StudentGuardian, error)
	LinkIfNotExists(ctx context.Context, relationship *userModels.StudentGuardian) (bool, error)
	UpdateColumns(ctx context.Context, relationship *userModels.StudentGuardian, columns ...string) (int64, error)
}

type enrollmentGuardianPhoneRows interface {
	Create(ctx context.Context, phone *userModels.GuardianPhoneNumber) error
	FindByGuardianID(ctx context.Context, guardianProfileID int64) ([]*userModels.GuardianPhoneNumber, error)
	FindByGuardianIDs(ctx context.Context, guardianProfileIDs []int64) (map[int64][]*userModels.GuardianPhoneNumber, error)
}

type enrollmentRosterStudentRows interface {
	ListClassRoster(ctx context.Context, schoolClass string) ([]*userModels.Student, error)
}

type enrollmentRosterPersonRows interface {
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*userModels.Person, error)
}

type enrollmentGuardianContactRows interface {
	ListEmergencyContactRows(ctx context.Context, studentIDs []int64) ([]userModels.GuardianEmergencyContactRow, error)
}

type enrollmentAccessLogAppender interface {
	Create(ctx context.Context, entry *auditModels.DataAccessLog) error
}

type enrollmentAdjustmentRows interface {
	ListByRequestChildID(ctx context.Context, requestChildID int64) ([]*auditModels.EnrollmentOfferingAdjustment, error)
}

type enrollmentRestorationAppender interface {
	Create(ctx context.Context, event *auditModels.EnrollmentRestoration) error
}

type enrollmentDeletionAppender interface {
	Create(ctx context.Context, event *auditModels.EnrollmentDeletion) error
}

type offeringChangeStudentRows interface {
	FindByID(ctx context.Context, id any) (*userModels.Student, error)
	FindByIDForUpdate(ctx context.Context, id int64) (*userModels.Student, error)
}

type offeringChangeWithdrawalRows interface {
	ListPending(ctx context.Context, filter userModels.CareWithdrawalCompletionFilter) ([]*userModels.CareWithdrawalCompletion, int, error)
}
