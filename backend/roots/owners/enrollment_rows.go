package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
)

type enrollmentPersonRows interface {
	FindByID(ctx context.Context, id any) (*peopleValues.DirectoryPersonSnapshot, error)
	FindByAccountID(ctx context.Context, accountID int64) (*peopleValues.DirectoryPersonSnapshot, error)
}

type enrollmentStaffRows interface {
	FindByPersonID(ctx context.Context, personID int64) (*peopleValues.Staff, error)
}

type enrollmentStudentRows interface {
	FindByID(ctx context.Context, id any) (*peopleValues.StudentState, error)
}

type enrollmentGuardianProfileRows interface {
	Create(ctx context.Context, profile *peopleValues.GuardianProfileSnapshot) error
	Update(ctx context.Context, profile *peopleValues.GuardianProfileSnapshot) error
	FindByAccountID(ctx context.Context, accountID int64) (*peopleValues.GuardianProfileSnapshot, error)
	FindByEmail(ctx context.Context, email string) (*peopleValues.GuardianProfileSnapshot, error)
	FindByEmails(ctx context.Context, emails []string) ([]*peopleValues.GuardianProfileSnapshot, error)
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*peopleValues.GuardianProfileSnapshot, error)
	FindLoginReadyPortalProfilesByIDs(ctx context.Context, ids []int64) (map[int64]*peopleValues.GuardianProfileSnapshot, error)
}

type enrollmentStudentGuardianRows interface {
	Create(ctx context.Context, relationship *peopleValues.GuardianRelationshipSnapshot) error
	Update(ctx context.Context, relationship *peopleValues.GuardianRelationshipSnapshot) error
	FindByStudentID(ctx context.Context, studentID int64) ([]*peopleValues.GuardianRelationshipSnapshot, error)
	FindByStudentAndGuardianForUpdate(ctx context.Context, studentID, guardianProfileID int64) (*peopleValues.GuardianRelationshipSnapshot, error)
	LinkIfNotExists(ctx context.Context, relationship *peopleValues.GuardianRelationshipSnapshot) (bool, error)
	UpdateColumns(ctx context.Context, relationship *peopleValues.GuardianRelationshipSnapshot, columns ...string) (int64, error)
}

type enrollmentGuardianPhoneRows interface {
	Create(ctx context.Context, phone *peopleValues.GuardianProfilePhoneSnapshot) error
	FindByGuardianID(ctx context.Context, guardianProfileID int64) ([]*peopleValues.GuardianProfilePhoneSnapshot, error)
	FindByGuardianIDs(ctx context.Context, guardianProfileIDs []int64) (map[int64][]*peopleValues.GuardianProfilePhoneSnapshot, error)
}

type enrollmentRosterStudentRows interface {
	ListClassRoster(ctx context.Context, schoolClass string) ([]*peopleValues.StudentState, error)
}

type enrollmentRosterPersonRows interface {
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*peopleValues.DirectoryPersonSnapshot, error)
}

type enrollmentGuardianContactRows interface {
	ListEmergencyContactRows(ctx context.Context, studentIDs []int64) ([]peopleValues.GuardianEmergencyContactRow, error)
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
	FindByID(ctx context.Context, id any) (*peopleValues.StudentState, error)
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*peopleValues.StudentState, error)
	FindByIDForUpdate(ctx context.Context, id int64) (*peopleValues.StudentState, error)
}

type offeringChangeWithdrawalRows interface {
	ListPending(ctx context.Context, filter careplan.WithdrawalListFilter) ([]*careplan.WithdrawalCompletion, int, error)
}
