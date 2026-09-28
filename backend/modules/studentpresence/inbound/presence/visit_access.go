package presence

import (
	"context"
	"fmt"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

const visitAccessPolicyName = "student_visit_access"

// VisitAccessQuery supplies tenant-scoped facts to the visit authorization policy.
type VisitAccessQuery interface {
	VisitStudentID(context.Context, int64) (int64, error)
	PersonIDByAccount(context.Context, int64) (int64, bool)
	StudentIDByPerson(context.Context, int64) (int64, bool)
	StaffIDByPerson(context.Context, int64) (int64, bool)
	TeacherIDByStaff(context.Context, int64) (int64, bool)
	TeacherGroupIDs(context.Context, int64) ([]int64, error)
	StudentGroupID(context.Context, int64) (int64, bool)
	SupervisedActiveGroupIDs(context.Context, int64) []int64
	StudentCurrentActiveGroupID(context.Context, int64) (int64, bool)
}

type VisitAuthorizer func(context.Context, int64, bool, int64, VisitAccessQuery) (bool, error)

// Authorization supplies the Security Runtime policies used by active routes.
type Authorization struct {
	Visit               VisitAuthorizer
	OperationalOverview func(context.Context, Settings, StaffAccess, bool, bool) (bool, error)
}

type visitAccessQuery struct{ resource *Resource }

func (q visitAccessQuery) VisitStudentID(ctx context.Context, visitID int64) (int64, error) {
	visit, err := q.resource.findPresenceVisit(ctx, visitID)
	if err != nil {
		return 0, err
	}
	return visit.StudentID, nil
}

func (q visitAccessQuery) PersonIDByAccount(ctx context.Context, accountID int64) (int64, bool) {
	person, err := q.resource.PersonService.FindByAccountID(ctx, accountID)
	if err != nil || person == nil {
		return 0, false
	}
	return person.ID, person.ID > 0
}

func (q visitAccessQuery) StudentIDByPerson(ctx context.Context, personID int64) (int64, bool) {
	student, err := q.resource.PersonService.GetStudentByPersonID(ctx, personID)
	if err != nil || student == nil {
		return 0, false
	}
	return student.ID, student.ID > 0
}

func (q visitAccessQuery) StaffIDByPerson(ctx context.Context, personID int64) (int64, bool) {
	staff, err := q.resource.PersonService.GetStaffByPersonID(ctx, personID)
	if err != nil || staff == nil {
		return 0, false
	}
	return staff.ID, staff.ID > 0
}

func (q visitAccessQuery) TeacherIDByStaff(ctx context.Context, staffID int64) (int64, bool) {
	teacher, err := q.resource.PersonService.GetTeacherByStaffID(ctx, staffID)
	if err != nil || teacher == nil {
		return 0, false
	}
	return teacher.ID, teacher.ID > 0
}

func (q visitAccessQuery) TeacherGroupIDs(ctx context.Context, teacherID int64) ([]int64, error) {
	return q.resource.EducationService(ctx, teacherID)
}

func (q visitAccessQuery) StudentGroupID(ctx context.Context, studentID int64) (int64, bool) {
	student, err := q.resource.PersonService.GetStudentByID(ctx, studentID)
	if err != nil || student == nil || student.GroupID == nil {
		return 0, false
	}
	return *student.GroupID, true
}

func (q visitAccessQuery) SupervisedActiveGroupIDs(ctx context.Context, staffID int64) []int64 {
	day := calendar.TodayDate().String()
	supervisors, err := q.resource.Presence.QueryGroupSupervisions(ctx, studentpresence.GroupSupervisionFilter{StaffID: &staffID, ActiveOn: &day})
	if err != nil {
		return nil
	}
	ids := make([]int64, 0, len(supervisors))
	for _, supervisor := range supervisors {
		ids = append(ids, supervisor.GroupID)
	}
	return ids
}

func (q visitAccessQuery) StudentCurrentActiveGroupID(ctx context.Context, studentID int64) (int64, bool) {
	visit, err := q.resource.currentPresenceVisit(ctx, studentID)
	if err != nil || visit == nil {
		return 0, false
	}
	return visit.ActiveGroupID, true
}

func (rs *Resource) requireVisitView(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accountID, readAll, err := common.VisitReadAccess(r.Context())
		if err != nil {
			common.RenderError(w, r, common.ErrorUnauthorized(err))
			return
		}
		// Malformed IDs historically reach the policy as zero and produce the
		// same 403 as an unknown relationship; preserve that response contract.
		visitID, _ := common.ParseID(r)
		allowed, err := rs.authorization.Visit(r.Context(), accountID, readAll, visitID, visitAccessQuery{resource: rs})
		if err != nil {
			renderVisitAuthorizationError(w, r, fmt.Errorf("policy %s evaluation failed: %w", visitAccessPolicyName, err))
			return
		}
		if !allowed {
			rs.getLogger().WarnContext(r.Context(), "forbidden access attempt", "path", r.URL.Path)
			common.RenderError(w, r, common.AuthorizationForbidden())
			return
		}
		next.ServeHTTP(w, r)
	})
}

func renderVisitAuthorizationError(w http.ResponseWriter, r *http.Request, err error) {
	common.RenderError(w, r, common.ErrorInternalServerWrap("Authorization error", err))
}
