package compose

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/exporttransfer"
	"github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/adapters/postgres"
)

// NewJournalRetention composes the journal retention of the nightly cleanup
// (#4044). It runs on the tenant transaction of the school in context and
// needs neither the settings nor the transport.
func NewJournalRetention() exporttransfer.JournalRetention {
	return postgres.JournalRetention{Journal: postgres.New(retainedDatabase()), Now: time.Now}
}
