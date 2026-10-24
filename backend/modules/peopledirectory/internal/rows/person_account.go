package users

import "time"

// PersonAccount is the account metadata attached to a person by the directory
// lookup. Credentials and account persistence operations stay with Identity &
// Access. The JSON fields preserve the existing person response.
type PersonAccount struct {
	ID            int64      `json:"id"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	Email         string     `json:"email"`
	Username      *string    `json:"username,omitempty"`
	Avatar        string     `json:"avatar,omitempty"`
	Active        bool       `json:"active"`
	IsPasswordOTP bool       `json:"is_password_otp"`
	LastLogin     *time.Time `json:"last_login,omitempty"`
}
