package staffmessaging_test

import (
	"fmt"
	"net/http"
	"sync"
	"testing"

	staffmessaging "github.com/moto-nrw/project-phoenix/modules/communication/http/staffmessages"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestStaffSendConcurrentRetriesAndAccountScope(t *testing.T) {
	t.Parallel()
	f := setupGroupFixture(t)
	a, b := f.person(t, "Anna", "staff"), f.person(t, "Ben", "staff")
	opened := f.do(t, a, http.MethodPost, "/threads/open", fmt.Sprintf(`{"account_id":%q}`, b.idString()))
	require.Equal(t, http.StatusOK, opened.Code, opened.Body.String())
	thread := decodeData[staffmessaging.ThreadDetailResponse](t, opened.Body.Bytes())
	path := "/threads/" + thread.ThreadID
	const input = `{"body":"Hallo","client_message_id":"concurrent-send"}`
	// Start overlapping requests in separate transactions. Assert through the API.
	var wg sync.WaitGroup
	results := make(chan string, 4)
	for range 4 {
		wg.Go(func() {
			response := f.do(t, a, http.MethodPost, path, input)
			if response.Code != http.StatusOK {
				results <- response.Body.String()
				return
			}
			results <- decodeData[staffmessaging.MessageResponse](t, response.Body.Bytes()).ID
		})
	}
	wg.Wait()
	close(results)
	var first string
	for id := range results {
		if first == "" {
			first = id
		}
		require.Equal(t, first, id)
	}
	require.NotEmpty(t, first)
	conflict := f.do(t, a, http.MethodPost, path, `{"body":"Geändert","client_message_id":"concurrent-send"}`)
	require.Equal(t, http.StatusConflict, conflict.Code, conflict.Body.String())
	// Identical opaque keys of a different account do not share a message.
	other := f.do(t, b, http.MethodPost, path, input)
	require.Equal(t, http.StatusOK, other.Code, other.Body.String())
	require.NotEqual(t, first, decodeData[staffmessaging.MessageResponse](t, other.Body.Bytes()).ID)
	read := f.do(t, a, http.MethodGet, path, "")
	require.Equal(t, http.StatusOK, read.Code, read.Body.String())
	messages := decodeData[staffmessaging.ThreadDetailResponse](t, read.Body.Bytes()).Messages
	require.Len(t, messages, 2)
	require.Equal(t, "concurrent-send", messages[0].ClientMessageID)
	// The same account in a different school owns an independent namespace.
	second := setupGroupFixture(t)
	testpkg.CreateTestStaffForExistingAccountInTenant(t, second.db, second.tenantID, a.id, "Anna", "Zweite Schule")
	otherA := a
	otherA.claims.TenantID = second.tenantID
	otherB := second.person(t, "David", "staff")
	openSecond := second.do(t, otherA, http.MethodPost, "/threads/open", fmt.Sprintf(`{"account_id":%q}`, otherB.idString()))
	require.Equal(t, http.StatusOK, openSecond.Code, openSecond.Body.String())
	secondThread := decodeData[staffmessaging.ThreadDetailResponse](t, openSecond.Body.Bytes())
	sent := second.do(t, otherA, http.MethodPost, "/threads/"+secondThread.ThreadID, input)
	require.Equal(t, http.StatusOK, sent.Code, sent.Body.String())
	require.NotEqual(t, first, decodeData[staffmessaging.MessageResponse](t, sent.Body.Bytes()).ID)
	forbidden := second.do(t, otherA, http.MethodPost, path, input)
	require.Equal(t, http.StatusNotFound, forbidden.Code, forbidden.Body.String())
}
