package api

import (
	"context"
	"errors"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type seedHTTPRequest = testpkg.HTTPRequest
type seedHTTPResponseWriter = testpkg.HTTPResponseWriter
type seedHTTPTestServer = testpkg.HTTPTestServer

const (
	seedHTTPMethodGet    = testpkg.HTTPMethodGet
	seedHTTPMethodPost   = testpkg.HTTPMethodPost
	seedHTTPMethodPut    = testpkg.HTTPMethodPut
	seedHTTPMethodDelete = testpkg.HTTPMethodDelete

	seedHTTPStatusOK                  = testpkg.HTTPStatusOK
	seedHTTPStatusCreated             = testpkg.HTTPStatusCreated
	seedHTTPStatusUnauthorized        = testpkg.HTTPStatusUnauthorized
	seedHTTPStatusNotFound            = testpkg.HTTPStatusNotFound
	seedHTTPStatusConflict            = testpkg.HTTPStatusConflict
	seedHTTPStatusInternalServerError = testpkg.HTTPStatusInternalServerError
	seedHTTPStatusServiceUnavailable  = testpkg.HTTPStatusServiceUnavailable
)

var newSeedHTTPTestServer = testpkg.NewHTTPTestServer

type seedTestAdapter struct{ inner *testpkg.HTTPAPIAdapter }

type seedTestRandom struct{ next byte }

func (r *seedTestRandom) Read(buffer []byte) (int, error) {
	for index := range buffer {
		r.next++
		buffer[index] = r.next
	}
	return len(buffer), nil
}

func newSeedTestRandom() *seedTestRandom { return &seedTestRandom{} }

func newSeedTestAdapter(baseURL string) *seedTestAdapter {
	return &seedTestAdapter{inner: testpkg.NewHTTPAPIAdapter(baseURL)}
}

func (a *seedTestAdapter) BaseURL() string                       { return a.inner.BaseURL() }
func (a *seedTestAdapter) CheckHealth(ctx context.Context) error { return a.inner.CheckHealth(ctx) }

func (a *seedTestAdapter) LoginOperator(ctx context.Context, email, password string) (AuthRef, error) {
	auth, err := a.inner.Login(ctx, "/operator/auth/login", email, password, "")
	return seedTestAuth(auth), err
}

func (a *seedTestAdapter) LoginTenant(ctx context.Context, email, password, tenantSlug string) (AuthRef, error) {
	auth, err := a.inner.Login(ctx, "/auth/login", email, password, tenantSlug)
	return seedTestAuth(auth), err
}

func (a *seedTestAdapter) LoginParent(ctx context.Context, email, password string) (AuthRef, error) {
	auth, err := a.inner.Login(ctx, "/parent/auth/login", email, password, "")
	return seedTestAuth(auth), err
}

func (a *seedTestAdapter) Raw(ctx context.Context, auth AuthRef, method, path string, body any, headers map[string]string) ([]byte, int, error) {
	raw, status, err := a.inner.Raw(ctx, seedTestAPIAuth(auth), method, path, body, headers)
	return raw, status, seedTestError(err)
}

func (a *seedTestAdapter) RawUpload(ctx context.Context, auth AuthRef, method, path, contentType string, body []byte) ([]byte, int, error) {
	raw, status, err := a.inner.RawUpload(ctx, seedTestAPIAuth(auth), method, path, contentType, body)
	return raw, status, seedTestError(err)
}

func seedTestAuth(auth testpkg.APIAuth) AuthRef {
	return AuthRef{Kind: AuthKind(auth.Kind), Label: auth.Label, Token: auth.Token, APIKey: auth.APIKey, PIN: auth.PIN}
}

func seedTestAPIAuth(auth AuthRef) testpkg.APIAuth {
	return testpkg.APIAuth{Kind: string(auth.Kind), Label: auth.Label, Token: auth.Token, APIKey: auth.APIKey, PIN: auth.PIN}
}

func seedTestError(err error) error {
	var requestErr *testpkg.APIRequestError
	if errors.As(err, &requestErr) {
		return &APIError{Method: requestErr.Method, Path: requestErr.Path, StatusCode: requestErr.StatusCode, Code: requestErr.Code, Message: requestErr.Message, Body: requestErr.Body}
	}
	return err
}
