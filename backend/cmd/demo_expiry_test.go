package cmd

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The expiry of the public demo (#3470) on an injected clock: it runs at
// once, then once per hour, hides the schools left without a link, never the
// standing school, and tries a failed hiding again.

type fakeDemoExpiryRuntime struct {
	expired []struct {
		deleted  int
		orphaned []string
	}
	expiries int
	retired  [][]string
	retire   error
}

func (r *fakeDemoExpiryRuntime) ExpireDemoAccesses(context.Context) (int, []string, error) {
	r.expiries++
	if len(r.expired) == 0 {
		return 0, nil, nil
	}
	next := r.expired[0]
	r.expired = r.expired[1:]
	return next.deleted, next.orphaned, nil
}

func (r *fakeDemoExpiryRuntime) RetireDemoSchools(_ context.Context, slugs []string) (int, error) {
	if r.retire != nil {
		return 0, r.retire
	}
	r.retired = append(r.retired, slugs)
	return len(slugs), nil
}

func TestDemoExpiryRunsAtOnceAndThenHourly(t *testing.T) {
	t.Parallel()
	clock := time.Date(2026, 9, 22, 8, 0, 0, 0, time.UTC)
	runtime := &fakeDemoExpiryRuntime{}
	expiry := newDemoExpiry(runtime, func() time.Time { return clock })

	require.NoError(t, expiry.run(t.Context()))
	assert.Equal(t, 1, runtime.expiries, "the first poll expires at once")
	clock = clock.Add(59 * time.Minute)
	require.NoError(t, expiry.run(t.Context()))
	assert.Equal(t, 1, runtime.expiries, "within the hour nothing runs")
	clock = clock.Add(time.Minute)
	require.NoError(t, expiry.run(t.Context()))
	assert.Equal(t, 2, runtime.expiries)
	assert.Empty(t, runtime.retired, "without a school left behind nothing is hidden")
}

func TestDemoExpiryHidesOrphanedSchoolsButNeverTheStandingOne(t *testing.T) {
	t.Parallel()
	runtime := &fakeDemoExpiryRuntime{expired: []struct {
		deleted  int
		orphaned []string
	}{
		{deleted: 3, orphaned: []string{"ogs-nord-k3m9xp", standingDemoSlug, "ogs-sued-a1b2c3"}},
	}}
	expiry := newDemoExpiry(runtime, time.Now)

	require.NoError(t, expiry.run(t.Context()))

	assert.Equal(t, [][]string{{"ogs-nord-k3m9xp", "ogs-sued-a1b2c3"}}, runtime.retired)
}

func TestDemoExpiryTriesAFailedHidingAgain(t *testing.T) {
	t.Parallel()
	clock := time.Date(2026, 9, 22, 8, 0, 0, 0, time.UTC)
	runtime := &fakeDemoExpiryRuntime{
		expired: []struct {
			deleted  int
			orphaned []string
		}{
			{deleted: 1, orphaned: []string{"ogs-nord-k3m9xp"}},
			{deleted: 1, orphaned: []string{"ogs-sued-a1b2c3"}},
		},
		retire: errors.New("database is away"),
	}
	expiry := newDemoExpiry(runtime, func() time.Time { return clock })

	require.Error(t, expiry.run(t.Context()))
	// The accesses are gone; the school must not be forgotten. The failed
	// run is due again at the next poll, not in an hour.
	runtime.retire = nil
	clock = clock.Add(time.Second)
	require.NoError(t, expiry.run(t.Context()))

	assert.Equal(t, 2, runtime.expiries)
	assert.Equal(t, [][]string{{"ogs-nord-k3m9xp", "ogs-sued-a1b2c3"}}, runtime.retired, "the remembered school is hidden with the new one")
}
