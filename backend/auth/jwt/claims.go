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
	CommonClaims
}

// ParseClaims parses JWT claims into AppClaims.
func (c *AppClaims) ParseClaims(claims map[string]any) error {
	id, ok := claims["id"]
	if !ok {
		return errors.New("could not parse claim id")
	}
	c.ID = int(id.(float64))

	sub, ok := claims[jwt.SubjectKey]
	if !ok {
		return errors.New("could not parse claim sub")
	}
	c.Sub = sub.(string)

	username, ok := claims["username"]
	if ok && username != nil {
		c.Username = username.(string)
	}

	firstName, ok := claims["first_name"]
	if ok && firstName != nil {
		c.FirstName = firstName.(string)
	}

	lastName, ok := claims["last_name"]
	if ok && lastName != nil {
		c.LastName = lastName.(string)
	}

	rl, ok := claims["roles"]
	if !ok {
		return errors.New("could not parse claims roles")
	}

	var roles []string
	if rl != nil {
		for _, v := range rl.([]any) {
			roles = append(roles, v.(string))
		}
	}
	c.Roles = roles

	// Parse permissions if they exist
	perms, ok := claims["permissions"]
	if ok && perms != nil {
		var permissions []string
		for _, v := range perms.([]any) {
			permissions = append(permissions, v.(string))
		}
		c.Permissions = permissions
	} else {
		c.Permissions = []string{} // Initialize as empty array if not present
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
	token, ok := claims["token"]
	if !ok {
		return errors.New("could not parse claim token")
	}
	c.Token = token.(string)
	return nil
}
