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
