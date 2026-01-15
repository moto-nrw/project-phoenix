package port

import (
	"errors"
	"fmt"

	"github.com/lestrrat-go/jwx/v2/jwt"
)

// CommonClaims defines standard JWT timestamps shared by access/refresh claims.
type CommonClaims struct {
	ExpiresAt int64 `json:"exp,omitempty"`
	IssuedAt  int64 `json:"iat,omitempty"`
}

// AppClaims represent the claims parsed from JWT access token.
type AppClaims struct {
	ID          int      `json:"id,omitempty"`
	Sub         string   `json:"sub,omitempty"`
	Username    string   `json:"username,omitempty"`
	FirstName   string   `json:"first_name,omitempty"`
	LastName    string   `json:"last_name,omitempty"`
	Roles       []string `json:"roles,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
	IsAdmin     bool     `json:"is_admin,omitempty"`
	IsTeacher   bool     `json:"is_teacher,omitempty"`
	CommonClaims
}

// RefreshClaims represent the claims parsed from JWT refresh token.
type RefreshClaims struct {
	ID    int    `json:"id,omitempty"`
	Token string `json:"token,omitempty"`
	CommonClaims
}

const errMissingClaim = "missing required claim: %s"

func getRequiredInt(claims map[string]any, key string) (int, error) {
	val, ok := claims[key]
	if !ok {
		return 0, fmt.Errorf(errMissingClaim, key)
	}
	f, ok := val.(float64)
	if !ok {
		return 0, fmt.Errorf("claim %s is not a number", key)
	}
	return int(f), nil
}

func getRequiredString(claims map[string]any, key string) (string, error) {
	val, ok := claims[key]
	if !ok {
		return "", fmt.Errorf(errMissingClaim, key)
	}
	s, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("claim %s is not a string", key)
	}
	return s, nil
}

func getOptionalString(claims map[string]any, key string) string {
	val, ok := claims[key]
	if !ok || val == nil {
		return ""
	}
	s, _ := val.(string)
	return s
}

func getOptionalBool(claims map[string]any, key string) bool {
	val, ok := claims[key]
	if !ok || val == nil {
		return false
	}
	b, _ := val.(bool)
	return b
}

func getRequiredStringSlice(claims map[string]any, key string) ([]string, error) {
	val, ok := claims[key]
	if !ok {
		return nil, fmt.Errorf(errMissingClaim, key)
	}
	result, err := toStringSliceStrict(val)
	if err != nil {
		return nil, fmt.Errorf("claim %s: %w", key, err)
	}
	return result, nil
}

func getOptionalStringSlice(claims map[string]any, key string) []string {
	val, ok := claims[key]
	if !ok || val == nil {
		return []string{}
	}
	return toStringSliceLenient(val)
}

func toStringSliceStrict(val any) ([]string, error) {
	slice, ok := val.([]any)
	if !ok {
		return nil, errors.New("value is not an array")
	}
	result := make([]string, 0, len(slice))
	for i, v := range slice {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("element %d is not a string", i)
		}
		result = append(result, s)
	}
	return result, nil
}

func toStringSliceLenient(val any) []string {
	slice, ok := val.([]any)
	if !ok {
		return []string{}
	}
	result := make([]string, 0, len(slice))
	for _, v := range slice {
		if s, ok := v.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// ParseClaims parses JWT claims into AppClaims.
func (c *AppClaims) ParseClaims(claims map[string]any) error {
	var err error

	c.ID, err = getRequiredInt(claims, "id")
	if err != nil {
		return err
	}

	c.Sub, err = getRequiredString(claims, jwt.SubjectKey)
	if err != nil {
		return err
	}

	c.Username = getOptionalString(claims, "username")
	c.FirstName = getOptionalString(claims, "first_name")
	c.LastName = getOptionalString(claims, "last_name")

	c.Roles, err = getRequiredStringSlice(claims, "roles")
	if err != nil {
		return err
	}

	c.Permissions = getOptionalStringSlice(claims, "permissions")
	c.IsAdmin = getOptionalBool(claims, "is_admin")
	c.IsTeacher = getOptionalBool(claims, "is_teacher")

	return nil
}

// ParseClaims parses the JWT claims into RefreshClaims.
func (c *RefreshClaims) ParseClaims(claims map[string]any) error {
	id, ok := claims["id"]
	if !ok {
		return errors.New("could not parse claim id")
	}
	switch v := id.(type) {
	case float64:
		c.ID = int(v)
	case int:
		c.ID = v
	case int64:
		c.ID = int(v)
	default:
		return errors.New("invalid type for claim id")
	}

	token, ok := claims["token"]
	if !ok {
		return errors.New("could not parse claim token")
	}
	c.Token = token.(string)

	return nil
}
