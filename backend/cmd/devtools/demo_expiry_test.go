package main

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
// standing school, tries a failed hiding again, erases the visitors of the
// schools out of use with every run (#4048) and last the uploads of the
// retired schools (#5544).

type fakeDemoExpiryRuntime struct {
	expired []struct {
		deleted  int
		orphaned []string
	}
	expiries int
	retired  [][]string
	retire   error
	// calls is the order of the steps: "expire", "retire", "files".
	calls  []string
	erased int
	files  error
}

func (r *fakeDemoExpiryRuntime) ExpireDemoAccesses(context.Context) (int, []string, error) {
	r.expiries++
	r.calls = append(r.calls, "expire")
	if len(r.expired) == 0 {
		return 0, nil, nil
	}
	next := r.expired[0]
	r.expired = r.expired[1:]
	return next.deleted, next.orphaned, nil
}

func (r *fakeDemoExpiryRuntime) RetireDemoSchools(_ context.Context, slugs []string) (int, int, error) {
	r.calls = append(r.calls, "retire")
	if r.retire != nil {
		return 0, 0, r.retire
	}
	if len(slugs) > 0 {
		r.retired = append(r.retired, slugs)
	}
	return len(slugs), r.erased, nil
}

func (r *fakeDemoExpiryRuntime) EraseRetiredDemoSchoolFiles(context.Context) (int, int64, error) {
	r.calls = append(r.calls, "files")
	if r.files != nil {
		return 0, 0, r.files
	}
	return 1, 3, nil
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

// Every run retires, with or without a school to hide: the retirement also
// erases the visitors of the demo schools out of use (#4048), and a failed
// order has no school to hide.
func TestDemoExpiryRetiresEveryRunToEraseTheVisitors(t *testing.T) {
	t.Parallel()
	clock := time.Date(2026, 9, 22, 8, 0, 0, 0, time.UTC)
	runtime := &fakeDemoExpiryRuntime{expired: []struct {
		deleted  int
		orphaned []string
	}{
		{deleted: 1, orphaned: []string{"ogs-nord-k3m9xp"}},
	}, erased: 1}
	expiry := newDemoExpiry(runtime, func() time.Time { return clock })

	require.NoError(t, expiry.run(t.Context()))
	clock = clock.Add(time.Hour)
	require.NoError(t, expiry.run(t.Context()))

	assert.Equal(t, []string{"expire", "retire", "files", "expire", "retire", "files"}, runtime.calls)
	assert.Equal(t, [][]string{{"ogs-nord-k3m9xp"}}, runtime.retired)
}

// The uploads of retired schools go last, after the visitors (#5544): a
// failed hiding or visitor erasure leaves the files for the next run.
func TestDemoExpiryErasesTheFilesAfterTheRetirement(t *testing.T) {
	t.Parallel()
	runtime := &fakeDemoExpiryRuntime{retire: errors.New("database is away")}
	expiry := newDemoExpiry(runtime, time.Now)

	require.Error(t, expiry.run(t.Context()))
	assert.Equal(t, []string{"expire", "retire"}, runtime.calls, "no file erasure before the retirement succeeded")

	runtime.retire = nil
	runtime.calls = nil
	require.NoError(t, expiry.run(t.Context()))
	assert.Equal(t, []string{"expire", "retire", "files"}, runtime.calls)
}

// A school whose files could not be erased keeps the run due: the next poll
// tries again instead of waiting an hour (#5544).
func TestDemoExpiryRunsAgainAfterAFailedFileErasure(t *testing.T) {
	t.Parallel()
	clock := time.Date(2026, 9, 22, 8, 0, 0, 0, time.UTC)
	runtime := &fakeDemoExpiryRuntime{files: errors.New("storage: unsafe namespace")}
	expiry := newDemoExpiry(runtime, func() time.Time { return clock })

	require.Error(t, expiry.run(t.Context()))
	runtime.files = nil
	clock = clock.Add(time.Second)
	require.NoError(t, expiry.run(t.Context()))
	clock = clock.Add(time.Second)
	require.NoError(t, expiry.run(t.Context()))

	assert.Equal(t, []string{"expire", "retire", "files", "expire", "retire", "files"}, runtime.calls,
		"the failed run is due again at the next poll, the successful one waits an hour")
}
