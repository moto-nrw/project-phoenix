package auth

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/auth"
)

// AuthService defines the operations for authentication and user management
type AuthService interface {
	// Login authenticates a user and returns access and refresh tokens
	Login(ctx context.Context, email, password string) (accessToken, refreshToken string, err error)

	// Register creates a new user account
	Register(ctx context.Context, email, username, name, password string) (*auth.Account, error)

	// ValidateToken validates an access token and returns the associated account
	ValidateToken(ctx context.Context, token string) (*auth.Account, error)

	// RefreshToken generates new token pair from a refresh token
	RefreshToken(ctx context.Context, refreshToken string) (accessToken, newRefreshToken string, err error)

	// Logout invalidates a refresh token
	Logout(ctx context.Context, refreshToken string) error

	// ChangePassword updates an account's password
	ChangePassword(ctx context.Context, accountID int, currentPassword, newPassword string) error

	// GetAccountByID retrieves an account by ID
	GetAccountByID(ctx context.Context, id int) (*auth.Account, error)

	// GetAccountByEmail retrieves an account by email
	GetAccountByEmail(ctx context.Context, email string) (*auth.Account, error)
}
