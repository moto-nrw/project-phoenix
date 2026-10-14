package api

import (
	"context"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/compose/vocabulary"
	platformModels "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose/vocabulary"
	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	educationRepo "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
)

// The rows the root's Identity & Access seams read and write, declared where
// they are used (Rule 1, #4006): the identity chain of the People Directory
// and School Membership, the groups the caller context names, the
// authentication and operator ledgers, and Delivery's push subscriptions.
// The legacy composition hands in its retained stores; each port names only
// the operations a seam calls.

type personRows interface {
	Create(ctx context.Context, person *userModels.Person) error
	FindByID(ctx context.Context, id any) (*userModels.Person, error)
	Update(ctx context.Context, person *userModels.Person) error
	List(ctx context.Context, filters map[string]any) ([]*userModels.Person, error)
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*userModels.Person, error)
	FindByTagID(ctx context.Context, tagID string) (*userModels.Person, error)
	FindByAccountID(ctx context.Context, accountID int64) (*userModels.Person, error)
	FindByAccountIDs(ctx context.Context, accountIDs []int64) (map[int64]*userModels.Person, error)
	LinkToAccount(ctx context.Context, personID int64, accountID int64) error
	LinkToRFIDCard(ctx context.Context, personID int64, tagID string) error
}

type staffRows interface {
	Create(ctx context.Context, staff *userModels.Staff) error
	FindByID(ctx context.Context, id any) (*userModels.Staff, error)
	FindByPersonID(ctx context.Context, personID int64) (*userModels.Staff, error)
}

type teacherRows interface {
	Create(ctx context.Context, teacher *userModels.Teacher) error
	FindByID(ctx context.Context, id any) (*userModels.Teacher, error)
	FindByStaffID(ctx context.Context, staffID int64) (*userModels.Teacher, error)
}

type studentRows interface {
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*userModels.Student, error)
	FindByPersonID(ctx context.Context, personID int64) (*userModels.Student, error)
	FindByIDForUpdate(ctx context.Context, id int64) (*userModels.Student, error)
}

type guardianProfileRows interface {
	Create(ctx context.Context, profile *userModels.GuardianProfile) error
	Delete(ctx context.Context, id int64) error
	FindByID(ctx context.Context, id int64) (*userModels.GuardianProfile, error)
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*userModels.GuardianProfile, error)
	FindByEmail(ctx context.Context, email string) (*userModels.GuardianProfile, error)
	LinkAccount(ctx context.Context, profileID int64, accountID int64) error
}

type studentGuardianRows interface {
	FindByID(ctx context.Context, id any) (*userModels.StudentGuardian, error)
	Update(ctx context.Context, relationship *userModels.StudentGuardian) error
	Delete(ctx context.Context, id any) error
	FindByStudentIDs(ctx context.Context, studentIDs []int64) ([]*userModels.StudentGuardian, error)
	FindByGuardianProfileID(ctx context.Context, guardianProfileID int64) ([]*userModels.StudentGuardian, error)
	FindByStudentAndGuardianForUpdate(ctx context.Context, studentID, guardianProfileID int64) (*userModels.StudentGuardian, error)
	LinkIfNotExists(ctx context.Context, relationship *userModels.StudentGuardian) (bool, error)
}

type groupRows interface {
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*educationRepo.Group, error)
}

type authEventLedger interface {
	Create(ctx context.Context, event *auditModels.AuthEvent) error
	CreateStaffPreviewEndOnce(ctx context.Context, event *auditModels.AuthEvent) (bool, error)
	StaffPreviewEnded(ctx context.Context, accountID int64, previewID string) (bool, error)
	LockStaffPreview(ctx context.Context, accountID int64, previewID string) error
	ListPendingAccountWideWipes(ctx context.Context, since time.Time) ([]auditModels.PendingAccountWideWipe, error)
	ClaimPendingAccountWideWipes(ctx context.Context, accountID int64) ([]auditModels.PendingAccountWideWipe, error)
}

type operatorAuditEntries interface {
	Create(ctx context.Context, entry *platformModels.OperatorAuditLog) error
}

type pushSubscriptionRows interface {
	DeleteStaffByAccountID(ctx context.Context, accountID int64) error
	DeleteParentByAccountID(ctx context.Context, accountID int64) error
	DeleteSchoolByAccountID(ctx context.Context, accountID int64) error
	DeleteByTokenFamilyID(ctx context.Context, accountID int64, familyID string) error
	DeleteStaffUnboundByAccount(ctx context.Context, accountID, tenantID int64) error
	DeleteParentUnboundByAccount(ctx context.Context, accountID, tenantID int64) error
	DeleteSchoolUnboundByAccount(ctx context.Context, accountID, tenantID int64) error
	DeleteOrphanedSubscriptions(ctx context.Context) error
}
