package simulate

import (
	"context"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type simulationHTTPRequest = testpkg.HTTPRequest
type simulationHTTPResponseWriter = testpkg.HTTPResponseWriter
type simulationHTTPTestServer = testpkg.HTTPTestServer

const (
	simulationHTTPMethodPost                = testpkg.HTTPMethodPost
	simulationHTTPMethodDelete              = testpkg.HTTPMethodDelete
	simulationHTTPStatusOK                  = testpkg.HTTPStatusOK
	simulationHTTPStatusNoContent           = testpkg.HTTPStatusNoContent
	simulationHTTPStatusUnauthorized        = testpkg.HTTPStatusUnauthorized
	simulationHTTPStatusConflict            = testpkg.HTTPStatusConflict
	simulationHTTPStatusInternalServerError = testpkg.HTTPStatusInternalServerError
)

var newSimulationHTTPTestServer = testpkg.NewHTTPTestServer

type simulationTestAuth struct {
	Kind, Label, Token string
}

const simulationTestAuthBearer = "bearer"

type simulationTestClient struct {
	inner *testpkg.HTTPAPIAdapter
	auth  testpkg.APIAuth
}

func newClient(baseURL string, _ bool) *simulationTestClient {
	return &simulationTestClient{inner: testpkg.NewHTTPAPIAdapter(baseURL)}
}

func newTestClientFactory(baseURL string, verbose bool) (Client, error) {
	return newClient(baseURL, verbose), nil
}

func (c *simulationTestClient) BindAuth(auth simulationTestAuth) {
	c.auth = testpkg.APIAuth{Kind: auth.Kind, Label: auth.Label, Token: auth.Token}
}

func (c *simulationTestClient) CheckHealth() error {
	return c.inner.CheckHealth(context.Background())
}

func (c *simulationTestClient) Login(email, password string, tenantSlug ...string) error {
	slug := ""
	if len(tenantSlug) > 0 {
		slug = tenantSlug[0]
	}
	auth, err := c.inner.Login(context.Background(), "/auth/login", email, password, slug)
	if err == nil {
		c.auth = auth
	}
	return err
}

func (c *simulationTestClient) Get(path string) ([]byte, error) {
	return c.raw(c.auth, "GET", path, nil)
}
func (c *simulationTestClient) Post(path string, body any) ([]byte, error) {
	return c.raw(c.auth, "POST", path, body)
}
func (c *simulationTestClient) Put(path string, body any) ([]byte, error) {
	return c.raw(c.auth, "PUT", path, body)
}
func (c *simulationTestClient) Delete(path string) ([]byte, error) {
	return c.raw(c.auth, "DELETE", path, nil)
}
func (c *simulationTestClient) DeviceGet(path, apiKey, pin string) ([]byte, error) {
	return c.raw(testpkg.APIAuth{Kind: "device", APIKey: apiKey, PIN: pin}, "GET", path, nil)
}
func (c *simulationTestClient) DevicePost(path string, body any, apiKey, pin string) ([]byte, error) {
	return c.raw(testpkg.APIAuth{Kind: "device", APIKey: apiKey, PIN: pin}, "POST", path, body)
}
func (c *simulationTestClient) DevicePut(path string, body any, apiKey, pin string) ([]byte, error) {
	return c.raw(testpkg.APIAuth{Kind: "device", APIKey: apiKey, PIN: pin}, "PUT", path, body)
}

func (c *simulationTestClient) raw(auth testpkg.APIAuth, method, path string, body any) ([]byte, error) {
	raw, _, err := c.inner.Raw(context.Background(), auth, method, path, body, nil)
	return raw, err
}
