package presence

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// LockStudentForVisit retains only admission/routing facts protected by the
// existing membership row lock. The kiosk's intervening visit/attendance
// commands do not write membership. In particular, this is not a snapshot of
// the live care flags: check-in must still observe its own status writes.
func (s *service) LockStudentForVisit(ctx context.Context, studentID int64) (studentpresence.VisitAdmission, error) {
	student, err := s.lockVisitStudent(ctx, studentID)
	if err != nil {
		return nil, err
	}
	var admission *StudentRecord
	if student != nil {
		admission = &StudentRecord{ID: student.ID, Lifecycle: student.Lifecycle}
		if student.GroupID != nil {
			groupID := *student.GroupID
			admission.GroupID = &groupID
		}
		if student.EnrolledUntil != nil {
			until := *student.EnrolledUntil
			admission.EnrolledUntil = &until
		}
	}
	return &lockedStudentVisit{service: s, ctx: ctx, studentID: studentID, admission: admission}, nil
}

type lockedStudentVisit struct {
	service   *service
	ctx       context.Context
	studentID int64
	admission *StudentRecord
	used      bool
}

func (v *lockedStudentVisit) StudentID() int64 { return v.studentID }

func (s *service) createAdmittedVisit(ctx, txCtx context.Context, visit *studentpresence.Visit, admission []studentpresence.VisitAdmission) error {
	if len(admission) != 1 {
		return &ActiveError{Op: "CreateVisit", Err: ErrInvalidData}
	}
	v, ok := admission[0].(*lockedStudentVisit)
	if !ok || v == nil || v.service != s || v.ctx != ctx || v.used || visit == nil || visit.StudentID != v.studentID {
		return &ActiveError{Op: "CreateVisit", Err: ErrInvalidData}
	}
	v.used = true
	return v.service.createVisitWithAdmission(txCtx, visit, func(ctx context.Context, _ int64) (*StudentRecord, error) {
		// The replaced locking read observed cancellation before checking
		// membership. Keep that error precedence when reusing its snapshot.
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return v.service.checkStudentAdmission(v.admission)
	})
}
