package api

import (
	parentAPI "github.com/moto-nrw/project-phoenix/modules/careplan/inbound/parent"
	schoolPortal "github.com/moto-nrw/project-phoenix/modules/schoolportal"
	"github.com/moto-nrw/project-phoenix/services"
)

// The school and parents portals consume the password reset capability
// through a runtime of plain-typed closures the service root binds (#3332).
// The two mappings below carry that runtime across the portal boundary, so
// neither portal names the Identity & Access contract and the root does not
// name the portals.

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
