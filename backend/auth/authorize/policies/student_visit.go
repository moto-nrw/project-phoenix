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
	// Admin users can always access
	if hasRole(authCtx.Subject.Roles, "admin") {
		return true, nil
	}

	// Users with specific permissions can access
	if hasPermission(authCtx.Subject.Permissions, permissions.VisitsRead) ||
		hasPermission(authCtx.Subject.Permissions, permissions.VisitsManage) {
		return true, nil
	}

	// Get student ID from extra context or resource ID
	var studentID int64
	if id, ok := authCtx.Extra["student_id"].(int64); ok {
		studentID = id
	} else if id, ok := authCtx.Resource.ID.(int64); ok {
		// If resource ID is the visit ID, we need to get the student ID from the visit
		visit, err := p.activeService.GetVisit(ctx, id)
		if err != nil {
			return false, nil
		}
		studentID = visit.StudentID
	}

	if studentID == 0 {
		return false, nil
	}

	// Get the requesting user's person record
	person, err := p.usersService.FindByAccountID(ctx, authCtx.Subject.AccountID)
	if err != nil {
		return false, nil
	}

	// Check if person is a student accessing their own visits
	student, err := p.usersService.StudentRepository().FindByPersonID(ctx, person.ID)
	if err == nil && student != nil && student.ID == studentID {
		return true, nil // Students can view their own visits
	}

	// Check if person is a staff member
	staff, err := p.usersService.StaffRepository().FindByPersonID(ctx, person.ID)
	if err != nil || staff == nil {
		return false, nil
	}

	// Check if staff is a teacher
	teacher, err := p.usersService.TeacherRepository().FindByStaffID(ctx, staff.ID)
	if err != nil || teacher == nil {
		return false, nil
	}

	// Get all groups the teacher supervises
	teacherGroups, err := p.educationService.GetTeacherGroups(ctx, teacher.ID)
	if err != nil {
		return false, err
	}

	// Get the student's information
	targetStudent, err := p.usersService.StudentRepository().FindByID(ctx, studentID)
	if err != nil || targetStudent == nil {
		return false, nil
	}

	// Check if student is in any of the teacher's groups
	if targetStudent.GroupID != nil {
		for _, group := range teacherGroups {
			if group.ID == *targetStudent.GroupID {
				return true, nil // Teacher supervises student's group
			}
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
