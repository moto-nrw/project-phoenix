package postgres

import (
	context "context"
	time "time"

	domain "github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/domain"
)

type JournalRetention struct {
	Journal *Store
	Now     func() time.Time
}

func (r JournalRetention) DeleteExpiredTransfers(ctx context.Context) (int, int, error) {
	now := r.Now()
	cutoff := domain.JournalRetentionCutoff(now)
	deleted, err := r.Journal.DeleteExpired(ctx, cutoff)
	if err != nil {
		return 0, 0, err
	}
	return deleted, int(now.Sub(cutoff).Hours() / 24), nil
}
