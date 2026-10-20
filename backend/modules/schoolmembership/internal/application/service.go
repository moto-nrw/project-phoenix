package application

import (
	"context"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/ports"
)

// Service is the School Membership owner. It validates every input at the
// owner edge before the operation is observed, runs every membership
// operation inside the caller's ambient transaction (or one it opens for the
// tenant in context), records one observation per call and turns "no row"
// outcomes into the public sentinels.
type Service struct {
	store      ports.Store
	employment ports.StaffEmployment
	tx         ports.Transaction
	observe    ports.Observer

	// quota and today back the Kinderkontingent check (#3567). A graph that
	// never binds quota still serves every other operation; its counting
	// writes fail closed instead of skipping the check.
	quota ports.ChildQuota
	today ports.Clock

	// The audited class-list administration (#2382) reaches two other
	// owners. Nil for a service composed without it; every other operation
	// works without them.
	classList *ClassListAdministration
}

var _ schoolmembership.Owner = (*Service)(nil)

func New(store ports.Store, employment ports.StaffEmployment, quota ports.ChildQuota, today ports.Clock, tx ports.Transaction, observe ports.Observer, classList *ClassListAdministration) *Service {
	if store == nil || employment == nil || today == nil || tx == nil || observe == nil {
		panic("school membership application: all dependencies are required")
	}
	if classList != nil && (classList.Students == nil || classList.Trail == nil) {
		panic("school membership application: class list entry administration needs a student directory and an audit trail")
	}
	return &Service{store: store, employment: employment, quota: quota, today: today, tx: tx, observe: observe, classList: classList}
}

// --- staff ---
//
// A staff member is the membership row this module stores plus the
// employment profile Workforce stores under the same ID (#2753). Reads compose
// the two in the caller's transaction; CreateStaff and UpdateStaff are the one
// unit of work that writes both, inside a savepoint.

func (s *Service) FindStaff(ctx context.Context, id int64) (schoolmembership.Staff, error) {
	if id <= 0 {
		return schoolmembership.Staff{}, invalid("staff ID is required")
	}
	return s.findStaff(ctx, id, "")
}

// FindStaffForMutation locks the row for the caller's transaction.
func (s *Service) FindStaffForMutation(ctx context.Context, id int64) (schoolmembership.Staff, error) {
	if id <= 0 {
		return schoolmembership.Staff{}, invalid("staff ID is required")
	}
	return s.findStaff(ctx, id, "UPDATE")
}

func (s *Service) findStaff(ctx context.Context, id int64, lock string) (result schoolmembership.Staff, err error) {
	run := s.runRead
	if lock != "" {
		run = s.runWrite
	}
	err = run(ctx, "find_staff", func(txCtx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindStaff(txCtx, id, lock, false)
		stats.Add(queryStats)
		if err == nil && !found {
			result = schoolmembership.Staff{}
			return schoolmembership.ErrStaffNotFound
		}
		if err != nil {
			return err
		}
		result, err = s.withEmployment(txCtx, result, stats)
		return err
	})
	return result, err
}

func (s *Service) FindStaffByPerson(ctx context.Context, personID int64) (schoolmembership.Staff, error) {
	if personID <= 0 {
		return schoolmembership.Staff{}, invalid("person ID is required")
	}
	return s.findStaffByPerson(ctx, "find_staff_by_person", personID, true)
}

// findStaffMembershipByPerson reads the membership row alone. The identity
// chain resolves every request's caller through it and needs only the ID,
// so it does not pay for the Workforce half.
func (s *Service) findStaffMembershipByPerson(ctx context.Context, personID int64) (schoolmembership.Staff, error) {
	return s.findStaffByPerson(ctx, "find_staff_membership_by_person", personID, false)
}

func (s *Service) findStaffByPerson(ctx context.Context, operation string, personID int64, withEmployment bool) (result schoolmembership.Staff, err error) {
	err = s.runRead(ctx, operation, func(txCtx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindStaffByPerson(txCtx, personID)
		stats.Add(queryStats)
		if err == nil && !found {
			return schoolmembership.ErrStaffNotFound
		}
		if err != nil || !withEmployment {
			return err
		}
		result, err = s.withEmployment(txCtx, result, stats)
		return err
	})
	return result, err
}

// ListStaff returns the staff rows matching the filter, sorted by ID.
func (s *Service) ListStaff(ctx context.Context, filter schoolmembership.StaffFilter) (result []schoolmembership.Staff, err error) {
	filter.IDs = uniquePositive(filter.IDs)
	filter.PersonIDs = uniquePositive(filter.PersonIDs)
	filter.TenantIDs = uniquePositive(filter.TenantIDs)
	err = s.runRead(ctx, "list_staff", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListStaff(txCtx, filter)
		stats.Add(queryStats)
		if err != nil || filter.MembershipOnly {
			return err
		}
		return s.composeEmployment(txCtx, result, stats)
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) CreateStaff(ctx context.Context, input schoolmembership.CreateStaff) (result schoolmembership.Staff, err error) {
	if err := validateStaffFields(&input.StaffFields); err != nil {
		return schoolmembership.Staff{}, err
	}
	fields := input.StaffFields
	err = s.runStaffWrite(ctx, "create_staff", func(txCtx context.Context, stats *domain.OperationStats) error {
		var createStats domain.OperationStats
		result, createStats, err = s.store.CreateStaff(txCtx, fields.PersonID)
		stats.Add(createStats)
		if err != nil {
			return err
		}
		result, err = s.saveEmployment(txCtx, result, fields, stats)
		return err
	})
	return result, err
}

func (s *Service) UpdateStaff(ctx context.Context, input schoolmembership.UpdateStaff) (result schoolmembership.Staff, err error) {
	if input.ID <= 0 {
		return schoolmembership.Staff{}, invalid("staff ID is required")
	}
	if err := validateStaffFields(&input.StaffFields); err != nil {
		return schoolmembership.Staff{}, err
	}
	id, fields := input.ID, input.StaffFields
	err = s.runStaffWrite(ctx, "update_staff", func(txCtx context.Context, stats *domain.OperationStats) error {
		if err := s.requireStaffLocked(txCtx, id, stats); err != nil {
			return err
		}
		var updateStats domain.OperationStats
		result, updateStats, err = s.store.UpdateStaff(txCtx, id, fields.PersonID)
		stats.Add(updateStats)
		if err != nil {
			return err
		}
		result, err = s.saveEmployment(txCtx, result, fields, stats)
		return err
	})
	return result, err
}

// DeleteStaff soft-deletes the staff row (deleted_at), keeping it.
func (s *Service) DeleteStaff(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalid("staff ID is required")
	}
	return s.runWrite(ctx, "delete_staff", func(txCtx context.Context, stats *domain.OperationStats) error {
		if err := s.requireStaffLocked(txCtx, id, stats); err != nil {
			return err
		}
		deleteStats, err := s.store.SoftDeleteStaff(txCtx, id)
		stats.Add(deleteStats)
		return err
	})
}

// runStaffWrite is the unit of work of a staff write that reaches both
// owners: a failed Workforce write rolls the membership write back even when
// an ambient caller catches the error and commits its own transaction.
func (s *Service) runStaffWrite(ctx context.Context, operation string, fn func(context.Context, *domain.OperationStats) error) error {
	return s.runWrite(ctx, operation, func(txCtx context.Context, stats *domain.OperationStats) error {
		return s.tx.RunSavepoint(txCtx, func(spCtx context.Context) error { return fn(spCtx, stats) })
	})
}

func (s *Service) saveEmployment(ctx context.Context, staff schoolmembership.Staff, fields schoolmembership.StaffFields, stats *domain.OperationStats) (schoolmembership.Staff, error) {
	employment := employmentOf(fields, staff.ID)
	stats.Queries++
	if err := s.employment.SaveStaffEmployment(ctx, employment); err != nil {
		return schoolmembership.Staff{}, err
	}
	return staffWithEmployment(staff, employment), nil
}

func (s *Service) withEmployment(ctx context.Context, staff schoolmembership.Staff, stats *domain.OperationStats) (schoolmembership.Staff, error) {
	members := []schoolmembership.Staff{staff}
	if err := s.composeEmployment(ctx, members, stats); err != nil {
		return schoolmembership.Staff{}, err
	}
	return members[0], nil
}

// composeEmployment fills the Workforce half of every listed staff member in
// one read. A membership without a profile keeps empty employment fields.
func (s *Service) composeEmployment(ctx context.Context, members []schoolmembership.Staff, stats *domain.OperationStats) error {
	if len(members) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(members))
	for _, member := range members {
		ids = append(ids, member.ID)
	}
	stats.Queries++
	profiles, err := s.employment.StaffEmployments(ctx, ids)
	if err != nil {
		return err
	}
	for i, member := range members {
		if profile, ok := profiles[member.ID]; ok {
			members[i] = staffWithEmployment(member, profile)
		}
	}
	return nil
}

// --- teachers ---

func (s *Service) FindTeacher(ctx context.Context, id int64) (result schoolmembership.Teacher, err error) {
	if id <= 0 {
		return schoolmembership.Teacher{}, invalid("teacher ID is required")
	}
	err = s.runRead(ctx, "find_teacher", func(txCtx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindTeacher(txCtx, id, "")
		stats.Add(queryStats)
		if err == nil && !found {
			return schoolmembership.ErrTeacherNotFound
		}
		return err
	})
	return result, err
}

// FindTeacherByStaff returns the live teacher profile of a staff member; a
// staff member without one yields ErrTeacherNotFound.
func (s *Service) FindTeacherByStaff(ctx context.Context, staffID int64) (result schoolmembership.Teacher, err error) {
	if staffID <= 0 {
		return schoolmembership.Teacher{}, invalid("staff ID is required")
	}
	err = s.runRead(ctx, "find_teacher_by_staff", func(txCtx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindTeacherByStaff(txCtx, staffID)
		stats.Add(queryStats)
		if err == nil && !found {
			return schoolmembership.ErrTeacherNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) ListTeachers(ctx context.Context, filter schoolmembership.TeacherFilter) (result []schoolmembership.Teacher, err error) {
	filter.IDs = uniquePositive(filter.IDs)
	filter.StaffIDs = uniquePositive(filter.StaffIDs)
	filter.Specialization = strings.TrimSpace(filter.Specialization)
	filter.SpecializationContains = strings.TrimSpace(filter.SpecializationContains)
	filter.RoleContains = strings.TrimSpace(filter.RoleContains)
	err = s.runRead(ctx, "list_teachers", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListTeachers(txCtx, filter)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) CreateTeacher(ctx context.Context, input schoolmembership.CreateTeacher) (result schoolmembership.Teacher, err error) {
	if err := validateTeacherFields(&input.TeacherFields); err != nil {
		return schoolmembership.Teacher{}, err
	}
	fields := input.TeacherFields
	err = s.runWrite(ctx, "create_teacher", func(txCtx context.Context, stats *domain.OperationStats) error {
		if err := s.requireStaffLocked(txCtx, fields.StaffID, stats); err != nil {
			return err
		}
		var createStats domain.OperationStats
		result, createStats, err = s.store.CreateTeacher(txCtx, fields)
		stats.Add(createStats)
		return err
	})
	return result, err
}

func (s *Service) UpdateTeacher(ctx context.Context, input schoolmembership.UpdateTeacher) (result schoolmembership.Teacher, err error) {
	if input.ID <= 0 {
		return schoolmembership.Teacher{}, invalid("teacher ID is required")
	}
	if err := validateTeacherFields(&input.TeacherFields); err != nil {
		return schoolmembership.Teacher{}, err
	}
	id, fields := input.ID, input.TeacherFields
	err = s.runWrite(ctx, "update_teacher", func(txCtx context.Context, stats *domain.OperationStats) error {
		// Preserve the missing-teacher contract before validating the target
		// staff, but acquire write locks in staff -> teacher order.
		_, visible, readStats, readErr := s.store.FindTeacher(txCtx, id, "")
		stats.Add(readStats)
		if readErr != nil {
			return readErr
		}
		if !visible {
			return schoolmembership.ErrTeacherNotFound
		}
		if err := s.requireStaffLocked(txCtx, fields.StaffID, stats); err != nil {
			return err
		}
		_, found, queryStats, err := s.store.FindTeacher(txCtx, id, "UPDATE")
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		if !found {
			return schoolmembership.ErrTeacherNotFound
		}
		var updateStats domain.OperationStats
		result, updateStats, err = s.store.UpdateTeacher(txCtx, id, fields)
		stats.Add(updateStats)
		return err
	})
	return result, err
}

// DeleteTeacher soft-deletes the teacher profile.
func (s *Service) DeleteTeacher(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalid("teacher ID is required")
	}
	return s.runWrite(ctx, "delete_teacher", func(txCtx context.Context, stats *domain.OperationStats) error {
		_, found, queryStats, err := s.store.FindTeacher(txCtx, id, "UPDATE")
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		if !found {
			return schoolmembership.ErrTeacherNotFound
		}
		deleteStats, err := s.store.SoftDeleteTeacher(txCtx, id)
		stats.Add(deleteStats)
		return err
	})
}

// --- guests ---

func (s *Service) FindGuest(ctx context.Context, id int64) (result schoolmembership.Guest, err error) {
	if id <= 0 {
		return schoolmembership.Guest{}, invalid("guest ID is required")
	}
	err = s.runRead(ctx, "find_guest", func(txCtx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindGuest(txCtx, id, "")
		stats.Add(queryStats)
		if err == nil && !found {
			return schoolmembership.ErrGuestNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) FindGuestByStaff(ctx context.Context, staffID int64) (result schoolmembership.Guest, err error) {
	if staffID <= 0 {
		return schoolmembership.Guest{}, invalid("staff ID is required")
	}
	err = s.runRead(ctx, "find_guest_by_staff", func(txCtx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindGuestByStaff(txCtx, staffID)
		stats.Add(queryStats)
		if err == nil && !found {
			return schoolmembership.ErrGuestNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) ListGuests(ctx context.Context, filter schoolmembership.GuestFilter) (result []schoolmembership.Guest, err error) {
	filter.IDs = uniquePositive(filter.IDs)
	filter.StaffIDs = uniquePositive(filter.StaffIDs)
	filter.OrganizationContains = strings.TrimSpace(filter.OrganizationContains)
	filter.ExpertiseContains = strings.TrimSpace(filter.ExpertiseContains)
	if err := validateDate(filter.ActiveOn, "active-on date"); err != nil {
		return nil, err
	}
	err = s.runRead(ctx, "list_guests", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListGuests(txCtx, filter)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) CreateGuest(ctx context.Context, input schoolmembership.CreateGuest) (result schoolmembership.Guest, err error) {
	if err := validateGuestFields(&input.GuestFields); err != nil {
		return schoolmembership.Guest{}, err
	}
	fields := input.GuestFields
	err = s.runWrite(ctx, "create_guest", func(txCtx context.Context, stats *domain.OperationStats) error {
		var createStats domain.OperationStats
		result, createStats, err = s.store.CreateGuest(txCtx, fields)
		stats.Add(createStats)
		return err
	})
	return result, err
}

func (s *Service) UpdateGuest(ctx context.Context, input schoolmembership.UpdateGuest) (result schoolmembership.Guest, err error) {
	if input.ID <= 0 {
		return schoolmembership.Guest{}, invalid("guest ID is required")
	}
	if err := validateGuestFields(&input.GuestFields); err != nil {
		return schoolmembership.Guest{}, err
	}
	id, fields := input.ID, input.GuestFields
	err = s.runWrite(ctx, "update_guest", func(txCtx context.Context, stats *domain.OperationStats) error {
		_, found, queryStats, err := s.store.FindGuest(txCtx, id, "UPDATE")
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		if !found {
			return schoolmembership.ErrGuestNotFound
		}
		var updateStats domain.OperationStats
		result, updateStats, err = s.store.UpdateGuest(txCtx, id, fields)
		stats.Add(updateStats)
		return err
	})
	return result, err
}

// DeleteGuest removes the guest profile; guests carry no tombstone.
func (s *Service) DeleteGuest(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalid("guest ID is required")
	}
	return s.runWrite(ctx, "delete_guest", func(txCtx context.Context, stats *domain.OperationStats) error {
		deleteStats, err := s.store.DeleteGuest(txCtx, id)
		stats.Add(deleteStats)
		return err
	})
}

// --- plumbing ---

func (s *Service) requireStaffLocked(ctx context.Context, id int64, stats *domain.OperationStats) error {
	_, found, queryStats, err := s.store.FindStaff(ctx, id, "UPDATE", false)
	stats.Add(queryStats)
	if err != nil {
		return err
	}
	if !found {
		return schoolmembership.ErrStaffNotFound
	}
	return nil
}

func (s *Service) runWrite(ctx context.Context, operation string, fn func(context.Context, *domain.OperationStats) error) error {
	return s.observeRun(ctx, operation, s.tx.RunWrite, fn)
}

func (s *Service) runRead(ctx context.Context, operation string, fn func(context.Context, *domain.OperationStats) error) error {
	return s.observeRun(ctx, operation, s.tx.RunRead, fn)
}

func (s *Service) observeRun(ctx context.Context, operation string, run func(context.Context, func(context.Context) error) error, fn func(context.Context, *domain.OperationStats) error) (err error) {
	started := time.Now()
	stats := domain.OperationStats{}
	defer func() {
		s.observe(ports.Observation{Operation: operation, Duration: time.Since(started), Stats: stats, Err: err})
	}()
	err = run(ctx, func(txCtx context.Context) error { return fn(txCtx, &stats) })
	if err != nil {
		stats.Rows = 0
	}
	return err
}
