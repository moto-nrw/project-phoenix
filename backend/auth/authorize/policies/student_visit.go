package policies

import (
	"context"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/authorize/policy"
	"github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/education"
	"github.com/moto-nrw/project-phoenix/services/users"
)

// StudentVisitPolicy controls access to student visits using existing service interfaces
type StudentVisitPolicy struct {
	educationService education.Service
	usersService     users.PersonService
	activeService    active.Service
}

// NewStudentVisitPolicy creates a new student visit policy
func NewStudentVisitPolicy(
	educationService education.Service,
	usersService users.PersonService,
	activeService active.Service,
) policy.Policy {
	return &StudentVisitPolicy{
		educationService: educationService,
		usersService:     usersService,
		activeService:    activeService,
	}
}

// Name returns the name of this policy
func (p *StudentVisitPolicy) Name() string {
	return "student_visit_access"
}

// ResourceType returns the resource type this policy applies to
func (p *StudentVisitPolicy) ResourceType() string {
	return "visit"
}

// Evaluate evaluates whether the subject can access student visits
func (p *StudentVisitPolicy) Evaluate(ctx context.Context, authCtx *policy.Context) (bool, error) {
	// Admin users or users with specific permissions can always access
	if p.canAccessByRoleOrPermission(authCtx) {
		return true, nil
	}

	// Extract student ID from context
	studentID, err := p.extractStudentID(ctx, authCtx)
	if err != nil || studentID == 0 {
		return false, nil
	}

	// Get the requesting user's person record
	person, err := p.usersService.FindByAccountID(ctx, authCtx.Subject.AccountID)
	if err != nil {
		return false, nil
	}

	// Check if person is a student accessing their own visits
	if p.isStudentOwnVisit(ctx, person.ID, studentID) {
		return true, nil
	}

	// Check if person is staff/teacher supervising the student
	staff, err := p.usersService.GetStaffByPersonID(ctx, person.ID)
	if err != nil || staff == nil {
		return false, nil
	}

	teacher, err := p.usersService.TeacherRepository().FindByStaffID(ctx, staff.ID)
	if err != nil || teacher == nil {
		return false, nil
	}

	// Check if teacher supervises student's group
	if allowed, err := p.isTeacherSupervisingStudent(ctx, teacher.ID, studentID); err != nil || allowed {
		return allowed, err
	}

	// Check if staff is supervising student's current active group
	return p.isStaffSupervisingActiveGroup(ctx, staff.ID, studentID)
}

// canAccessByRoleOrPermission checks if user has admin role or visit permissions
func (p *StudentVisitPolicy) canAccessByRoleOrPermission(authCtx *policy.Context) bool {
	if hasRole(authCtx.Subject.Roles, "admin") {
		return true
	}

	return hasPermission(authCtx.Subject.Permissions, permissions.VisitsRead) ||
		hasPermission(authCtx.Subject.Permissions, permissions.VisitsManage)
}

// extractStudentID extracts student ID from extra context or resource ID
func (p *StudentVisitPolicy) extractStudentID(ctx context.Context, authCtx *policy.Context) (int64, error) {
	// Try to get from extra context first
	if id, ok := authCtx.Extra["student_id"].(int64); ok {
		return id, nil
	}

	// Try to get from resource ID (visit ID)
	if id, ok := authCtx.Resource.ID.(int64); ok {
		visit, err := p.activeService.GetVisit(ctx, id)
		if err != nil {
			return 0, err
		}
		return visit.StudentID, nil
	}

	return 0, nil
}

// isStudentOwnVisit checks if the person is the student accessing their own visits
func (p *StudentVisitPolicy) isStudentOwnVisit(ctx context.Context, personID, studentID int64) bool {
	student, err := p.usersService.StudentRepository().FindByPersonID(ctx, personID)
	return err == nil && student != nil && student.ID == studentID
}

// isTeacherSupervisingStudent checks if teacher supervises the student's group
func (p *StudentVisitPolicy) isTeacherSupervisingStudent(ctx context.Context, teacherID, studentID int64) (bool, error) {
	teacherGroups, err := p.educationService.GetTeacherGroups(ctx, teacherID)
	if err != nil {
		return false, err
	}

	targetStudent, err := p.usersService.StudentRepository().FindByID(ctx, studentID)
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

// isStaffSupervisingActiveGroup checks if staff is supervising student's current active group
func (p *StudentVisitPolicy) isStaffSupervisingActiveGroup(ctx context.Context, staffID, studentID int64) (bool, error) {
	activeSupervisors, err := p.activeService.FindSupervisorsByStaffID(ctx, staffID)
	if err != nil || len(activeSupervisors) == 0 {
		return false, nil
	}

	currentVisit, err := p.activeService.GetStudentCurrentVisit(ctx, studentID)
	if err != nil || currentVisit == nil {
		return false, nil
	}

	for _, supervisor := range activeSupervisors {
		if supervisor.GroupID == currentVisit.ActiveGroupID {
			return true, nil
		}
	}

	return false, nil
}

// Helper functions
func hasRole(roles []string, role string) bool {
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}

func hasPermission(permissions []string, permission string) bool {
	for _, p := range permissions {
		if p == permission || p == "*:*" || p == "admin:*" {
			return true
		}
	}
	return false
}
