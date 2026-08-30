package api

import (
	"context"
	"fmt"
)

type AuthKind string

const (
	AuthBearer AuthKind = "bearer"
	AuthDevice AuthKind = "device"
)

type AuthRef struct {
	Kind   AuthKind
	Label  string
	Token  string
	APIKey string
	PIN    string
}

func DeviceAuth(apiKey, pin, label string) AuthRef {
	return AuthRef{Kind: AuthDevice, Label: label, APIKey: apiKey, PIN: pin}
}

type APIError struct {
	Method     string
	Path       string
	StatusCode int
	Message    string
	Body       string
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return fmt.Sprintf("%s %s failed: %d - %s", e.Method, e.Path, e.StatusCode, e.Message)
	}
	if e.Body != "" {
		return fmt.Sprintf("%s %s failed: %d - %s", e.Method, e.Path, e.StatusCode, e.Body)
	}
	return fmt.Sprintf("%s %s failed: %d", e.Method, e.Path, e.StatusCode)
}

type Adapter interface {
	BaseURL() string
	CheckHealth(context.Context) error
	LoginOperator(context.Context, string, string) (AuthRef, error)
	LoginTenant(context.Context, string, string, string) (AuthRef, error)
	LoginParent(context.Context, string, string) (AuthRef, error)
	Raw(context.Context, AuthRef, string, string, any, map[string]string) ([]byte, int, error)
	RawUpload(context.Context, AuthRef, string, string, string, []byte) ([]byte, int, error)
}
