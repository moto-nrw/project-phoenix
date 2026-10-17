package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/application/groups"
)

// publicGroupError reports err with School Structure's public sentinels and
// error types, keeping its text and causes.
func publicGroupError(err error) error { return groups.PublicGroupError(err) }
