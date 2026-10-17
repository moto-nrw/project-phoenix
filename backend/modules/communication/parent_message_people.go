package communication

import "context"

// ParentMessageStudent is the exact student-eligibility projection consumed by
// messaging. The People Directory reader supplies its canonical facts.
type ParentMessageStudent struct {
	Alumnus              bool
	AuthorizationStudent bool
}

func (s *ParentMessageStudent) IsAlumnus() bool { return s != nil && s.Alumnus }
func (s *ParentMessageStudent) IsAuthorizationStudent() bool {
	return s != nil && s.AuthorizationStudent
}

type ParentMessagePeople interface {
	FindParentMessageStudent(context.Context, int64) (*ParentMessageStudent, error)
	StaffSenderNames
}
type parentMessagePeople struct {
	student func(context.Context, int64) (*ParentMessageStudent, error)
	names   StaffSenderNames
}

func NewParentMessagePeople(student func(context.Context, int64) (*ParentMessageStudent, error), names StaffSenderNames) ParentMessagePeople {
	return parentMessagePeople{student: student, names: names}
}
func (r parentMessagePeople) FindParentMessageStudent(ctx context.Context, id int64) (*ParentMessageStudent, error) {
	return r.student(ctx, id)
}
func (r parentMessagePeople) FindStaffSenderName(ctx context.Context, id int64) (*StaffSenderName, error) {
	return r.names.FindStaffSenderName(ctx, id)
}

// ParentMessageGuardianContact retains the original mail/name/locale facts.
type ParentMessageGuardianContact struct {
	FirstName    string
	LastName     string
	Email        *string
	PortalLocale *string
}
type ParentMessageGuardianProfiles interface {
	FindMessageGuardianProfile(context.Context, int64) (*ParentMessageGuardianContact, error)
}
type parentMessageGuardianProfiles func(context.Context, int64) (*ParentMessageGuardianContact, error)

func NewParentMessageGuardianProfiles(read func(context.Context, int64) (*ParentMessageGuardianContact, error)) ParentMessageGuardianProfiles {
	return parentMessageGuardianProfiles(read)
}
func (read parentMessageGuardianProfiles) FindMessageGuardianProfile(ctx context.Context, id int64) (*ParentMessageGuardianContact, error) {
	return read(ctx, id)
}
