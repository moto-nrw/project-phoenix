package owners

import (
	"context"
	"io"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
)

// StaffStudentReads binds Communication's student read port to the Security
// Runtime's rules over the caller context: an administrator or verified staff
// of the school reads every child. Without a caller only the administrator
// does, which is what the messaging suites build.
type StaffStudentReads struct {
	Caller interface {
		HasCurrentStaff(context.Context) (bool, error)
		MyGroupIDs(context.Context) ([]int64, error)
	}
}

// ReadsAllStudents reports whether the caller reads children tenant-wide.
func (r StaffStudentReads) ReadsAllStudents(ctx context.Context, permissions []string) bool {
	return securityruntime.ResolveStudentReadScope(ctx, permissions, r.Caller)
}

// CanReadStudent reports whether the caller may read one child.
func (r StaffStudentReads) CanReadStudent(ctx context.Context, permissions []string, student interface{ IsAuthorizationStudent() bool }) bool {
	return securityruntime.CanReadStudent(ctx, permissions, student, r.Caller)
}

// ReadsLeadershipMessages reports whether the caller reads the conversations
// parents address to the leadership only (#5200): parent_messages:leadership
// through a role, or admin:*.
func (r StaffStudentReads) ReadsLeadershipMessages(_ context.Context, perms []string) bool {
	return securityruntime.HasPermission(permissions.ParentMessagesLeadership, perms)
}

// MyGroupIDs are the caller's OGS groups.
func (r StaffStudentReads) MyGroupIDs(ctx context.Context) ([]int64, error) {
	return r.Caller.MyGroupIDs(ctx)
}

// SecurityFingerprints binds Communication's declaration fingerprints to the
// Security Runtime's SHA-256 content identity.
type SecurityFingerprints struct{}

// Content is the identity of an in-memory canonical content or record.
func (SecurityFingerprints) Content(content []byte) string {
	return securityruntime.Fingerprint(content)
}

// Reader streams one attachment into the identity.
func (SecurityFingerprints) Reader(r io.Reader) (string, int64, error) {
	return securityruntime.ReaderFingerprint(r)
}
