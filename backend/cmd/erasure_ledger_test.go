package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/erasureledger"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/stretchr/testify/require"
)

const receipt = `{"school_id":17,"order_reference":"ORDER-RESTORE","status":"completed","completed_at":"2026-10-24T12:00:00Z"}`

func TestLedgerDurabilityIdempotenceAndDamage(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ledger")
	ledger := erasureledger.New(path, securityruntime.Fingerprint)
	require.Error(t, ledger.Check(nil, ""))
	require.Error(t, ledger.Record(json.RawMessage(receipt)))
	require.NoError(t, erasureledger.Initialize(path))
	require.Error(t, erasureledger.Initialize(path))
	require.NoError(t, ledger.Record(json.RawMessage(receipt)))
	before, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, 2, strings.Count(string(before), "\n"))
	require.NotContains(t, string(before), `"status"`)
	require.NoError(t, ledger.Record(json.RawMessage(receipt)))
	after, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, before, after)
	for _, broken := range [][]byte{nil, before[:len(before)-1], []byte(strings.Replace(string(before), "ORDER-RESTORE", "ORDER-CHANGED", 1))} {
		require.NoError(t, os.WriteFile(path, broken, 0600))
		require.Error(t, ledger.Check(nil, ""))
		require.Error(t, ledger.Record(json.RawMessage(receipt)))
	}
}

func TestLedgerRejectsUnsafePathsAndIncompleteReceipts(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ledger")
	require.NoError(t, erasureledger.Initialize(path))
	ledger := erasureledger.New(path, securityruntime.Fingerprint)
	require.Error(t, ledger.Record(json.RawMessage(strings.Replace(receipt, "completed", "failed", 1))))
	require.NoError(t, os.Chmod(path, 0644))
	require.Error(t, ledger.Check(nil, ""))
	require.NoError(t, os.Chmod(path, 0600))
	link := path + "-link"
	require.NoError(t, os.Symlink(path, link))
	require.Error(t, erasureledger.New(link, securityruntime.Fingerprint).Check(nil, ""))
	require.Error(t, erasureledger.New("relative-ledger", securityruntime.Fingerprint).Check(nil, ""))
}

func TestLedgerConcurrentWritersNeverCorruptList(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ledger")
	require.NoError(t, erasureledger.Initialize(path))
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() { _ = erasureledger.New(path, securityruntime.Fingerprint).Record(json.RawMessage(receipt)) })
	}
	wg.Wait()
	// Failed lock contenders retry; success and retries produce one record.
	require.NoError(t, erasureledger.New(path, securityruntime.Fingerprint).Record(json.RawMessage(receipt)))
	require.NoError(t, erasureledger.New(path, securityruntime.Fingerprint).Check(nil, ""))
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, 2, strings.Count(string(data), "\n"))
}
