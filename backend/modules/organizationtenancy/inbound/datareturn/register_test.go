package datareturn_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy"
	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy/inbound/datareturn"
	"github.com/stretchr/testify/require"
)

const goodToken = "mdr_good"

// fakePackages opens one package for goodToken and nothing else.
type fakePackages struct {
	downloads int
	broken    bool
}

var released = organizationtenancy.DataReturnPackage{Package: "school", FormatVersion: 1, ManifestSHA256: "ab", Recipient: "target_provider",
	ArchiveSHA256: "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824", ArchiveBytes: 5,
	ExpiresAt: time.Date(2026, 11, 24, 9, 0, 0, 0, time.UTC), DownloadsLeft: 3}

func (f *fakePackages) Package(_ context.Context, token string) (organizationtenancy.DataReturnPackage, error) {
	if token != goodToken {
		return organizationtenancy.DataReturnPackage{}, organizationtenancy.ErrDataReturnDenied
	}
	return released, nil
}

func (f *fakePackages) Download(_ context.Context, token string, start func(organizationtenancy.DataReturnPackage) io.Writer) error {
	if token != goodToken {
		return organizationtenancy.ErrDataReturnDenied
	}
	if f.broken {
		return organizationtenancy.ErrDataReturnArchiveMissing
	}
	f.downloads++
	_, err := start(released).Write([]byte("hello"))
	return err
}

func serve(t *testing.T, router http.Handler, path, authorization string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestRegisterAndOpenAPIAreServedWithoutSession(t *testing.T) {
	t.Parallel()
	router := datareturn.NewResource(&fakePackages{}, nil).Router()

	rec := serve(t, router, "/register", "")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))
	var register organizationtenancy.ReturnRegister
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &register))
	require.Equal(t, organizationtenancy.ExportFormatVersion, register.FormatVersion)
	require.NotEmpty(t, register.Tables)
	require.NotEmpty(t, register.Standards)

	rec = serve(t, router, "/openapi.json", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var spec struct {
		OpenAPI string                    `json:"openapi"`
		Paths   map[string]map[string]any `json:"paths"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &spec))
	require.Equal(t, "3.1.0", spec.OpenAPI)
	for _, path := range []string{"/register", "/openapi.json", "/package", "/package/archive"} {
		require.Contains(t, spec.Paths[path], "get", "the OpenAPI description documents GET %s", path)
	}
}

func TestPackageOpensOnlyWithItsBearerToken(t *testing.T) {
	t.Parallel()
	packages := &fakePackages{}
	router := datareturn.NewResource(packages, nil).Router()

	for _, authorization := range []string{"", "Bearer mdr_other", "Basic " + goodToken, goodToken} {
		for _, path := range []string{"/package", "/package/archive"} {
			rec := serve(t, router, path, authorization)
			require.Equal(t, http.StatusUnauthorized, rec.Code, "%s with %q", path, authorization)
			require.Equal(t, `Bearer realm="moto-data-return"`, rec.Header().Get("WWW-Authenticate"))
			require.JSONEq(t, `"general.permission"`, string(mustField(t, rec.Body.Bytes(), "code")))
		}
	}
	// A token in the query string is no credential.
	require.Equal(t, http.StatusUnauthorized, serve(t, router, "/package/archive?token="+goodToken, "").Code)
	require.Zero(t, packages.downloads)

	rec := serve(t, router, "/package", "Bearer "+goodToken)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	var got organizationtenancy.DataReturnPackage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, released, got)
	require.Zero(t, packages.downloads, "describing a package spends no download")

	rec = serve(t, router, "/package/archive", "bearer "+goodToken)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "application/zip", rec.Header().Get("Content-Type"))
	require.Equal(t, "5", rec.Header().Get("Content-Length"))
	require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	require.Equal(t, "sha-256=:LPJNul+wow4m6DsqxbninhsWHlwfp0JecwQzYpOLmCQ=:", rec.Header().Get("Repr-Digest"))
	require.Equal(t, "hello", rec.Body.String())
	require.Equal(t, 1, packages.downloads)
}

func TestMissingArchiveAnswersUnavailable(t *testing.T) {
	t.Parallel()
	router := datareturn.NewResource(&fakePackages{broken: true}, nil).Router()
	rec := serve(t, router, "/package/archive", "Bearer "+goodToken)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.JSONEq(t, `"general.unavailable"`, string(mustField(t, rec.Body.Bytes(), "code")))
}

func TestPackageRoutesRunBehindTheRateLimit(t *testing.T) {
	t.Parallel()
	limited := 0
	limit := func(http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			limited++
			w.WriteHeader(http.StatusTooManyRequests)
		})
	}
	router := datareturn.NewResource(&fakePackages{}, limit).Router()
	require.Equal(t, http.StatusTooManyRequests, serve(t, router, "/package", "Bearer "+goodToken).Code)
	require.Equal(t, http.StatusTooManyRequests, serve(t, router, "/package/archive", "Bearer "+goodToken).Code)
	require.Equal(t, http.StatusOK, serve(t, router, "/register", "").Code)
	require.Equal(t, 2, limited)
}

func mustField(t *testing.T, body []byte, field string) json.RawMessage {
	t.Helper()
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &fields))
	return fields[field]
}
