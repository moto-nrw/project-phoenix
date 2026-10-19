package application

import (
	identityBindingPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
)

// operatorLoginGate is the gate the operator login consults; the demo
// environment may leave it unconfigured (DemoDependencies.OperatorWithoutSecondFactor).
func OperatorLoginGate(flows MFAFlows, demo *identityBindingPorts.DemoDependencies) OperatorMFAGate {
	if demo != nil && demo.OperatorWithoutSecondFactor {
		return OperatorMFAGate{}
	}
	return OperatorMFAGate{Flows: flows.Operator}
}
