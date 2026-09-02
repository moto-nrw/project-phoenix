package repositories

import (
	"context"
	"fmt"

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

// The three repositories decorated here are people-directory repositories that
// used to answer "is this account a colleague at this school?" or "which staff
// members are offboarded?" with a join into users.staff. School Membership owns
// those rows now, so the decorator resolves the membership through the owner
// and hands the concrete repository the resolved ids.

// bindStaffMembershipDecorators wires the decorators once, innermost, so the
// school/person/group wrappers bound afterwards keep wrapping them and a later
// BindSchoolMembership swap still reaches them (the capability is read lazily).
func (f *Factory) bindStaffMembershipDecorators() {
	capability := func() schoolmembership.Capability { return f.schoolMembership }
	persons := func() userModels.PersonRepository { return f.membershipDeps.persons }
	f.StaffDocument = staffDocumentMembershipRepository{
		StaffDocumentRepository: usersRepo.NewStaffDocumentRepository(f.db), membership: capability,
	}
	f.ParentMessageRead = parentMessageStaffRepository{
		ParentMessageReadRepository: usersRepo.NewParentMessageReadRepository(f.db), membership: capability, persons: persons,
	}
	f.StaffMessageRead = usersRepo.NewStaffMessageReadRepository(f.db, func(ctx context.Context) ([]int64, error) {
		return currentTenantStaffAccounts(ctx, capability(), persons())
	})
}

// staffAccountsByTenant maps every tenant visible in the caller's transaction
// to the login accounts of its live staff. Under a tenant transaction that is
// one school; under an admin transaction (the cross-tenant guardian inbox) it
// is every school the caller may see.
func staffAccountsByTenant(ctx context.Context, membership schoolmembership.Capability, persons userModels.PersonRepository) (map[int64][]int64, error) {
	members, err := membership.ListStaff(ctx, schoolmembership.StaffFilter{})
	if err != nil {
		return nil, fmt.Errorf("list staff for message projection: %w", err)
	}
	if len(members) == 0 {
		return map[int64][]int64{}, nil
	}
	personIDs := make([]int64, 0, len(members))
	for _, member := range members {
		personIDs = append(personIDs, member.PersonID)
	}
	rows, err := persons.FindByIDs(ctx, personIDs)
	if err != nil {
		return nil, fmt.Errorf("load staff persons for message projection: %w", err)
	}
	result := make(map[int64][]int64)
	seen := make(map[[2]int64]bool, len(members))
	for _, member := range members {
		person, found := rows[member.PersonID]
		if !found || person.AccountID == nil {
			continue
		}
		key := [2]int64{member.TenantID, *person.AccountID}
		if seen[key] {
			continue
		}
		seen[key] = true
		result[member.TenantID] = append(result[member.TenantID], *person.AccountID)
	}
	return result, nil
}

// --- staff documents ---

type staffDocumentMembershipRepository struct {
	*usersRepo.StaffDocumentRepository
	membership func() schoolmembership.Capability
}

var _ userModels.StaffDocumentRepository = staffDocumentMembershipRepository{}

// ListOffboardedPendingFileCleanups resolves the offboarded staff through the
// membership owner and asks the document repository for their pending file
// cleanups.
func (r staffDocumentMembershipRepository) ListOffboardedPendingFileCleanups(ctx context.Context) ([]*userModels.StaffDocument, error) {
	members, err := r.membership().ListStaff(ctx, schoolmembership.StaffFilter{IncludeDeleted: true})
	if err != nil {
		return nil, fmt.Errorf("list offboarded staff: %w", err)
	}
	offboarded := make([]int64, 0, len(members))
	for _, member := range members {
		if member.IsDeleted() {
			offboarded = append(offboarded, member.ID)
		}
	}
	return r.ListPendingFileCleanupsForStaff(ctx, offboarded)
}

// --- parent messaging ---

type parentMessageStaffRepository struct {
	*usersRepo.ParentMessageReadRepository
	membership func() schoolmembership.Capability
	persons    func() userModels.PersonRepository
}

var _ userModels.ParentMessageReadRepository = parentMessageStaffRepository{}

func (r parentMessageStaffRepository) staffAccounts(ctx context.Context) (map[int64][]int64, error) {
	return staffAccountsByTenant(ctx, r.membership(), r.persons())
}

func (r parentMessageStaffRepository) ListThreadsForGuardianStudent(ctx context.Context, accountID, studentID int64) ([]*userModels.InboxThread, error) {
	staffAccounts, err := r.staffAccounts(ctx)
	if err != nil {
		return nil, err
	}
	return r.ListThreadsForGuardianStudentWithStaff(ctx, accountID, studentID, staffAccounts)
}

func (r parentMessageStaffRepository) ListThreadsForGuardianTenants(ctx context.Context, accountID int64, tenantIDs []int64) ([]*userModels.InboxThread, error) {
	if len(tenantIDs) == 0 {
		return []*userModels.InboxThread{}, nil
	}
	staffAccounts, err := r.staffAccounts(ctx)
	if err != nil {
		return nil, err
	}
	wanted := make(map[int64]bool, len(tenantIDs))
	for _, tenantID := range tenantIDs {
		wanted[tenantID] = true
	}
	for tenantID := range staffAccounts {
		if !wanted[tenantID] {
			delete(staffAccounts, tenantID)
		}
	}
	return r.ListThreadsForGuardianTenantsWithStaff(ctx, accountID, tenantIDs, staffAccounts)
}

func (r parentMessageStaffRepository) LatestReadCursorByOther(ctx context.Context, threadID, excludeAccountID int64) (*userModels.ReadCursor, error) {
	staffAccounts, err := r.staffAccounts(ctx)
	if err != nil {
		return nil, err
	}
	return r.LatestReadCursorByOtherStaff(ctx, threadID, excludeAccountID, staffAccounts)
}

// currentTenantStaffAccounts is the flat account set of the live staff visible
// in the caller's transaction — the staff-messaging surface is tenant-scoped,
// so every account in the map belongs to the same school.
func currentTenantStaffAccounts(ctx context.Context, membership schoolmembership.Capability, persons userModels.PersonRepository) ([]int64, error) {
	byTenant, err := staffAccountsByTenant(ctx, membership, persons)
	if err != nil {
		return nil, err
	}
	accounts := make([]int64, 0, len(byTenant))
	for _, ids := range byTenant {
		accounts = append(accounts, ids...)
	}
	return accounts, nil
}
