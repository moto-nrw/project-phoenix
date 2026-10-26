package api

import (
	"bytes"
	"context"
	"image/png"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestSeedTraegerMasterDataStepKeepsTheBrandingThroughTheTraegerPortal(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var calls []string
	var logo []byte
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/traeger/auth/login":
			_, _ = w.Write([]byte(`{"status":"authenticated","access_token":"traeger-token","refresh_token":"r"}`))
			return
		case "/traeger/master-data/logo":
			assert.Equal(t, "Bearer traeger-token", r.Header.Get("Authorization"))
			_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			assert.NoError(t, err)
			part, err := multipart.NewReader(r.Body, params["boundary"]).NextPart()
			assert.NoError(t, err)
			assert.Equal(t, "logo", part.FormName())
			logo, _ = io.ReadAll(part)
		case "/traeger/master-data/":
			// The Träger already has one of the two bank accounts.
			_, _ = w.Write([]byte(`{"status":"success","data":{"bank_accounts":[{"label":"Ferienbetreuung"}]}}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"success"}`))
	})
	defer srv.Close()

	runtime := &Runtime{
		Client:    newTestClient(srv.URL, false),
		Bootstrap: &bootstrapSeedState{AdminEmail: "admin@example.test", AdminPassword: testpkg.NewTestPassword()},
	}
	require.NoError(t, (seedTraegerMasterDataStep{}).Run(context.Background(), runtime))
	assert.Equal(t, []string{
		"POST /traeger/auth/login",
		"PUT /traeger/settings/values/branding.primary_color",
		"PUT /traeger/settings/values/branding.letterhead_text",
		"PUT /traeger/settings/values/branding.footer_text",
		"PUT /traeger/master-data/contact",
		"PUT /traeger/master-data/logo",
		"GET /traeger/master-data/",
		"POST /traeger/master-data/bank-accounts",
	}, calls, "only the missing bank account is added")
	_, err := png.Decode(bytes.NewReader(logo))
	assert.NoError(t, err, "the logo is a real PNG")
}

// The demo process seeds several schools of one Demo-Träger at once: another
// school adds the bank accounts between the read and the write. The label
// refusal leaves the Träger with that school's accounts.
func TestSeedTraegerBankAccountsKeepAnotherSchoolsAccounts(t *testing.T) {
	t.Parallel()

	posts := 0
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"status":"success","data":{"bank_accounts":[]}}`))
			return
		}
		posts++
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"status":400,"code":"traeger.bank_account_label_taken","detail":"the Träger already has a bank account with this label"}`))
	})
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	require.NoError(t, seedTraegerBankAccountsOnce(client, AuthRef{Kind: AuthBearer, Token: "traeger-token"}))
	assert.Equal(t, 2, posts, "both accounts were offered once")
}

// Any other refusal still fails the step.
func TestSeedTraegerBankAccountsFailOnOtherRefusals(t *testing.T) {
	t.Parallel()

	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"status":"success","data":{"bank_accounts":[]}}`))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"status":400,"code":"traeger.bank_account_iban_invalid","detail":"invalid IBAN"}`))
	})
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	require.ErrorContains(t, seedTraegerBankAccountsOnce(client, AuthRef{Kind: AuthBearer, Token: "traeger-token"}), "add the Träger bank account Elternbeiträge")
}
