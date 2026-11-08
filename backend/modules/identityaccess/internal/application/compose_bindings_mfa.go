package application

import (
	identityBindingPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
)

// OperatorLoginGate is the gate the operator login consults: the composed
// operator flows. Operator MFA is mandatory; the demo environment may leave
// it unconfigured (DemoDependencies.OperatorWithoutSecondFactor), and an
// unconfigured gate issues the token pair directly.
func OperatorLoginGate(flows MFAFlows, demo *identityBindingPorts.DemoDependencies) *OperatorMFAFlows {
	if demo != nil && demo.OperatorWithoutSecondFactor {
		return nil
	}
	return flows.Operator
}
