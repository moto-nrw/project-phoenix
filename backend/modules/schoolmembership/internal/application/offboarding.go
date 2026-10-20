package application

import (
	"context"
	"encoding/json"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
)

func (s *Service) RetireStaff(ctx context.Context, staffID int64, revision string) (result domain.Retirement, err error) {
	err = s.runWrite(ctx, "retire_staff", func(txCtx context.Context, stats *domain.OperationStats) error {
		snapshot, preview, previewErr := s.retirementSnapshot(txCtx, staffID, stats)
		if previewErr != nil {
			return previewErr
		}
		if snapshot.Staff.ID == 0 {
			return nil
		}
		if preview.Revision != revision {
			return domain.ErrOffboardingConflict
		}
		result.StaffID, result.PersonID = snapshot.Staff.ID, snapshot.Staff.PersonID
		return s.retireSnapshot(txCtx, snapshot, &result, stats)
	})
	if err != nil {
		return domain.Retirement{}, err
	}
	return result, nil
}

// retireSnapshot removes the locked teacher profile with its group
// assignments, then the class assignments, and soft-deletes the membership.
func (s *Service) retireSnapshot(ctx context.Context, snapshot retirementSnapshot, result *domain.Retirement, stats *domain.OperationStats) error {
	if snapshot.Teacher.ID != 0 {
		if err := s.retireTeacher(ctx, snapshot.Teacher.ID, result, stats); err != nil {
			return err
		}
	}
	deleteStats, deleteErr := s.store.DeleteClassAssignmentsByStaff(ctx, snapshot.StaffID)
	stats.Add(deleteStats)
	if deleteErr != nil {
		return deleteErr
	}
	result.ClassAssignments = deleteStats.Rows
	// The work-time binding stays on the retired membership: it has no
	// audit, and a Wiederaufnahme needs it back (ADR 0021). Workforce
	// releases it when the template itself is deleted.
	deleteStats, deleteErr = s.store.SoftDeleteStaff(ctx, snapshot.StaffID)
	stats.Add(deleteStats)
	return deleteErr
}

func (s *Service) retireTeacher(ctx context.Context, teacherID int64, result *domain.Retirement, stats *domain.OperationStats) error {
	result.TeacherID = teacherID
	deleteStats, deleteErr := s.store.DeleteGroupAssignmentsByTeacher(ctx, teacherID)
	stats.Add(deleteStats)
	if deleteErr != nil {
		return deleteErr
	}
	result.GroupAssignments = deleteStats.Rows
	deleteStats, deleteErr = s.store.SoftDeleteTeacher(ctx, teacherID)
	stats.Add(deleteStats)
	return deleteErr
}

type retirementSnapshot struct {
	StaffID int64
	Staff   domain.Staff
	Teacher domain.Teacher
	Classes []domain.ClassAssignment
	Groups  []domain.GroupAssignment
}

func (s *Service) PreviewRetirement(ctx context.Context, staffID int64) (result domain.RetirementPreview, err error) {
	err = s.runWrite(ctx, "preview_staff_retirement", func(txCtx context.Context, stats *domain.OperationStats) error {
		_, result, err = s.retirementSnapshot(txCtx, staffID, stats)
		return err
	})
	return result, err
}

func (s *Service) retirementSnapshot(ctx context.Context, staffID int64, stats *domain.OperationStats) (retirementSnapshot, domain.RetirementPreview, error) {
	snapshot := retirementSnapshot{StaffID: staffID}
	staff, found, err := s.lockStaffForRetirement(ctx, staffID, stats)
	if err != nil {
		return snapshot, domain.RetirementPreview{}, err
	}
	if found {
		snapshot.Staff = staff
		if err := s.lockRetirementAssignments(ctx, &snapshot, stats); err != nil {
			return snapshot, domain.RetirementPreview{}, err
		}
	}
	payload, err := retirementRevision(snapshot)
	if err != nil {
		return snapshot, domain.RetirementPreview{}, err
	}
	preview := domain.RetirementPreview{Revision: string(payload)}
	if found {
		preview.Retirement = domain.Retirement{StaffID: staff.ID, PersonID: staff.PersonID, TeacherID: snapshot.Teacher.ID, GroupAssignments: int64(len(snapshot.Groups)), ClassAssignments: int64(len(snapshot.Classes))}
	}
	return snapshot, preview, nil
}

// lockRetirementAssignments locks the teacher profile with its group
// assignments and then the class assignments of the snapshot's staff member.
func (s *Service) lockRetirementAssignments(ctx context.Context, snapshot *retirementSnapshot, stats *domain.OperationStats) error {
	if err := s.lockRetirementTeacher(ctx, snapshot, stats); err != nil {
		return err
	}
	classes, queryStats, err := s.store.ListClassAssignments(ctx, domain.ClassAssignmentFilter{StaffIDs: []int64{snapshot.StaffID}, ForUpdate: true})
	snapshot.Classes = classes
	stats.Add(queryStats)
	return err
}

// lockRetirementTeacher locks the staff member's teacher profile and its group
// assignments. A profile that moved to another staff member in between is an
// offboarding conflict.
func (s *Service) lockRetirementTeacher(ctx context.Context, snapshot *retirementSnapshot, stats *domain.OperationStats) error {
	teacher, found, teacherStats, err := s.store.FindTeacherByStaff(ctx, snapshot.StaffID)
	stats.Add(teacherStats)
	if err != nil || !found {
		return err
	}
	teacher, found, teacherStats, err = s.store.FindTeacher(ctx, teacher.ID, "UPDATE")
	stats.Add(teacherStats)
	if err != nil || !found {
		return err
	}
	if teacher.StaffID != snapshot.StaffID {
		return domain.ErrOffboardingConflict
	}
	snapshot.Teacher = teacher
	groups, queryStats, err := s.store.ListGroupAssignments(ctx, domain.GroupAssignmentFilter{TeacherIDs: []int64{teacher.ID}, ForUpdate: true})
	snapshot.Groups = groups
	stats.Add(queryStats)
	return err
}

// retirementRevision is an exact encoding of the decision facts, not a bearer
// credential. It excludes staff notes and teacher qualifications: retirement
// preserves those fields and does not use them to select mutations.
func retirementRevision(snapshot retirementSnapshot) ([]byte, error) {
	return json.Marshal(struct {
		StaffID          int64
		TenantID         int64
		StaffUpdatedAt   time.Time
		PersonID         int64
		WorkTimeModelID  *int64
		TeacherID        int64
		TeacherUpdatedAt time.Time
		Classes          []domain.ClassAssignment
		Groups           []domain.GroupAssignment
	}{StaffID: snapshot.StaffID, TenantID: snapshot.Staff.TenantID,
		StaffUpdatedAt: snapshot.Staff.UpdatedAt, PersonID: snapshot.Staff.PersonID,
		WorkTimeModelID: snapshot.Staff.WorkTimeModelID, TeacherID: snapshot.Teacher.ID,
		TeacherUpdatedAt: snapshot.Teacher.UpdatedAt, Classes: snapshot.Classes, Groups: snapshot.Groups})
}

// lockStaffForRetirement locks the live membership and composes the whole
// staff member: the work-time binding is part of the revision.
func (s *Service) lockStaffForRetirement(ctx context.Context, staffID int64, stats *domain.OperationStats) (domain.Staff, bool, error) {
	staff, found, queryStats, err := s.store.FindStaff(ctx, staffID, "UPDATE", false)
	stats.Add(queryStats)
	if err != nil || !found {
		return domain.Staff{}, found, err
	}
	staff, err = s.withEmployment(ctx, staff, stats)
	return staff, err == nil, err
}
