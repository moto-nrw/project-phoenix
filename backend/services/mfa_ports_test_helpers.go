package services

import (
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/services/platform"
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

// NewOperatorMFAPortForTests serves the retained operator second factor over
// module.
func NewOperatorMFAPortForTests(module identityaccess.OperatorMFAFlows) platform.OperatorMFAService {
	return newOperatorMFAPort(module)
}

// NewOperatorPasskeyPortForTests serves the retained operator ceremonies over
// module.
func NewOperatorPasskeyPortForTests(module identityaccess.OperatorPasskeyFlows) platform.OperatorPasskeyService {
	return newOperatorPasskeyPort(module)
}

// NewModuleAccountMFAForTests adapts a retained second factor back to the
// module's capability, as the test module's gate seam does.
func NewModuleAccountMFAForTests(port auth.MFAService) identityaccess.AccountMFA {
	return newModuleAccountMFA(port)
}
