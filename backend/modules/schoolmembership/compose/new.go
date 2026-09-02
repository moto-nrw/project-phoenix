// Package compose wires the School Membership module over the shared tenant
// runtime and the Bun database.
package compose

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type Observation = ports.Observation

type Dependencies struct {
	DB      *bun.DB
	Observe func(Observation)
}

// New composes the School Membership module. Every operation runs on the
// caller's ambient tenant transaction when one exists and otherwise opens
// one for the tenant in context, so row-level security decides visibility
// exactly as it did for the legacy repositories.
func New(dependencies Dependencies) (*schoolmembership.Module, error) {
	if dependencies.DB == nil || dependencies.Observe == nil {
		return nil, errors.New("school membership compose: all dependencies are required")
	}
	store := postgres.New(func(ctx context.Context) (bun.IDB, int64, error) {
		transaction, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return nil, 0, errors.New("school membership postgres: transaction is required")
		}
		tx, ok := transaction.(bun.Tx)
		if !ok {
			return nil, 0, fmt.Errorf("school membership postgres: unsupported transaction %T", transaction)
		}
		return tx, tenant.FromContext(ctx), nil
	})
	service := application.New(store, transaction{}, func(observation Observation) {
		observation.Err = mapError(observation.Err)
		dependencies.Observe(observation)
	})
	return schoolmembership.NewModule(engine{service: service}), nil
}

type transaction struct{}

func (transaction) RunWrite(ctx context.Context, callback func(context.Context) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return callback(ctx)
	}
	return tenant.WithinCurrentTenant(ctx, callback)
}

func (transaction) RunRead(ctx context.Context, callback func(context.Context) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return callback(ctx)
	}
	if _, err := tenant.TenantFromContext(ctx); err == nil {
		return tenant.WithinCurrentTenant(ctx, callback)
	}
	return tenant.WithinAdmin(ctx, callback)
}

type engine struct{ service *application.Service }

func (e engine) FindStaff(ctx context.Context, id int64, lock string) (schoolmembership.Staff, error) {
	value, err := e.service.FindStaff(ctx, id, lock)
	return staffToPublic(value), mapError(err)
}

func (e engine) FindStaffByPerson(ctx context.Context, personID int64) (schoolmembership.Staff, error) {
	value, err := e.service.FindStaffByPerson(ctx, personID)
	return staffToPublic(value), mapError(err)
}

func (e engine) ListStaff(ctx context.Context, filter schoolmembership.StaffFilter) ([]schoolmembership.Staff, error) {
	values, err := e.service.ListStaff(ctx, domain.StaffFilter{
		IDs: filter.IDs, PersonIDs: filter.PersonIDs, WorkTimeModelID: filter.WorkTimeModelID, IncludeDeleted: filter.IncludeDeleted,
	})
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]schoolmembership.Staff, 0, len(values))
	for _, value := range values {
		result = append(result, staffToPublic(value))
	}
	return result, nil
}

func (e engine) CreateStaff(ctx context.Context, input schoolmembership.CreateStaff) (schoolmembership.Staff, error) {
	value, err := e.service.CreateStaff(ctx, staffFieldsToDomain(input.StaffFields))
	return staffToPublic(value), mapError(err)
}

func (e engine) UpdateStaff(ctx context.Context, input schoolmembership.UpdateStaff) (schoolmembership.Staff, error) {
	value, err := e.service.UpdateStaff(ctx, input.ID, staffFieldsToDomain(input.StaffFields))
	return staffToPublic(value), mapError(err)
}

func (e engine) DeleteStaff(ctx context.Context, id int64) error {
	return mapError(e.service.DeleteStaff(ctx, id))
}

func (e engine) ClearWorkTimeModel(ctx context.Context, id int64) error {
	return mapError(e.service.ClearWorkTimeModel(ctx, id))
}

func (e engine) AppendStaffNotes(ctx context.Context, id int64, notes string) (schoolmembership.Staff, error) {
	value, err := e.service.AppendStaffNotes(ctx, id, notes)
	return staffToPublic(value), mapError(err)
}

func (e engine) SetBirthdayDisplayOptOut(ctx context.Context, id int64, optOut bool) error {
	return mapError(e.service.SetBirthdayDisplayOptOut(ctx, id, optOut))
}

func (e engine) RebaseWorkTimeModelAnchor(ctx context.Context, workTimeModelID int64, anchorDate string) ([]int64, error) {
	ids, err := e.service.RebaseWorkTimeModelAnchor(ctx, workTimeModelID, anchorDate)
	return ids, mapError(err)
}

func (e engine) FindTeacher(ctx context.Context, id int64) (schoolmembership.Teacher, error) {
	value, err := e.service.FindTeacher(ctx, id)
	return teacherToPublic(value), mapError(err)
}

func (e engine) FindTeacherByStaff(ctx context.Context, staffID int64) (schoolmembership.Teacher, error) {
	value, err := e.service.FindTeacherByStaff(ctx, staffID)
	return teacherToPublic(value), mapError(err)
}

func (e engine) ListTeachers(ctx context.Context, filter schoolmembership.TeacherFilter) ([]schoolmembership.Teacher, error) {
	values, err := e.service.ListTeachers(ctx, domain.TeacherFilter{
		IDs: filter.IDs, StaffIDs: filter.StaffIDs, Specialization: filter.Specialization,
		SpecializationContains: filter.SpecializationContains, RoleContains: filter.RoleContains,
		HasQualifications: filter.HasQualifications, IncludeDeleted: filter.IncludeDeleted,
	})
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]schoolmembership.Teacher, 0, len(values))
	for _, value := range values {
		result = append(result, teacherToPublic(value))
	}
	return result, nil
}

func (e engine) CreateTeacher(ctx context.Context, input schoolmembership.CreateTeacher) (schoolmembership.Teacher, error) {
	value, err := e.service.CreateTeacher(ctx, teacherFieldsToDomain(input.TeacherFields))
	return teacherToPublic(value), mapError(err)
}

func (e engine) UpdateTeacher(ctx context.Context, input schoolmembership.UpdateTeacher) (schoolmembership.Teacher, error) {
	value, err := e.service.UpdateTeacher(ctx, input.ID, teacherFieldsToDomain(input.TeacherFields))
	return teacherToPublic(value), mapError(err)
}

func (e engine) DeleteTeacher(ctx context.Context, id int64) error {
	return mapError(e.service.DeleteTeacher(ctx, id))
}

func (e engine) FindGuest(ctx context.Context, id int64) (schoolmembership.Guest, error) {
	value, err := e.service.FindGuest(ctx, id)
	return guestToPublic(value), mapError(err)
}

func (e engine) FindGuestByStaff(ctx context.Context, staffID int64) (schoolmembership.Guest, error) {
	value, err := e.service.FindGuestByStaff(ctx, staffID)
	return guestToPublic(value), mapError(err)
}

func (e engine) ListGuests(ctx context.Context, filter schoolmembership.GuestFilter) ([]schoolmembership.Guest, error) {
	values, err := e.service.ListGuests(ctx, domain.GuestFilter{
		IDs: filter.IDs, StaffIDs: filter.StaffIDs, ActiveOn: filter.ActiveOn,
		OrganizationContains: filter.OrganizationContains, ExpertiseContains: filter.ExpertiseContains,
		HasOrganization: filter.HasOrganization,
	})
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]schoolmembership.Guest, 0, len(values))
	for _, value := range values {
		result = append(result, guestToPublic(value))
	}
	return result, nil
}

func (e engine) CreateGuest(ctx context.Context, input schoolmembership.CreateGuest) (schoolmembership.Guest, error) {
	value, err := e.service.CreateGuest(ctx, guestFieldsToDomain(input.GuestFields))
	return guestToPublic(value), mapError(err)
}

func (e engine) UpdateGuest(ctx context.Context, input schoolmembership.UpdateGuest) (schoolmembership.Guest, error) {
	value, err := e.service.UpdateGuest(ctx, input.ID, guestFieldsToDomain(input.GuestFields))
	return guestToPublic(value), mapError(err)
}

func (e engine) DeleteGuest(ctx context.Context, id int64) error {
	return mapError(e.service.DeleteGuest(ctx, id))
}

func staffFieldsToDomain(fields schoolmembership.StaffFields) domain.StaffFields {
	return domain.StaffFields{
		PersonID: fields.PersonID, StaffNotes: fields.StaffNotes, EmploymentType: fields.EmploymentType,
		WorkTimeModelID: fields.WorkTimeModelID, PersonnelNumber: fields.PersonnelNumber,
		RotationAnchorDate: fields.RotationAnchorDate, BirthdayDisplayOptOut: fields.BirthdayDisplayOptOut,
	}
}

func teacherFieldsToDomain(fields schoolmembership.TeacherFields) domain.TeacherFields {
	return domain.TeacherFields{
		StaffID: fields.StaffID, Specialization: fields.Specialization, Role: fields.Role, Qualifications: fields.Qualifications,
	}
}

func guestFieldsToDomain(fields schoolmembership.GuestFields) domain.GuestFields {
	return domain.GuestFields{
		StaffID: fields.StaffID, Organization: fields.Organization, ContactEmail: fields.ContactEmail,
		ContactPhone: fields.ContactPhone, ActivityExpertise: fields.ActivityExpertise,
		StartDate: fields.StartDate, EndDate: fields.EndDate, Notes: fields.Notes,
	}
}

func staffToPublic(value domain.Staff) schoolmembership.Staff {
	return schoolmembership.Staff{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		PersonID: value.PersonID, StaffNotes: value.StaffNotes, EmploymentType: value.EmploymentType,
		WorkTimeModelID: value.WorkTimeModelID, PersonnelNumber: value.PersonnelNumber,
		RotationAnchorDate: value.RotationAnchorDate, BirthdayDisplayOptOut: value.BirthdayDisplayOptOut,
		DeletedAt: value.DeletedAt,
	}
}

func teacherToPublic(value domain.Teacher) schoolmembership.Teacher {
	return schoolmembership.Teacher{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		StaffID: value.StaffID, Specialization: value.Specialization, Role: value.Role,
		Qualifications: value.Qualifications, DeletedAt: value.DeletedAt,
	}
}

func guestToPublic(value domain.Guest) schoolmembership.Guest {
	return schoolmembership.Guest{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		StaffID: value.StaffID, Organization: value.Organization, ContactEmail: value.ContactEmail,
		ContactPhone: value.ContactPhone, ActivityExpertise: value.ActivityExpertise,
		StartDate: value.StartDate, EndDate: value.EndDate, Notes: value.Notes,
	}
}

func mapError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrStaffNotFound):
		return schoolmembership.ErrStaffNotFound
	case errors.Is(err, domain.ErrTeacherNotFound):
		return schoolmembership.ErrTeacherNotFound
	case errors.Is(err, domain.ErrGuestNotFound):
		return schoolmembership.ErrGuestNotFound
	case errors.Is(err, domain.ErrStaffPersonConflict):
		return schoolmembership.ErrStaffPersonConflict
	case errors.Is(err, domain.ErrTeacherStaffConflict):
		return schoolmembership.ErrTeacherStaffConflict
	case errors.Is(err, domain.ErrGuestStaffConflict):
		return schoolmembership.ErrGuestStaffConflict
	case errors.Is(err, domain.ErrPersonnelNumberConflict):
		return schoolmembership.ErrPersonnelNumberConflict
	default:
		return err
	}
}
