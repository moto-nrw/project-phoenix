package postgres_test

import (
	"context"
	"testing"
	"time"

	ownerCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestChildIdentityLockUsesTheUnicodeNameMatchKey(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store := ownerCompose.New()
	birthday := calendar.NewDate(2016, time.March, 14)
	require.NoError(t, testpkg.WithTenantTx(t, t.Context(), db, testpkg.Tenant(t), func(ctx context.Context, tx bun.Tx) error {
		for _, name := range [][2]string{{" JÖRG ", "MÜLLER"}, {"Jörg", "Müller"}, {"Jo\u0308rg", "Mu\u0308ller"}} {
			require.NoError(t, store.AcquireChildIdentityLock(ctx, name[0], name[1], birthday))
		}
		locks := func() int {
			var count int
			require.NoError(t, tx.NewRaw(`SELECT count(*) FROM pg_locks
				WHERE pid = pg_backend_pid() AND locktype = 'advisory' AND classid = ?`, int64(0x656e6964)).Scan(ctx, &count))
			return count
		}
		require.Equal(t, 1, locks(), "case and NFC variants must serialize the same child")
		require.NoError(t, store.AcquireChildIdentityLock(ctx, "Jorg", "Muller", birthday))
		require.Equal(t, 2, locks(), "accents still distinguish identities")
		require.NoError(t, store.AcquireChildIdentityLock(ctx, "Jörg", "Müller", birthday.AddDays(1)))
		require.Equal(t, 3, locks(), "the birthday still distinguishes identities")
		return nil
	}))
}
