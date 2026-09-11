package api

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithTemporarySeedSettingRestoresAfterActionFailure(t *testing.T) {
	t.Parallel()

	var calls int
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, _ *seedHTTPRequest) {
		calls++
		if calls == 2 {
			w.WriteHeader(500)
			_, _ = fmt.Fprint(w, "restore failed")
			return
		}
		_, _ = fmt.Fprint(w, "{}")
	})
	defer srv.Close()

	rt := &Runtime{Client: newTestClient(srv.URL, false)}
	actionErr := errors.New("action failed")
	err := withTemporarySeedSetting(rt, AuthRef{}, "demo.key", "temporary", "restored", func() error {
		return actionErr
	})

	require.ErrorIs(t, err, actionErr)
	assert.ErrorContains(t, err, "restore demo.key")
	assert.Equal(t, 2, calls)
}
