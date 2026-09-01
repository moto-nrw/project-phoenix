package test

// Func-field mocks for platform.SchoolRepository and users.StaffRepository,
// following the configtest.Mock convention: exported XxxFn fields, each
// method delegates to its Fn field, and a nil field returns zero values.

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/models/users"
)

type FeedbackEntryCounterMock struct {
	Count int
	Err   error
}

func (m *FeedbackEntryCounterMock) CountForStudent(context.Context, int64) (int, error) {
	return m.Count, m.Err
}

// SchoolRepoMock is a func-field test double for platform.SchoolRepository.
type SchoolRepoMock struct {
	CreateFn                          func(ctx context.Context, school *platform.School) error
	FindByIDFn                        func(ctx context.Context, id int64) (*platform.School, error)
	FindByIDForShareFn                func(ctx context.Context, id int64) (*platform.School, error)
	FindByIDForUpdateFn               func(ctx context.Context, id int64) (*platform.School, error)
	FindBySlugFn                      func(ctx context.Context, slug string) (*platform.School, error)
	FindByOrganizationAndSlugFn       func(ctx context.Context, organizationID int64, slug string) (*platform.School, error)
	FindBySubdomainFn                 func(ctx context.Context, subdomain string) (*platform.School, error)
	ListFn                            func(ctx context.Context) ([]*platform.School, error)
	ListNonDeletedFn                  func(ctx context.Context) ([]platform.School, error)
	ListActiveFn                      func(ctx context.Context) ([]platform.School, error)
	ListPublicFn                      func(ctx context.Context) ([]platform.School, error)
	FindActiveByAccountIDFn           func(ctx context.Context, accountID int64) ([]platform.School, error)
	UpdateFn                          func(ctx context.Context, school *platform.School) error
	SoftDeleteFn                      func(ctx context.Context, id int64) error
	RestoreFn                         func(ctx context.Context, id int64) error
	CountByIDsFn                      func(ctx context.Context, ids []int64) (int, error)
	CountNonDeletedByOrganizationIDFn func(ctx context.Context, organizationID int64) (int, error)
}

var _ platform.SchoolRepository = (*SchoolRepoMock)(nil)

func (m *SchoolRepoMock) Create(ctx context.Context, school *platform.School) error {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, school)
	}
	return nil
}

func (m *SchoolRepoMock) FindByID(ctx context.Context, id int64) (*platform.School, error) {
	if m.FindByIDFn != nil {
		return m.FindByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *SchoolRepoMock) FindByIDForShare(ctx context.Context, id int64) (*platform.School, error) {
	if m.FindByIDForShareFn != nil {
		return m.FindByIDForShareFn(ctx, id)
	}
	return nil, nil
}

func (m *SchoolRepoMock) FindByIDForUpdate(ctx context.Context, id int64) (*platform.School, error) {
	if m.FindByIDForUpdateFn != nil {
		return m.FindByIDForUpdateFn(ctx, id)
	}
	return nil, nil
}

func (m *SchoolRepoMock) FindBySlug(ctx context.Context, slug string) (*platform.School, error) {
	if m.FindBySlugFn != nil {
		return m.FindBySlugFn(ctx, slug)
	}
	return nil, nil
}

func (m *SchoolRepoMock) FindByOrganizationAndSlug(ctx context.Context, organizationID int64, slug string) (*platform.School, error) {
	if m.FindByOrganizationAndSlugFn != nil {
		return m.FindByOrganizationAndSlugFn(ctx, organizationID, slug)
	}
	return nil, nil
}

func (m *SchoolRepoMock) FindBySubdomain(ctx context.Context, subdomain string) (*platform.School, error) {
	if m.FindBySubdomainFn != nil {
		return m.FindBySubdomainFn(ctx, subdomain)
	}
	return nil, nil
}

func (m *SchoolRepoMock) List(ctx context.Context) ([]*platform.School, error) {
	if m.ListFn != nil {
		return m.ListFn(ctx)
	}
	return nil, nil
}

func (m *SchoolRepoMock) ListNonDeleted(ctx context.Context) ([]platform.School, error) {
	if m.ListNonDeletedFn != nil {
		return m.ListNonDeletedFn(ctx)
	}
	return nil, nil
}

func (m *SchoolRepoMock) ListActive(ctx context.Context) ([]platform.School, error) {
	if m.ListActiveFn != nil {
		return m.ListActiveFn(ctx)
	}
	return nil, nil
}

func (m *SchoolRepoMock) ListPublic(ctx context.Context) ([]platform.School, error) {
	if m.ListPublicFn != nil {
		return m.ListPublicFn(ctx)
	}
	return nil, nil
}

func (m *SchoolRepoMock) FindActiveByAccountID(ctx context.Context, accountID int64) ([]platform.School, error) {
	if m.FindActiveByAccountIDFn != nil {
		return m.FindActiveByAccountIDFn(ctx, accountID)
	}
	return nil, nil
}

func (m *SchoolRepoMock) Update(ctx context.Context, school *platform.School) error {
	if m.UpdateFn != nil {
		return m.UpdateFn(ctx, school)
	}
	return nil
}

func (m *SchoolRepoMock) SoftDelete(ctx context.Context, id int64) error {
	if m.SoftDeleteFn != nil {
		return m.SoftDeleteFn(ctx, id)
	}
	return nil
}

func (m *SchoolRepoMock) Restore(ctx context.Context, id int64) error {
	if m.RestoreFn != nil {
		return m.RestoreFn(ctx, id)
	}
	return nil
}

func (m *SchoolRepoMock) CountByIDs(ctx context.Context, ids []int64) (int, error) {
	if m.CountByIDsFn != nil {
		return m.CountByIDsFn(ctx, ids)
	}
	return 0, nil
}

func (m *SchoolRepoMock) CountNonDeletedByOrganizationID(ctx context.Context, organizationID int64) (int, error) {
	if m.CountNonDeletedByOrganizationIDFn != nil {
		return m.CountNonDeletedByOrganizationIDFn(ctx, organizationID)
	}
	return 0, nil
}

// StaffRepoMock is a func-field test double for users.StaffRepository.
type StaffRepoMock struct {
	CreateFn                        func(ctx context.Context, entity *users.Staff) error
	FindByIDFn                      func(ctx context.Context, id any) (*users.Staff, error)
	FindByIDForUpdateFn             func(ctx context.Context, id int64) (*users.Staff, error)
	UpdateFn                        func(ctx context.Context, entity *users.Staff) error
	DeleteFn                        func(ctx context.Context, id any) error
	ListFn                          func(ctx context.Context, filters map[string]any) ([]*users.Staff, error)
	FindByPersonIDFn                func(ctx context.Context, personID int64) (*users.Staff, error)
	ListAllWithPersonFn             func(ctx context.Context) ([]*users.Staff, error)
	ClearWorkTimeModelFn            func(ctx context.Context, id int64) error
	FindWithPersonFn                func(ctx context.Context, id int64) (*users.Staff, error)
	FindByIDsFn                     func(ctx context.Context, ids []int64) (map[int64]*users.Staff, error)
	FindWithPersonByIDsFn           func(ctx context.Context, ids []int64) (map[int64]*users.Staff, error)
	ListStaffByRolesFn              func(ctx context.Context, roles []string) ([]*users.StaffWithRoleInfo, error)
	ListStaffWithPermissionFn       func(ctx context.Context, permissionName string) ([]*users.StaffWithRoleInfo, error)
	GetStaffContactInfoFn           func(ctx context.Context, staffID int64) (*users.StaffWithRoleInfo, error)
	FindReachableCalendarStaffIDsFn func(ctx context.Context, ids []int64) (map[int64]bool, error)
	ListAccountIDsByStaffIDsFn      func(ctx context.Context, staffIDs []int64) (map[int64]int64, error)
	ListAllStaffAccountIDsFn        func(ctx context.Context) (map[int64]int64, error)
	FindBirthdaysOnFn               func(ctx context.Context, days []users.MonthDay) ([]users.BirthdayEntry, error)
	ListBirthdaysForExportFn        func(ctx context.Context) ([]users.BirthdayEntry, error)
	SetBirthdayDisplayOptOutFn      func(ctx context.Context, staffID int64, optOut bool) error
}

var _ users.StaffRepository = (*StaffRepoMock)(nil)

func (m *StaffRepoMock) FindBirthdaysOn(ctx context.Context, days []users.MonthDay) ([]users.BirthdayEntry, error) {
	if m.FindBirthdaysOnFn != nil {
		return m.FindBirthdaysOnFn(ctx, days)
	}
	return nil, nil
}

func (m *StaffRepoMock) ListBirthdaysForExport(ctx context.Context) ([]users.BirthdayEntry, error) {
	if m.ListBirthdaysForExportFn != nil {
		return m.ListBirthdaysForExportFn(ctx)
	}
	return nil, nil
}

func (m *StaffRepoMock) SetBirthdayDisplayOptOut(ctx context.Context, staffID int64, optOut bool) error {
	if m.SetBirthdayDisplayOptOutFn != nil {
		return m.SetBirthdayDisplayOptOutFn(ctx, staffID, optOut)
	}
	return nil
}

func (m *StaffRepoMock) Create(ctx context.Context, entity *users.Staff) error {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, entity)
	}
	return nil
}

func (m *StaffRepoMock) FindByID(ctx context.Context, id any) (*users.Staff, error) {
	if m.FindByIDFn != nil {
		return m.FindByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *StaffRepoMock) FindByIDForUpdate(ctx context.Context, id int64) (*users.Staff, error) {
	if m.FindByIDForUpdateFn != nil {
		return m.FindByIDForUpdateFn(ctx, id)
	}
	return m.FindByID(ctx, id)
}

func (m *StaffRepoMock) Update(ctx context.Context, entity *users.Staff) error {
	if m.UpdateFn != nil {
		return m.UpdateFn(ctx, entity)
	}
	return nil
}

func (m *StaffRepoMock) Delete(ctx context.Context, id any) error {
	if m.DeleteFn != nil {
		return m.DeleteFn(ctx, id)
	}
	return nil
}

func (m *StaffRepoMock) List(ctx context.Context, filters map[string]any) ([]*users.Staff, error) {
	if m.ListFn != nil {
		return m.ListFn(ctx, filters)
	}
	return nil, nil
}

func (m *StaffRepoMock) FindByPersonID(ctx context.Context, personID int64) (*users.Staff, error) {
	if m.FindByPersonIDFn != nil {
		return m.FindByPersonIDFn(ctx, personID)
	}
	return nil, nil
}

func (m *StaffRepoMock) ListAllWithPerson(ctx context.Context) ([]*users.Staff, error) {
	if m.ListAllWithPersonFn != nil {
		return m.ListAllWithPersonFn(ctx)
	}
	return nil, nil
}

func (m *StaffRepoMock) ClearWorkTimeModel(ctx context.Context, id int64) error {
	if m.ClearWorkTimeModelFn != nil {
		return m.ClearWorkTimeModelFn(ctx, id)
	}
	return nil
}

func (m *StaffRepoMock) FindWithPerson(ctx context.Context, id int64) (*users.Staff, error) {
	if m.FindWithPersonFn != nil {
		return m.FindWithPersonFn(ctx, id)
	}
	return nil, nil
}

func (m *StaffRepoMock) FindByIDs(ctx context.Context, ids []int64) (map[int64]*users.Staff, error) {
	if m.FindByIDsFn != nil {
		return m.FindByIDsFn(ctx, ids)
	}
	return nil, nil
}

func (m *StaffRepoMock) FindWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*users.Staff, error) {
	if m.FindWithPersonByIDsFn != nil {
		return m.FindWithPersonByIDsFn(ctx, ids)
	}
	return nil, nil
}

func (m *StaffRepoMock) GetStaffContactInfo(ctx context.Context, staffID int64) (*users.StaffWithRoleInfo, error) {
	if m.GetStaffContactInfoFn != nil {
		return m.GetStaffContactInfoFn(ctx, staffID)
	}
	return nil, nil
}

func (m *StaffRepoMock) ListAccountIDsByStaffIDs(ctx context.Context, staffIDs []int64) (map[int64]int64, error) {
	if m.ListAccountIDsByStaffIDsFn != nil {
		return m.ListAccountIDsByStaffIDsFn(ctx, staffIDs)
	}
	return map[int64]int64{}, nil
}

func (m *StaffRepoMock) ListAllStaffAccountIDs(ctx context.Context) (map[int64]int64, error) {
	if m.ListAllStaffAccountIDsFn != nil {
		return m.ListAllStaffAccountIDsFn(ctx)
	}
	return map[int64]int64{}, nil
}

func (m *StaffRepoMock) ListStaffWithPermission(ctx context.Context, permissionName string) ([]*users.StaffWithRoleInfo, error) {
	if m.ListStaffWithPermissionFn != nil {
		return m.ListStaffWithPermissionFn(ctx, permissionName)
	}
	return nil, nil
}

func (m *StaffRepoMock) ListStaffByRoles(ctx context.Context, roles []string) ([]*users.StaffWithRoleInfo, error) {
	if m.ListStaffByRolesFn != nil {
		return m.ListStaffByRolesFn(ctx, roles)
	}
	return nil, nil
}

func (m *StaffRepoMock) FindReachableCalendarStaffIDs(ctx context.Context, ids []int64) (map[int64]bool, error) {
	if m.FindReachableCalendarStaffIDsFn != nil {
		return m.FindReachableCalendarStaffIDsFn(ctx, ids)
	}
	return map[int64]bool{}, nil
}
