package compose

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/exporttransfer"
	"github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/domain"
)

// NewJournalRetention composes the journal retention of the nightly cleanup
// (#4044). It runs on the tenant transaction of the school in context and
// needs neither the settings nor the transport.
func NewJournalRetention() exporttransfer.JournalRetention {
	return journalRetention{journal: postgres.New(database()), now: time.Now}
}

type journalRetention struct {
	journal *postgres.Store
	now     func() time.Time
}

func (r journalRetention) DeleteExpiredTransfers(ctx context.Context) (int, int, error) {
	now := r.now()
	cutoff := domain.JournalRetentionCutoff(now)
	deleted, err := r.journal.DeleteExpired(ctx, cutoff)
	if err != nil {
		return 0, 0, err
	}
	return deleted, int(now.Sub(cutoff).Hours() / 24), nil
}
