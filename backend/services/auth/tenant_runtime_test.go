package auth

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	userModel "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/uptrace/bun"
)

// identityStub serves the school identity port over the given repositories:
// the stub walks the person -> staff -> caregiver chain the way the owner
// module does, without transponders, import hints or the child refusal.
func identityStub(persons userModel.PersonRepository, staff userModel.StaffRepository, teachers userModel.TeacherRepository) *stubAccountSessions {
	return &stubAccountSessions{repos: &repositories.Factory{Person: persons, Staff: staff, Teacher: teachers}}
}

func newMockTenantRuntime(t *testing.T, db *bun.DB) *tenant.UnitOfWork {
	t.Helper()
	runtime := testpkg.TenantRuntime(t, db)
	return &runtime
}

func newTestInvitationService(t *testing.T, config InvitationServiceConfig) InvitationService {
	t.Helper()
	if config.DB != nil {
		service := NewInvitationService(config)
		service.(interface{ SetTenantRuntime(tenant.UnitOfWork) }).SetTenantRuntime(*newMockTenantRuntime(t, config.DB))
		return service
	}
	return NewInvitationService(config)
}
