package jwt

import (
	"errors"

	"github.com/lestrrat-go/jwx/v2/jwt"
)

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
	Permissions []string `json:"permissions,omitempty"` // Added permissions field
	// Static role flags for quick access
	IsAdmin   bool `json:"is_admin,omitempty"`
	IsTeacher bool `json:"is_teacher,omitempty"`
	CommonClaims
}

// ParseClaims parses JWT claims into AppClaims.
func (c *AppClaims) ParseClaims(claims map[string]any) error {
	var err error

	// Parse required fields
	c.ID, err = parseRequiredInt(claims, "id")
	if err != nil {
		return err
	}

	c.Sub, err = parseRequiredString(claims, jwt.SubjectKey)
	if err != nil {
		return err
	}

	// Parse optional string fields
	c.Username = parseOptionalString(claims, "username")
	c.FirstName = parseOptionalString(claims, "first_name")
	c.LastName = parseOptionalString(claims, "last_name")

	// Parse roles (required array)
	c.Roles, err = parseRequiredStringArray(claims, "roles")
	if err != nil {
		return err
	}

	// Parse optional permissions array
	c.Permissions = parseOptionalStringArray(claims, "permissions")

	// Parse optional boolean flags
	c.IsAdmin = parseOptionalBool(claims, "is_admin")
	c.IsTeacher = parseOptionalBool(claims, "is_teacher")

	return nil
}

// parseRequiredString extracts a required string claim
func parseRequiredString(claims map[string]any, key string) (string, error) {
	value, ok := claims[key]
	if !ok {
		return "", errors.New("could not parse claim " + key)
	}
	return value.(string), nil
}

// parseRequiredInt extracts a required int claim from float64
func parseRequiredInt(claims map[string]any, key string) (int, error) {
	value, ok := claims[key]
	if !ok {
		return 0, errors.New("could not parse claim " + key)
	}
	return int(value.(float64)), nil
}

// parseOptionalString extracts an optional string claim
func parseOptionalString(claims map[string]any, key string) string {
	value, ok := claims[key]
	if ok && value != nil {
		return value.(string)
	}
	return ""
}

// parseOptionalBool extracts an optional bool claim
func parseOptionalBool(claims map[string]any, key string) bool {
	value, ok := claims[key]
	if ok && value != nil {
		return value.(bool)
	}
	return false
}

// parseRequiredStringArray extracts a required array of strings
func parseRequiredStringArray(claims map[string]any, key string) ([]string, error) {
	value, ok := claims[key]
	if !ok {
		return nil, errors.New("could not parse claims " + key)
	}

	if value == nil {
		return []string{}, nil
	}

	return convertToStringArray(value.([]any)), nil
}

// parseOptionalStringArray extracts an optional array of strings
func parseOptionalStringArray(claims map[string]any, key string) []string {
	value, ok := claims[key]
	if !ok || value == nil {
		return []string{}
	}

	return convertToStringArray(value.([]any))
}

// convertToStringArray converts []any to []string
func convertToStringArray(arr []any) []string {
	result := make([]string, 0, len(arr))
	for _, v := range arr {
		result = append(result, v.(string))
	}
	return result
}

// RefreshClaims represents the claims parsed from JWT refresh token.
type RefreshClaims struct {
	ID    int    `json:"id,omitempty"`
	Token string `json:"token,omitempty"`
	CommonClaims
}

// ParseClaims parses the JWT claims into RefreshClaims.
func (c *RefreshClaims) ParseClaims(claims map[string]any) error {
	// Parse ID field
	id, ok := claims["id"]
	if !ok {
		return errors.New("could not parse claim id")
	}
	// Handle type assertion for numeric ID
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

	// Parse token field
	token, ok := claims["token"]
	if !ok {
		return errors.New("could not parse claim token")
	}
	c.Token = token.(string)

	return nil
}
