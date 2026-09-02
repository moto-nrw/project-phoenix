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
	store   ports.Store
	tx      ports.Transaction
	observe ports.Observer
}

func New(store ports.Store, tx ports.Transaction, observe ports.Observer) *Service {
	if store == nil || tx == nil || observe == nil {
		panic("school membership application: all dependencies are required")
	}
	return &Service{store: store, tx: tx, observe: observe}
}

// --- staff ---

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
		return err
	})
	return result, err
}

func (s *Service) ListStaff(ctx context.Context, filter domain.StaffFilter) (result []domain.Staff, err error) {
	err = s.runRead(ctx, "list_staff", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListStaff(txCtx, filter)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) CreateStaff(ctx context.Context, fields domain.StaffFields) (result domain.Staff, err error) {
	err = s.runWrite(ctx, "create_staff", func(txCtx context.Context, stats *domain.OperationStats) error {
		var createStats domain.OperationStats
		result, createStats, err = s.store.CreateStaff(txCtx, fields)
		stats.Add(createStats)
		return err
	})
	return result, err
}

func (s *Service) UpdateStaff(ctx context.Context, id int64, fields domain.StaffFields) (result domain.Staff, err error) {
	err = s.runWrite(ctx, "update_staff", func(txCtx context.Context, stats *domain.OperationStats) error {
		if err := s.requireStaffLocked(txCtx, id, stats); err != nil {
			return err
		}
		var updateStats domain.OperationStats
		result, updateStats, err = s.store.UpdateStaff(txCtx, id, fields)
		stats.Add(updateStats)
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

// ClearWorkTimeModel also reaches soft-deleted rows: offboarding runs it
// after the tombstone so the retained row stops referencing the template.
func (s *Service) ClearWorkTimeModel(ctx context.Context, id int64) error {
	return s.runWrite(ctx, "clear_work_time_model", func(txCtx context.Context, stats *domain.OperationStats) error {
		clearStats, err := s.store.ClearWorkTimeModel(txCtx, id)
		stats.Add(clearStats)
		return err
	})
}

func (s *Service) AppendStaffNotes(ctx context.Context, id int64, notes string) (result domain.Staff, err error) {
	err = s.runWrite(ctx, "append_staff_notes", func(txCtx context.Context, stats *domain.OperationStats) error {
		staff, found, queryStats, err := s.store.FindStaff(txCtx, id, "UPDATE", false)
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrStaffNotFound
		}
		setStats, err := s.store.SetStaffNotes(txCtx, id, domain.AppendNotes(staff.StaffNotes, notes))
		stats.Add(setStats)
		if err != nil {
			return err
		}
		staff.StaffNotes = domain.AppendNotes(staff.StaffNotes, notes)
		result = staff
		return nil
	})
	return result, err
}

func (s *Service) SetBirthdayDisplayOptOut(ctx context.Context, id int64, optOut bool) error {
	return s.runWrite(ctx, "set_birthday_display_opt_out", func(txCtx context.Context, stats *domain.OperationStats) error {
		setStats, err := s.store.SetBirthdayDisplayOptOut(txCtx, id, optOut)
		stats.Add(setStats)
		return err
	})
}

func (s *Service) RebaseWorkTimeModelAnchor(ctx context.Context, workTimeModelID int64, anchorDate string) (result []int64, err error) {
	err = s.runWrite(ctx, "rebase_work_time_model_anchor", func(txCtx context.Context, stats *domain.OperationStats) error {
		var writeStats domain.OperationStats
		result, writeStats, err = s.store.RebaseWorkTimeModelAnchor(txCtx, workTimeModelID, anchorDate)
		stats.Add(writeStats)
		return err
	})
	if result == nil {
		result = []int64{}
	}
	return result, err
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
		var createStats domain.OperationStats
		result, createStats, err = s.store.CreateTeacher(txCtx, fields)
		stats.Add(createStats)
		return err
	})
	return result, err
}

func (s *Service) UpdateTeacher(ctx context.Context, id int64, fields domain.TeacherFields) (result domain.Teacher, err error) {
	err = s.runWrite(ctx, "update_teacher", func(txCtx context.Context, stats *domain.OperationStats) error {
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
