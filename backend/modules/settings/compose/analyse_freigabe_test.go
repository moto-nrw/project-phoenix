package compose

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

type freigabeReader struct {
	value bool
	err   error
	keys  []string
}

func (r *freigabeReader) ResolveBoolForTenant(_ context.Context, _ int64, key string) (bool, error) {
	r.keys = append(r.keys, key)
	return r.value, r.err
}

func (*freigabeReader) ResolveIntForTenant(context.Context, int64, string) (int, error) {
	return 0, errors.New("not read")
}

func (*freigabeReader) ResolveStringForTenant(context.Context, int64, string) (string, error) {
	return "", errors.New("not read")
}

func TestNewAnalyseFreigabeReadsTheSchoolSetting(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.DiscardHandler)

	on := &freigabeReader{value: true}
	assert.True(t, NewAnalyseFreigabe(on, logger)(context.Background(), 42))
	assert.Equal(t, []string{"analytics.freigabe"}, on.keys)

	off := &freigabeReader{value: false}
	assert.False(t, NewAnalyseFreigabe(off, logger)(context.Background(), 42))
}

// A settings error must never make a school's staff a person.
func TestNewAnalyseFreigabeFailsClosed(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.DiscardHandler)
	broken := &freigabeReader{value: true, err: errors.New("settings unavailable")}

	assert.False(t, NewAnalyseFreigabe(broken, logger)(context.Background(), 42))
}
