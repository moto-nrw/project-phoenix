package cmd

import (
	"context"
	"errors"

	backendapi "github.com/moto-nrw/project-phoenix/api"
	seedapi "github.com/moto-nrw/project-phoenix/seed/api"
	"github.com/moto-nrw/project-phoenix/simulate"
)

type seedCommandAdapter struct{ inner *backendapi.Adapter }

func newSeedCommandAdapter(baseURL string, verbose bool) seedCommandAdapter {
	return seedCommandAdapter{inner: backendapi.NewCommandAdapter(baseURL, verbose)}
}

func (a seedCommandAdapter) BaseURL() string                       { return a.inner.BaseURL() }
func (a seedCommandAdapter) CheckHealth(ctx context.Context) error { return a.inner.CheckHealth(ctx) }

func (a seedCommandAdapter) LoginOperator(ctx context.Context, email, password string) (seedapi.AuthRef, error) {
	auth, err := a.inner.LoginOperator(ctx, email, password)
	return commandSeedAuth(auth), err
}

func (a seedCommandAdapter) LoginTenant(ctx context.Context, email, password, tenantSlug string) (seedapi.AuthRef, error) {
	auth, err := a.inner.LoginTenant(ctx, email, password, tenantSlug)
	return commandSeedAuth(auth), err
}

func (a seedCommandAdapter) LoginParent(ctx context.Context, email, password string) (seedapi.AuthRef, error) {
	auth, err := a.inner.LoginParent(ctx, email, password)
	return commandSeedAuth(auth), err
}

func (a seedCommandAdapter) Raw(ctx context.Context, auth seedapi.AuthRef, method, path string, body any, headers map[string]string) ([]byte, int, error) {
	raw, status, err := a.inner.Raw(ctx, commandAPIAuth(auth), method, path, body, headers)
	return raw, status, commandSeedError(err)
}

func (a seedCommandAdapter) RawUpload(ctx context.Context, auth seedapi.AuthRef, method, path, contentType string, body []byte) ([]byte, int, error) {
	raw, status, err := a.inner.RawUpload(ctx, commandAPIAuth(auth), method, path, contentType, body)
	return raw, status, commandSeedError(err)
}

func commandSeedAuth(auth backendapi.AuthRef) seedapi.AuthRef {
	return seedapi.AuthRef{Kind: seedapi.AuthKind(auth.Kind), Label: auth.Label, Token: auth.Token, APIKey: auth.APIKey, PIN: auth.PIN}
}

func commandAPIAuth(auth seedapi.AuthRef) backendapi.AuthRef {
	return backendapi.AuthRef{Kind: backendapi.AuthKind(auth.Kind), Label: auth.Label, Token: auth.Token, APIKey: auth.APIKey, PIN: auth.PIN}
}

func commandSeedError(err error) error {
	var apiErr *backendapi.APIError
	if errors.As(err, &apiErr) {
		return &seedapi.APIError{Method: apiErr.Method, Path: apiErr.Path, StatusCode: apiErr.StatusCode, Message: apiErr.Message, Body: apiErr.Body}
	}
	return err
}

func newSimulationClient(baseURL string, verbose bool) (simulate.Client, error) {
	if err := assertNonProductionURL(baseURL); err != nil {
		return nil, err
	}
	return seedapi.NewClientWithAdapter(newSeedCommandAdapter(baseURL, verbose), verbose), nil
}
