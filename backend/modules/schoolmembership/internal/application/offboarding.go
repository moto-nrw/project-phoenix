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
		staff, teacher := snapshot.Staff, snapshot.Teacher
		result.StaffID, result.PersonID = staff.ID, staff.PersonID
		if teacher.ID != 0 {
			result.TeacherID = teacher.ID
			deleteStats, deleteErr := s.store.DeleteGroupAssignmentsByTeacher(txCtx, teacher.ID)
			stats.Add(deleteStats)
			if deleteErr != nil {
				return deleteErr
			}
			result.GroupAssignments = deleteStats.Rows
			deleteStats, deleteErr = s.store.SoftDeleteTeacher(txCtx, teacher.ID)
			stats.Add(deleteStats)
			if deleteErr != nil {
				return deleteErr
			}
		}
		deleteStats, deleteErr := s.store.DeleteClassAssignmentsByStaff(txCtx, staffID)
		stats.Add(deleteStats)
		if deleteErr != nil {
			return deleteErr
		}
		result.ClassAssignments = deleteStats.Rows
		if staff.WorkTimeModelID != nil {
			clearStats, clearErr := s.store.ClearWorkTimeModel(txCtx, staffID)
			stats.Add(clearStats)
			if clearErr != nil {
				return clearErr
			}
		}
		deleteStats, deleteErr = s.store.SoftDeleteStaff(txCtx, staffID)
		stats.Add(deleteStats)
		return deleteErr
	})
	if err != nil {
		return domain.Retirement{}, err
	}
	return result, nil
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
	staff, found, queryStats, err := s.store.FindStaff(ctx, staffID, "UPDATE", false)
	stats.Add(queryStats)
	if err != nil {
		return snapshot, domain.RetirementPreview{}, err
	}
	if found {
		snapshot.Staff = staff
		teacher, teacherFound, teacherStats, err := s.store.FindTeacherByStaff(ctx, staffID)
		stats.Add(teacherStats)
		if err != nil {
			return snapshot, domain.RetirementPreview{}, err
		}
		if teacherFound {
			teacher, teacherFound, teacherStats, err = s.store.FindTeacher(ctx, teacher.ID, "UPDATE")
			stats.Add(teacherStats)
			if err != nil {
				return snapshot, domain.RetirementPreview{}, err
			}
			if teacherFound {
				if teacher.StaffID != staffID {
					return snapshot, domain.RetirementPreview{}, domain.ErrOffboardingConflict
				}
				snapshot.Teacher = teacher
				snapshot.Groups, queryStats, err = s.store.ListGroupAssignments(ctx, domain.GroupAssignmentFilter{TeacherIDs: []int64{teacher.ID}, ForUpdate: true})
				stats.Add(queryStats)
				if err != nil {
					return snapshot, domain.RetirementPreview{}, err
				}
			}
		}
		snapshot.Classes, queryStats, err = s.store.ListClassAssignments(ctx, domain.ClassAssignmentFilter{StaffIDs: []int64{staffID}, ForUpdate: true})
		stats.Add(queryStats)
		if err != nil {
			return snapshot, domain.RetirementPreview{}, err
		}
	}
	// The revision is an exact encoding of the decision facts, not a bearer
	// credential. Exclude staff notes and teacher qualifications: retirement
	// preserves those fields and does not use them to select mutations.
	payload, err := json.Marshal(struct {
		StaffID          int64
		TenantID         int64
		StaffUpdatedAt   time.Time
		PersonID         int64
		WorkTimeModelID  *int64
		TeacherID        int64
		TeacherUpdatedAt time.Time
		Classes          []domain.ClassAssignment
		Groups           []domain.GroupAssignment
	}{StaffID: staffID, TenantID: snapshot.Staff.TenantID,
		StaffUpdatedAt: snapshot.Staff.UpdatedAt, PersonID: snapshot.Staff.PersonID,
		WorkTimeModelID: snapshot.Staff.WorkTimeModelID, TeacherID: snapshot.Teacher.ID,
		TeacherUpdatedAt: snapshot.Teacher.UpdatedAt, Classes: snapshot.Classes, Groups: snapshot.Groups})
	if err != nil {
		return snapshot, domain.RetirementPreview{}, err
	}
	preview := domain.RetirementPreview{Revision: string(payload)}
	if found {
		preview.Retirement = domain.Retirement{StaffID: staff.ID, PersonID: staff.PersonID, TeacherID: snapshot.Teacher.ID, GroupAssignments: int64(len(snapshot.Groups)), ClassAssignments: int64(len(snapshot.Classes))}
	}
	return snapshot, preview, nil
}
