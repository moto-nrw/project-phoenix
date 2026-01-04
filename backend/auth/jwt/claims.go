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
// Uses safe type assertions to prevent panics from malformed tokens.
func (c *AppClaims) ParseClaims(claims map[string]any) error {
	id, ok := claims["id"]
	if !ok {
		return errors.New("could not parse claim id")
	}
	idFloat, ok := id.(float64)
	if !ok {
		return errors.New("claim id is not a number")
	}
	c.ID = int(idFloat)

	sub, ok := claims[jwt.SubjectKey]
	if !ok {
		return errors.New("could not parse claim sub")
	}
	subStr, ok := sub.(string)
	if !ok {
		return errors.New("claim sub is not a string")
	}
	c.Sub = subStr

	if username, ok := claims["username"]; ok && username != nil {
		if usernameStr, ok := username.(string); ok {
			c.Username = usernameStr
		}
	}

	if firstName, ok := claims["first_name"]; ok && firstName != nil {
		if firstNameStr, ok := firstName.(string); ok {
			c.FirstName = firstNameStr
		}
	}

	if lastName, ok := claims["last_name"]; ok && lastName != nil {
		if lastNameStr, ok := lastName.(string); ok {
			c.LastName = lastNameStr
		}
	}

	rl, ok := claims["roles"]
	if !ok {
		return errors.New("could not parse claims roles")
	}

	var roles []string
	if rl != nil {
		rlSlice, ok := rl.([]any)
		if !ok {
			return errors.New("claim roles is not an array")
		}
		for _, v := range rlSlice {
			if roleStr, ok := v.(string); ok {
				roles = append(roles, roleStr)
			}
		}
	}
	c.Roles = roles

	// Parse permissions if they exist
	if perms, ok := claims["permissions"]; ok && perms != nil {
		if permsSlice, ok := perms.([]any); ok {
			var permissions []string
			for _, v := range permsSlice {
				if permStr, ok := v.(string); ok {
					permissions = append(permissions, permStr)
				}
			}
			c.Permissions = permissions
		}
	}
	if c.Permissions == nil {
		c.Permissions = []string{} // Initialize as empty array if not present
	}

	// Parse static role flags
	if isAdmin, ok := claims["is_admin"]; ok && isAdmin != nil {
		if isAdminBool, ok := isAdmin.(bool); ok {
			c.IsAdmin = isAdminBool
		}
	}

	if isTeacher, ok := claims["is_teacher"]; ok && isTeacher != nil {
		if isTeacherBool, ok := isTeacher.(bool); ok {
			c.IsTeacher = isTeacherBool
		}
	}

	return nil
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
