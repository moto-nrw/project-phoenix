package api

import (
	"context"
	"time"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"

	educationRepo "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
)

// The rows the root's Identity & Access seams read and write, declared where
// they are used (Rule 1, #4006): the identity chain of the People Directory
// and School Membership, the groups the caller context names, the
// authentication and operator ledgers, and Delivery's push subscriptions.
// The service graph hands in its retained stores; each port names only the
// operations a seam calls.

type personRows interface {
	FindByID(ctx context.Context, id any) (*peopleValues.DirectoryPersonSnapshot, error)
	List(ctx context.Context, filters map[string]any) ([]*peopleValues.DirectoryPersonSnapshot, error)
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*peopleValues.DirectoryPersonSnapshot, error)
	FindByTagID(ctx context.Context, tagID string) (*peopleValues.DirectoryPersonSnapshot, error)
	FindByAccountID(ctx context.Context, accountID int64) (*peopleValues.DirectoryPersonSnapshot, error)
	FindByAccountIDs(ctx context.Context, accountIDs []int64) (map[int64]*peopleValues.DirectoryPersonSnapshot, error)
}

type staffRows interface {
	FindByID(ctx context.Context, id any) (*peopleValues.Staff, error)
	FindByPersonID(ctx context.Context, personID int64) (*peopleValues.Staff, error)
}

type teacherRows interface {
	FindByID(ctx context.Context, id any) (*peopleValues.Teacher, error)
	FindByStaffID(ctx context.Context, staffID int64) (*peopleValues.Teacher, error)
}

type studentRows interface {
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*peopleValues.StudentState, error)
	FindByPersonID(ctx context.Context, personID int64) (*peopleValues.StudentState, error)
	FindByIDForUpdate(ctx context.Context, id int64) (*peopleValues.StudentState, error)
}

type guardianProfileRows interface {
	FindByID(ctx context.Context, id int64) (*peopleValues.GuardianProfileSnapshot, error)
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*peopleValues.GuardianProfileSnapshot, error)
	FindByEmail(ctx context.Context, email string) (*peopleValues.GuardianProfileSnapshot, error)
}

type studentGuardianRows interface {
	FindByStudentIDs(ctx context.Context, studentIDs []int64) ([]*peopleValues.GuardianRelationshipSnapshot, error)
	FindByGuardianProfileID(ctx context.Context, guardianProfileID int64) ([]*peopleValues.GuardianRelationshipSnapshot, error)
	FindByStudentAndGuardianForUpdate(ctx context.Context, studentID, guardianProfileID int64) (*peopleValues.GuardianRelationshipSnapshot, error)
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
	Create(ctx context.Context, entry *auditModels.OperatorAuditEntry) error
}

type pushSubscriptionRows interface {
	DeleteStaffByAccountID(ctx context.Context, accountID int64) error
	DeleteParentByAccountID(ctx context.Context, accountID int64) error
	DeleteSchoolByAccountID(ctx context.Context, accountID int64) error
	DeleteByTokenFamilyID(ctx context.Context, accountID int64, familyID string) error
	DeleteUnboundByAccountForPortal(ctx context.Context, accountID, tenantID int64, portal string) error
	DeleteOrphanedSubscriptions(ctx context.Context) error
}
