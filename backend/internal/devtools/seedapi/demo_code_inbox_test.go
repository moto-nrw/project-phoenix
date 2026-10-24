package api

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDemoCodeInboxIsUnavailableOutsideDemo(t *testing.T) {
	t.Parallel()
	for _, environment := range []string{"", "production", "staging", "development", "demo-production"} {
		inbox, err := NewDemoCodeInbox(environment)
		require.Error(t, err)
		require.Nil(t, inbox)
	}
}

func TestDemoCodeInboxReadsOnlyFreshRecipientCodesInProcess(t *testing.T) {
	t.Parallel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	inbox := startDemoCodeInbox(listener)
	t.Cleanup(func() { require.NoError(t, inbox.Close()) })
	since := time.Now()
	conn, err := net.Dial("tcp", listener.Addr().String())
	require.NoError(t, err)
	require.NoError(t, conn.SetDeadline(time.Now().Add(5*time.Second)))
	require.NoError(t, json.NewEncoder(conn).Encode(demoCode{Recipient: "demo@example.test", Code: "123456"}))
	response, err := io.ReadAll(conn)
	require.NoError(t, err)
	require.NoError(t, conn.Close())
	require.Equal(t, []byte{1}, response, "wire response acknowledges delivery and never contains a code")
	_, err = inbox.FetchCode(context.Background(), "other@example.test", since)
	require.ErrorIs(t, err, errDemoCodeUnavailable)
	_, err = inbox.FetchCode(context.Background(), "demo@example.test", time.Now().Add(time.Minute))
	require.ErrorIs(t, err, errDemoCodeUnavailable)
	code, err := inbox.FetchCode(context.Background(), "demo@example.test", since)
	require.NoError(t, err)
	require.Equal(t, "123456", code)
	_, err = inbox.FetchCode(context.Background(), "demo@example.test", since)
	require.ErrorIs(t, err, errDemoCodeUnavailable, "codes cannot be read twice")
}
