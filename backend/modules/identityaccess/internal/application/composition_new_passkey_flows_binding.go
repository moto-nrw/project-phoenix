package application

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
	restoredPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
)

func NewPasskeyFlowsBinding(flows MFAFlows, service *Service, auth *AccountAuthentication, operatorAuth *OperatorAuthentication, accountPasskeys *AccountPasskey, operatorPasskeys *OperatorPasskey, deps *restoredPorts.MFADependencies, runtimeFactory func() ports.Runtime) (MFAFlows, error) {
	if deps == nil || deps.Passkeys == nil {
		return flows, nil
	}
	if auth == nil || operatorAuth == nil {
		return MFAFlows{}, errors.New("identity access compose: the passkey ceremonies require the session and operator flows")
	}
	runtime := runtimeFactory()
	accountFlows, err := NewAccountPasskeyFlows(AccountPasskeyFlowDependencies{
		Accounts: flows.Records, Records: accountPasskeys, MFA: flows.Account,
		Sessions: auth, Runtime: runtime, RPID: deps.Passkeys.RPID, RPName: deps.Passkeys.RPName,
		TenantDomain: deps.Passkeys.TenantDomain,
	})
	if err != nil {
		return MFAFlows{}, err
	}
	operatorFlows, err := NewOperatorPasskeyFlows(service, OperatorPasskeyFlowDependencies{
		Records: operatorPasskeys, MFA: flows.Operator, Sessions: operatorAuth, Runtime: runtime,
		RPID: deps.Passkeys.RPID, RPName: deps.Passkeys.RPName,
		OperatorFrontendURL: deps.Passkeys.OperatorFrontendURL,
	})
	if err != nil {
		return MFAFlows{}, err
	}
	// The ceremonies are carried in a new value rather than written into the
	// composed one: the composition surface stays free of mutable wiring.
	return MFAFlows{
		Records: flows.Records, Account: flows.Account, Capability: flows.Capability, Operator: flows.Operator,
		AccountPasskey: accountFlows, OperatorPasskey: operatorFlows,
	}, nil
}
