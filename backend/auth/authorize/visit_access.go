package authorize

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	educationModel "github.com/moto-nrw/project-phoenix/models/education"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
)

// visitAccessPolicyName is kept from the retired policy-engine subsystem so
// the 500 error wrap stays byte-identical for log consumers.
const visitAccessPolicyName = "student_visit_access"

// VisitAccessActive is the narrow subset of the active service CanViewVisit
// needs: load the visit, list the caller's active-group supervisions, and
// find the student's current visit. Defined here (not imported) to keep this
// package a sibling of services/active without a package cycle.
type VisitAccessActive interface {
	GetVisit(ctx context.Context, id int64) (*activeModel.Visit, error)
	FindSupervisorsByStaffID(ctx context.Context, staffID int64) ([]*activeModel.GroupSupervisor, error)
	GetStudentCurrentVisit(ctx context.Context, studentID int64) (*activeModel.Visit, error)
}

// VisitAccessPersons is the narrow subset of the person service CanViewVisit
// needs to resolve the caller (person → student/staff/teacher) and the
// target student.
type VisitAccessPersons interface {
	FindByAccountID(ctx context.Context, accountID int64) (*usersModel.Person, error)
	GetStudentByPersonID(ctx context.Context, personID int64) (*usersModel.Student, error)
	GetStaffByPersonID(ctx context.Context, personID int64) (*usersModel.Staff, error)
	GetTeacherByStaffID(ctx context.Context, staffID int64) (*usersModel.Teacher, error)
	GetStudentByID(ctx context.Context, id int64) (*usersModel.Student, error)
}

// VisitAccessEducation is the narrow subset of the education service
// CanViewVisit needs: the groups a teacher supervises.
type VisitAccessEducation interface {
	GetTeacherGroups(ctx context.Context, teacherID int64) ([]*educationModel.Group, error)
}

// CanViewVisit decides whether the caller may read a single visit
// (GET /active/visits/{id}).
//
// Order:
//  1. Role "admin", an admin wildcard permission, or visits:read /
//     visits:manage → always true.
//  2. The caller is the visit's student → true.
//  3. The caller is a teacher of the student's education group → true.
//  4. The caller is a staff member supervising the student's current active
//     group → true.
//  5. Otherwise → false.
//
// Target-visit lookup failures propagate so the middleware preserves the
// retired StudentVisitPolicy's authorization-error path. Relationship lookup
// failures deny access (false, nil) except a GetTeacherGroups error, which also
// propagates instead of masking an infrastructure failure as a 403.
func CanViewVisit(
	ctx context.Context,
	accountID int64,
	roles []string,
	userPermissions []string,
	visitID int64,
	activeSvc VisitAccessActive,
	persons VisitAccessPersons,
	education VisitAccessEducation,
) (bool, error) {
	// Admin users or users with specific permissions can always access
	if slices.Contains(roles, "admin") ||
		HasAdminWildcard(userPermissions) ||
		slices.Contains(userPermissions, permissions.VisitsRead) ||
		slices.Contains(userPermissions, permissions.VisitsManage) {
		return true, nil
	}

	// Resolve the target student from the visit
	if visitID <= 0 {
		return false, nil
	}
	visit, err := activeSvc.GetVisit(ctx, visitID)
	if err != nil {
		return false, err
	}
	studentID := visit.StudentID
	if studentID == 0 {
		return false, nil
	}

	// Get the requesting user's person record
	person, err := persons.FindByAccountID(ctx, accountID)
	if err != nil {
		return false, nil
	}

	// Check if person is a student accessing their own visits
	if student, err := persons.GetStudentByPersonID(ctx, person.ID); err == nil && student != nil && student.ID == studentID {
		return true, nil
	}

	// Check if person is staff/teacher supervising the student
	staff, err := persons.GetStaffByPersonID(ctx, person.ID)
	if err != nil || staff == nil {
		return false, nil
	}

	teacher, err := persons.GetTeacherByStaffID(ctx, staff.ID)
	if err != nil || teacher == nil {
		return false, nil
	}

	// Check if teacher supervises student's group
	if allowed, err := teacherSupervisesStudent(ctx, education, persons, teacher.ID, studentID); err != nil || allowed {
		return allowed, err
	}

	// Check if staff is supervising student's current active group
	return staffSupervisesActiveGroup(ctx, activeSvc, staff.ID, studentID), nil
}

// teacherSupervisesStudent checks if the teacher supervises the student's
// education group. A GetTeacherGroups error propagates (500 path).
func teacherSupervisesStudent(ctx context.Context, education VisitAccessEducation, persons VisitAccessPersons, teacherID, studentID int64) (bool, error) {
	teacherGroups, err := education.GetTeacherGroups(ctx, teacherID)
	if err != nil {
		return false, err
	}

	targetStudent, err := persons.GetStudentByID(ctx, studentID)
	if err != nil || targetStudent == nil || targetStudent.GroupID == nil {
		return false, nil
	}

	for _, group := range teacherGroups {
		if group.ID == *targetStudent.GroupID {
			return true, nil
		}
	}

	return false, nil
}

// staffSupervisesActiveGroup checks if the staff member supervises the
// student's current active group. All lookup failures deny.
func staffSupervisesActiveGroup(ctx context.Context, activeSvc VisitAccessActive, staffID, studentID int64) bool {
	activeSupervisors, err := activeSvc.FindSupervisorsByStaffID(ctx, staffID)
	if err != nil || len(activeSupervisors) == 0 {
		return false
	}

	currentVisit, err := activeSvc.GetStudentCurrentVisit(ctx, studentID)
	if err != nil || currentVisit == nil {
		return false
	}

	for _, supervisor := range activeSupervisors {
		if supervisor.GroupID == currentVisit.ActiveGroupID {
			return true
		}
	}

	return false
}

// RequireVisitView guards GET /active/visits/{id} with CanViewVisit. The
// visit ID comes from the "id" URL parameter; an unparsable ID denies (as
// the retired extractor did). Responses and log lines are inherited
// unchanged from the retired resource middleware: authorization errors
// render 500 "Authorization error", denials render 403 with a
// "forbidden access attempt" warning.
func RequireVisitView(activeSvc VisitAccessActive, persons VisitAccessPersons, education VisitAccessEducation) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := jwt.ClaimsFromCtx(r.Context())
			userPermissions := jwt.PermissionsFromCtx(r.Context())
			visitID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

			allowed, err := CanViewVisit(r.Context(), int64(claims.ID), claims.Roles, userPermissions, visitID, activeSvc, persons, education)
			if err != nil {
				handleAuthorizationError(w, r, fmt.Errorf("policy %s evaluation failed: %w", visitAccessPolicyName, err))
				return
			}

			if !allowed {
				handleForbiddenResponse(w, r)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// handleAuthorizationError handles authorization error response
func handleAuthorizationError(w http.ResponseWriter, r *http.Request, err error) {
	slog.ErrorContext(r.Context(), "authorization error",
		slog.String("error", err.Error()),
		slog.String("path", r.URL.Path),
	)
	if render.Render(w, r, &ErrResponse{
		HTTPStatusCode: http.StatusInternalServerError,
		StatusText:     "Authorization error",
		ErrorText:      err.Error(),
	}) != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleForbiddenResponse handles forbidden response
func handleForbiddenResponse(w http.ResponseWriter, r *http.Request) {
	slog.WarnContext(r.Context(), "forbidden access attempt",
		slog.String("path", r.URL.Path),
	)
	if render.Render(w, r, ErrForbidden) != nil {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
	}
}
