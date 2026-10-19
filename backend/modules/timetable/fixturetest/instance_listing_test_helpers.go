//go:build testhooks

package fixturetest

import (
	context "context"

	ports "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

type InstanceListing interface {
	ListInstances(arg0 context.Context, arg1 ports.LegacyInstanceFilter) ([]*ActivityInstance, error)
}
