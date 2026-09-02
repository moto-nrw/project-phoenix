package peopledirectory

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	ErrStudentNotFound = errors.New("student not found")
	ErrInvalidStudent  = errors.New("invalid student")
)

type InvalidStudentError struct{ Reason string }

func (e *InvalidStudentError) Error() string { return e.Reason }
func (e *InvalidStudentError) Unwrap() error { return ErrInvalidStudent }

// StudentStatusAlumnus marks a graduated child. The row stays, but every
// roster query of the directory leaves it out.
const StudentStatusAlumnus = "alumnus"

// Student is the directory view of users.students that other owners may
// read: identity, class and group, lifecycle status, the live absence
// flags and the photo path. Guardian contact columns stay with the owner.
// Enrolment dates are calendar days in BirthdayLayout, empty when unset.
type Student struct {
	ID            int64      `json:"id"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	TenantID      int64      `json:"tenant_id"`
	PersonID      int64      `json:"person_id"`
	SchoolClass   string     `json:"school_class"`
	GroupID       *int64     `json:"group_id,omitempty"`
	Status        string     `json:"status"`
	EnrolledFrom  string     `json:"enrolled_from,omitempty"`
	EnrolledUntil string     `json:"enrolled_until,omitempty"`
	Sick          *bool      `json:"sick,omitempty"`
	SickSince     *time.Time `json:"sick_since,omitempty"`
	Excused       *bool      `json:"excused,omitempty"`
	ExcusedSince  *time.Time `json:"excused_since,omitempty"`
	PhotoPath     *string    `json:"photo_path,omitempty"`
}

func (s Student) IsAlumnus() bool { return s.Status == StudentStatusAlumnus }

// StudentQuery reads the student directory. Reads are scoped to the tenant
// in context; inside an admin transaction they span every tenant.
type StudentQuery interface {
	// ListStudentsByID returns the rows for ids, alumni included, so callers
	// that need the lifecycle status of a graduate can still see it.
	ListStudentsByID(context.Context, []int64) ([]Student, error)
	// ListStudentsAcrossTenantsByID resolves visiting students (holiday care
	// at a partner school) in a separate admin transaction. Callers must
	// already hold a reference to the student (an open visit).
	ListStudentsAcrossTenantsByID(context.Context, []int64) ([]Student, error)
	// ListStudentsByClasses returns the non-alumni students of the given
	// classes ordered by class, then id.
	ListStudentsByClasses(context.Context, []string) ([]Student, error)
	// ListEnrolledStudents returns every non-alumni student of the current
	// tenant ordered by id.
	ListEnrolledStudents(context.Context) ([]Student, error)
	// ListSchoolClasses returns the distinct non-empty classes of non-alumni
	// students, ordered.
	ListSchoolClasses(context.Context) ([]string, error)
}

// StudentCommand changes student rows. Every command runs in the caller's
// transaction or opens one for the tenant in context.
type StudentCommand interface {
	// LockStudent takes the student row FOR UPDATE; it is the first lock of
	// every care-day writer. ErrStudentNotFound when the tenant has no such
	// row.
	LockStudent(context.Context, int64) error
	// PromoteStudents moves exactly the given non-alumni students that are
	// still in fromClass to toClass and returns the row count.
	PromoteStudents(ctx context.Context, ids []int64, fromClass, toClass string) (int64, error)
	// RevertStudentClass moves one student back to fromClass while it still
	// sits in toClass; 0 rows when the class changed since.
	RevertStudentClass(ctx context.Context, id int64, fromClass, toClass string) (int64, error)
	// GraduateStudentsByClasses flips every non-alumni student of the
	// classes to alumnus.
	GraduateStudentsByClasses(context.Context, []string) (int64, error)
	// GraduateStudents flips exactly the given non-alumni students to
	// alumnus.
	GraduateStudents(context.Context, []int64) (int64, error)
	// ReactivateStudents restores the given alumni to status and returns the
	// ids it actually changed.
	ReactivateStudents(ctx context.Context, ids []int64, status string) ([]int64, error)
}

type studentEngine interface {
	ListStudentsByIDs(context.Context, []int64) ([]Student, error)
	ListStudentsAcrossTenantsByIDs(context.Context, []int64) ([]Student, error)
	ListStudentsByClasses(context.Context, []string) ([]Student, error)
	ListEnrolledStudents(context.Context) ([]Student, error)
	ListSchoolClasses(context.Context) ([]string, error)
	LockStudent(context.Context, int64) error
	PromoteStudents(context.Context, []int64, string, string) (int64, error)
	RevertStudentClass(context.Context, int64, string, string) (int64, error)
	GraduateStudentsByClasses(context.Context, []string) (int64, error)
	GraduateStudents(context.Context, []int64) (int64, error)
	ReactivateStudents(context.Context, []int64, string) ([]int64, error)
}

func (m *Module) ListStudentsByID(ctx context.Context, ids []int64) ([]Student, error) {
	ids = uniquePositive(ids)
	if len(ids) == 0 {
		return []Student{}, nil
	}
	return m.engine.ListStudentsByIDs(ctx, ids)
}

func (m *Module) ListStudentsAcrossTenantsByID(ctx context.Context, ids []int64) ([]Student, error) {
	ids = uniquePositive(ids)
	if len(ids) == 0 {
		return []Student{}, nil
	}
	return m.engine.ListStudentsAcrossTenantsByIDs(ctx, ids)
}

func (m *Module) ListStudentsByClasses(ctx context.Context, classes []string) ([]Student, error) {
	classes = uniqueClasses(classes)
	if len(classes) == 0 {
		return []Student{}, nil
	}
	return m.engine.ListStudentsByClasses(ctx, classes)
}

func (m *Module) ListEnrolledStudents(ctx context.Context) ([]Student, error) {
	return m.engine.ListEnrolledStudents(ctx)
}

func (m *Module) ListSchoolClasses(ctx context.Context) ([]string, error) {
	return m.engine.ListSchoolClasses(ctx)
}

func (m *Module) LockStudent(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalidStudent("student ID is required")
	}
	return m.engine.LockStudent(ctx, id)
}

func (m *Module) PromoteStudents(ctx context.Context, ids []int64, fromClass, toClass string) (int64, error) {
	ids = uniquePositive(ids)
	if len(ids) == 0 {
		return 0, nil
	}
	if fromClass == "" || toClass == "" {
		return 0, invalidStudent("from and to class are required")
	}
	return m.engine.PromoteStudents(ctx, ids, fromClass, toClass)
}

func (m *Module) RevertStudentClass(ctx context.Context, id int64, fromClass, toClass string) (int64, error) {
	if id <= 0 {
		return 0, invalidStudent("student ID is required")
	}
	if fromClass == "" || toClass == "" {
		return 0, invalidStudent("from and to class are required")
	}
	return m.engine.RevertStudentClass(ctx, id, fromClass, toClass)
}

func (m *Module) GraduateStudentsByClasses(ctx context.Context, classes []string) (int64, error) {
	classes = uniqueClasses(classes)
	if len(classes) == 0 {
		return 0, nil
	}
	return m.engine.GraduateStudentsByClasses(ctx, classes)
}

func (m *Module) GraduateStudents(ctx context.Context, ids []int64) (int64, error) {
	ids = uniquePositive(ids)
	if len(ids) == 0 {
		return 0, nil
	}
	return m.engine.GraduateStudents(ctx, ids)
}

func (m *Module) ReactivateStudents(ctx context.Context, ids []int64, status string) ([]int64, error) {
	ids = uniquePositive(ids)
	if len(ids) == 0 {
		return []int64{}, nil
	}
	status = strings.TrimSpace(status)
	if status == "" || status == StudentStatusAlumnus {
		return nil, invalidStudent("target status must be a non-alumnus lifecycle status")
	}
	return m.engine.ReactivateStudents(ctx, ids, status)
}

func uniqueClasses(classes []string) []string {
	result := make([]string, 0, len(classes))
	seen := make(map[string]struct{}, len(classes))
	for _, class := range classes {
		if class == "" {
			continue
		}
		if _, ok := seen[class]; ok {
			continue
		}
		seen[class] = struct{}{}
		result = append(result, class)
	}
	return result
}

func invalidStudent(reason string) error { return &InvalidStudentError{Reason: reason} }
