package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/ports"
)

// Service runs every membership operation inside the caller's ambient
// transaction (or one it opens for the tenant in context), records one
// observation per call and turns "no row" outcomes into the stable domain
// errors.
type Service struct {
	store      ports.Store
	employment ports.StaffEmployment
	tx         ports.Transaction
	observe    ports.Observer

	// The audited class-list administration (#2382) reaches two owners the
	// composition root only builds after this service exists, so they are
	// bound late through BindClassListEntryAdministration. Every other
	// operation works without them.
	classListAdmin classListAdministration
}

func New(store ports.Store, employment ports.StaffEmployment, tx ports.Transaction, observe ports.Observer) *Service {
	if store == nil || employment == nil || tx == nil || observe == nil {
		panic("school membership application: all dependencies are required")
	}
	return &Service{store: store, employment: employment, tx: tx, observe: observe}
}

// --- staff ---
//
// A staff member is the membership row this module stores plus the
// employment profile Workforce stores under the same ID (#2753). Reads compose
// the two in the caller's transaction; CreateStaff and UpdateStaff are the one
// unit of work that writes both, inside a savepoint.

func (s *Service) FindStaff(ctx context.Context, id int64, lock string) (result domain.Staff, err error) {
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
			result = domain.Staff{}
			return domain.ErrStaffNotFound
		}
		if err != nil {
			return err
		}
		result, err = s.withEmployment(txCtx, result, stats)
		return err
	})
	return result, err
}

func (s *Service) FindStaffByPerson(ctx context.Context, personID int64) (result domain.Staff, err error) {
	err = s.runRead(ctx, "find_staff_by_person", func(txCtx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindStaffByPerson(txCtx, personID)
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrStaffNotFound
		}
		if err != nil {
			return err
		}
		result, err = s.withEmployment(txCtx, result, stats)
		return err
	})
	return result, err
}

func (s *Service) ListStaff(ctx context.Context, filter domain.StaffFilter) (result []domain.Staff, err error) {
	err = s.runRead(ctx, "list_staff", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListStaff(txCtx, filter)
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		return s.composeEmployment(txCtx, result, stats)
	})
	return result, err
}

func (s *Service) CreateStaff(ctx context.Context, fields domain.StaffFields) (result domain.Staff, err error) {
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

func (s *Service) UpdateStaff(ctx context.Context, id int64, fields domain.StaffFields) (result domain.Staff, err error) {
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

func (s *Service) DeleteStaff(ctx context.Context, id int64) error {
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

func (s *Service) saveEmployment(ctx context.Context, staff domain.Staff, fields domain.StaffFields, stats *domain.OperationStats) (domain.Staff, error) {
	employment := fields.Employment(staff.ID)
	stats.Queries++
	if err := s.employment.SaveStaffEmployment(ctx, employment); err != nil {
		return domain.Staff{}, err
	}
	return staff.WithEmployment(employment), nil
}

func (s *Service) withEmployment(ctx context.Context, staff domain.Staff, stats *domain.OperationStats) (domain.Staff, error) {
	members := []domain.Staff{staff}
	if err := s.composeEmployment(ctx, members, stats); err != nil {
		return domain.Staff{}, err
	}
	return members[0], nil
}

// composeEmployment fills the Workforce half of every listed staff member in
// one read. A membership without a profile keeps empty employment fields.
func (s *Service) composeEmployment(ctx context.Context, members []domain.Staff, stats *domain.OperationStats) error {
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
			members[i] = member.WithEmployment(profile)
		}
	}
	return nil
}

// --- teachers ---

func (s *Service) FindTeacher(ctx context.Context, id int64) (result domain.Teacher, err error) {
	err = s.runRead(ctx, "find_teacher", func(txCtx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindTeacher(txCtx, id, "")
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrTeacherNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) FindTeacherByStaff(ctx context.Context, staffID int64) (result domain.Teacher, err error) {
	err = s.runRead(ctx, "find_teacher_by_staff", func(txCtx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindTeacherByStaff(txCtx, staffID)
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrTeacherNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) ListTeachers(ctx context.Context, filter domain.TeacherFilter) (result []domain.Teacher, err error) {
	err = s.runRead(ctx, "list_teachers", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListTeachers(txCtx, filter)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) CreateTeacher(ctx context.Context, fields domain.TeacherFields) (result domain.Teacher, err error) {
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

func (s *Service) UpdateTeacher(ctx context.Context, id int64, fields domain.TeacherFields) (result domain.Teacher, err error) {
	err = s.runWrite(ctx, "update_teacher", func(txCtx context.Context, stats *domain.OperationStats) error {
		// Preserve the missing-teacher contract before validating the target
		// staff, but acquire write locks in staff -> teacher order.
		_, visible, readStats, readErr := s.store.FindTeacher(txCtx, id, "")
		stats.Add(readStats)
		if readErr != nil {
			return readErr
		}
		if !visible {
			return domain.ErrTeacherNotFound
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
			return domain.ErrTeacherNotFound
		}
		var updateStats domain.OperationStats
		result, updateStats, err = s.store.UpdateTeacher(txCtx, id, fields)
		stats.Add(updateStats)
		return err
	})
	return result, err
}

func (s *Service) DeleteTeacher(ctx context.Context, id int64) error {
	return s.runWrite(ctx, "delete_teacher", func(txCtx context.Context, stats *domain.OperationStats) error {
		_, found, queryStats, err := s.store.FindTeacher(txCtx, id, "UPDATE")
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrTeacherNotFound
		}
		deleteStats, err := s.store.SoftDeleteTeacher(txCtx, id)
		stats.Add(deleteStats)
		return err
	})
}

// --- guests ---

func (s *Service) FindGuest(ctx context.Context, id int64) (result domain.Guest, err error) {
	err = s.runRead(ctx, "find_guest", func(txCtx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindGuest(txCtx, id, "")
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrGuestNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) FindGuestByStaff(ctx context.Context, staffID int64) (result domain.Guest, err error) {
	err = s.runRead(ctx, "find_guest_by_staff", func(txCtx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindGuestByStaff(txCtx, staffID)
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrGuestNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) ListGuests(ctx context.Context, filter domain.GuestFilter) (result []domain.Guest, err error) {
	err = s.runRead(ctx, "list_guests", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListGuests(txCtx, filter)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) CreateGuest(ctx context.Context, fields domain.GuestFields) (result domain.Guest, err error) {
	err = s.runWrite(ctx, "create_guest", func(txCtx context.Context, stats *domain.OperationStats) error {
		var createStats domain.OperationStats
		result, createStats, err = s.store.CreateGuest(txCtx, fields)
		stats.Add(createStats)
		return err
	})
	return result, err
}

func (s *Service) UpdateGuest(ctx context.Context, id int64, fields domain.GuestFields) (result domain.Guest, err error) {
	err = s.runWrite(ctx, "update_guest", func(txCtx context.Context, stats *domain.OperationStats) error {
		_, found, queryStats, err := s.store.FindGuest(txCtx, id, "UPDATE")
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrGuestNotFound
		}
		var updateStats domain.OperationStats
		result, updateStats, err = s.store.UpdateGuest(txCtx, id, fields)
		stats.Add(updateStats)
		return err
	})
	return result, err
}

func (s *Service) DeleteGuest(ctx context.Context, id int64) error {
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
		return domain.ErrStaffNotFound
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
