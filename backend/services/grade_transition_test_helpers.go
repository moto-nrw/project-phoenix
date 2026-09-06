package services

import (
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditSvc "github.com/moto-nrw/project-phoenix/services/audit"
	"github.com/moto-nrw/project-phoenix/services/education"
	"github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/uptrace/bun"
)

type GradeTransitionTestModule struct {
	GradeTransition *education.GradeTransitionService
}

func NewGradeTransitionTestModule(db *bun.DB, clocks ...func() time.Time) (GradeTransitionTestModule, error) {
	command, err := auditSvc.NewCommand(repositories.NewTestAuditStore(db), func(auditSvc.AppendObservation) {})
	if err != nil {
		return GradeTransitionTestModule{}, err
	}
	r, err := repositories.NewGradeTransitionTestRepositories(db, command, clocks...)
	if err != nil {
		return GradeTransitionTestModule{}, err
	}
	tt := r.Timetable
	service := education.NewGradeTransitionService(education.GradeTransitionServiceDependencies{
		TransitionRepo: r.Transition, StudentRepo: tt.Student, PersonRepo: tt.Person,
		VisitRepo: tt.ActiveVisit, AttendanceRepo: r.Attendance, ClassTeacherRepo: tt.ClassTeacher, StaffRepo: tt.Staff,
		ClassListEntryRepo: r.ClassListEntry, ClassListEntryAudit: r.ClassListEntryAudit,
		RosterReconciler: schedule.NewRosterReconciler(tt.ActivityInstance, tt.InstanceStudent, tt.StudentEnrollment, slog.Default(), optionalClock(clocks)),
		DB:               db, Today: timezone.CalendarDateClock(optionalClock(clocks)),
	})
	return GradeTransitionTestModule{GradeTransition: service}, nil
}
