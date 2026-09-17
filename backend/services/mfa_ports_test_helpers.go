package services

import (
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/services/auth"
)

// The retained MFA and passkey ports are what the HTTP surfaces classify on
// until the handlers move (#3230, #3231), so the error translation they carry
// is behaviour worth pinning. These helpers serve the ports over a supplied
// capability, the way the factory serves them over the composed module.

// NewAccountMFAPortForTests serves the retained second factor over module.
func NewAccountMFAPortForTests(module identityaccess.AccountMFA) auth.MFAService {
	return newAccountMFAPort(module)
}

// NewAccountPasskeyPortForTests serves the retained school-portal ceremonies
// over module.
func NewAccountPasskeyPortForTests(module identityaccess.AccountPasskeyFlows) auth.PasskeyService {
	return newAccountPasskeyPort(module)
}

// NewModuleAccountMFAForTests adapts a retained second factor back to the
// module's capability, as the test module's gate seam does.
func NewModuleAccountMFAForTests(port auth.MFAService) identityaccess.AccountMFA {
	return newModuleAccountMFA(port)
}
