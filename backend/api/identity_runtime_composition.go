package api

import (
	enrollmentAPI "github.com/moto-nrw/project-phoenix/api/enrollment"
	parentAPI "github.com/moto-nrw/project-phoenix/modules/careplan/inbound/parent"
	schoolPortal "github.com/moto-nrw/project-phoenix/modules/schoolportal"
	"github.com/moto-nrw/project-phoenix/services"
)

// The portals and the enrollment routes consume Identity & Access through
// runtimes of plain-typed closures the service root binds (#3332). The
// mappings below carry those runtimes across the adapter boundary, so no
// adapter names the owner's contract and the root does not name the
// adapters.

func schoolPasswordResets(runtime services.PasswordResetRuntime) schoolPortal.PasswordResetRuntime {
	return schoolPortal.PasswordResetRuntime{
		Initiate:     runtime.Initiate,
		Reset:        runtime.Reset,
		LinkUnusable: runtime.LinkUnusable,
		TooWeak:      runtime.TooWeak,
		RetryAfter:   runtime.RetryAfter,
	}
}

func parentPasswordResets(runtime services.PasswordResetRuntime) parentAPI.PasswordResetRuntime {
	return parentAPI.PasswordResetRuntime{
		Initiate:     runtime.Initiate,
		Reset:        runtime.Reset,
		LinkUnusable: runtime.LinkUnusable,
		TooWeak:      runtime.TooWeak,
		RetryAfter:   runtime.RetryAfter,
	}
}

// The enrollment decision routes fire a guardian invitation after an
// approval. They may not name the Identity & Access contract either, so the
// root hands them the one call they make (#3332).
func enrollmentGuardianInvitations(invitations services.GuardianInvitationCapability) enrollmentAPI.GuardianInvitationRuntime {
	runtime := services.EnrollmentGuardianInvitationRuntime(invitations)
	return enrollmentAPI.GuardianInvitationRuntime{Create: runtime.Create}
}
