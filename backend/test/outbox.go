package test

import "github.com/moto-nrw/project-phoenix/models/platform/platformtest"

// The outbox test double lives with the outbox port's owner, Organisation &
// Tenancy (platformtest); these aliases keep the shared fixture names.
type CapturingOutbox = platformtest.CapturingOutbox

var NewCapturingOutbox = platformtest.NewCapturingOutbox
