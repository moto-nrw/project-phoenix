// Package userpass provides JSON Web Token (JWT) authentication and authorization middleware.
// It implements a username/password authentication flow by verifying credentials and generating JWT access and refresh tokens.
package userpass

import (
	"errors"
	"fmt"
	jwt2 "github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/logging"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/go-ozzo/ozzo-validation/is"
	"github.com/gofrs/uuid"
	"github.com/mssola/user_agent"
	"github.com/sirupsen/logrus"
)

// AuthStorer defines database operations on accounts and tokens.
type AuthStorer interface {
	GetAccount(id int) (*Account, error)
	GetAccountByEmail(email string) (*Account, error)
	CreateAccount(a *Account) error
	UpdateAccount(a *Account) error
	UpdateAccountPassword(id int, passwordHash string) error

	GetToken(token string) (*jwt2.Token, error)
	CreateOrUpdateToken(t *jwt2.Token) error
	DeleteToken(t *jwt2.Token) error
	PurgeExpiredToken() error
}

// TokenAuthInterface defines the JWT token auth operations needed by the resource
type TokenAuthInterface interface {
	Verifier() func(http.Handler) http.Handler
	GenTokenPair(appClaims jwt2.AppClaims, refreshClaims jwt2.RefreshClaims) (string, string, error)
	GetRefreshExpiry() time.Duration
}

// Resource implements username/password account authentication against a database.
type Resource struct {
	TokenAuth TokenAuthInterface
	Store     AuthStorer
}

// NewResource returns a configured authentication resource.
func NewResource(authStore AuthStorer) (*Resource, error) {
	tokenAuth, err := jwt2.NewTokenAuth()
	if err != nil {
		return nil, err
	}

	resource := &Resource{
		TokenAuth: tokenAuth,
		Store:     authStore,
	}

	return resource, nil
}

// Router provides necessary routes for username/password authentication flow.
func (rs *Resource) Router() *chi.Mux {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Post("/login", rs.login)
	r.Post("/register", rs.register)
	r.Group(func(r chi.Router) {
		r.Use(rs.TokenAuth.Verifier())
		r.Use(jwt2.Authenticator)
		r.Post("/change-password", rs.changePassword)
	})
	r.Group(func(r chi.Router) {
		r.Use(rs.TokenAuth.Verifier())
		r.Use(jwt2.AuthenticateRefreshJWT)
		r.Post("/refresh", rs.refresh)
		r.Post("/logout", rs.logout)
	})
	return r
}

func log(r *http.Request) logrus.FieldLogger {
	return logging.GetLogEntry(r)
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (body *loginRequest) Bind(r *http.Request) error {
	body.Email = strings.TrimSpace(body.Email)
	body.Email = strings.ToLower(body.Email)

	return validation.ValidateStruct(body,
		validation.Field(&body.Email, validation.Required, is.Email),
		validation.Field(&body.Password, validation.Required),
	)
}

type tokenResponse struct {
	Access  string `json:"access_token"`
	Refresh string `json:"refresh_token"`
}

func (rs *Resource) login(w http.ResponseWriter, r *http.Request) {
	body := &loginRequest{}
	if err := render.Bind(r, body); err != nil {
		log(r).WithField("email", body.Email).Warn(err)
		render.Render(w, r, ErrInvalidLogin(errors.New(InvalidLogin)))
		return
	}

	acc, err := rs.Store.GetAccountByEmail(body.Email)
	if err != nil {
		log(r).WithField("email", body.Email).Warn(err)
		render.Render(w, r, ErrUnknownLogin(errors.New(UnknownLogin)))
		return
	}

	if !acc.CanLogin() {
		render.Render(w, r, ErrLoginDisabled(errors.New(LoginDisabled)))
		return
	}

	if acc.PasswordHash == "" {
		log(r).WithField("email", body.Email).Warn("Account has no password set")
		render.Render(w, r, ErrInvalidCredentials(ErrInvalidPassword))
		return
	}

	valid, err := VerifyPassword(body.Password, acc.PasswordHash)
	if err != nil || !valid {
		log(r).WithField("email", body.Email).Warn("Invalid password")
		render.Render(w, r, ErrInvalidCredentials(ErrInvalidPassword))
		return
	}

	ua := user_agent.New(r.UserAgent())
	browser, _ := ua.Browser()

	token := &jwt2.Token{
		Token:      uuid.Must(uuid.NewV4()).String(),
		Expiry:     time.Now().Add(rs.TokenAuth.GetRefreshExpiry()),
		UpdatedAt:  time.Now(),
		AccountID:  acc.ID,
		Mobile:     ua.Mobile(),
		Identifier: fmt.Sprintf("%s on %s", browser, ua.OS()),
	}

	if err := rs.Store.CreateOrUpdateToken(token); err != nil {
		log(r).Error(err)
		render.Render(w, r, ErrInternalServerError)
		return
	}

	access, refresh, err := rs.TokenAuth.GenTokenPair(acc.Claims(), token.Claims())
	if err != nil {
		log(r).Error(err)
		render.Render(w, r, ErrInternalServerError)
		return
	}

	acc.LastLogin = time.Now()
	if err := rs.Store.UpdateAccount(acc); err != nil {
		log(r).Error(err)
		render.Render(w, r, ErrInternalServerError)
		return
	}

	render.Respond(w, r, &tokenResponse{
		Access:  access,
		Refresh: refresh,
	})
}

type registerRequest struct {
	Email           string `json:"email"`
	Name            string `json:"name"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
}

func (body *registerRequest) Bind(r *http.Request) error {
	body.Email = strings.TrimSpace(body.Email)
	body.Email = strings.ToLower(body.Email)
	body.Name = strings.TrimSpace(body.Name)

	return validation.ValidateStruct(body,
		validation.Field(&body.Email, validation.Required, is.Email, is.LowerCase),
		validation.Field(&body.Name, validation.Required, is.ASCII),
		validation.Field(&body.Password, validation.Required, validation.By(validatePassword)),
		validation.Field(&body.ConfirmPassword, validation.Required, validation.By(func(value interface{}) error {
			pass, _ := value.(string)
			if pass != body.Password {
				return errors.New(PasswordMismatch)
			}
			return nil
		})),
	)
}

func validatePassword(value interface{}) error {
	password, _ := value.(string)

	if len(password) < 8 {
		return ErrPasswordTooShort
	}

	if !regexp.MustCompile(`[A-Z]`).MatchString(password) {
		return ErrPasswordNoUpper
	}

	if !regexp.MustCompile(`[a-z]`).MatchString(password) {
		return ErrPasswordNoLower
	}

	if !regexp.MustCompile(`[0-9]`).MatchString(password) {
		return ErrPasswordNoNumber
	}

	if !regexp.MustCompile(`[^a-zA-Z0-9]`).MatchString(password) {
		return ErrPasswordNoSpecial
	}

	return nil
}

type registerResponse struct {
	ID    int    `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

func (rs *Resource) register(w http.ResponseWriter, r *http.Request) {
	body := &registerRequest{}
	if err := render.Bind(r, body); err != nil {
		log(r).WithField("email", body.Email).Warn(err)
		render.Render(w, r, ErrInvalidRegistration(err))
		return
	}

	// Check if email already exists
	_, err := rs.Store.GetAccountByEmail(body.Email)
	if err == nil {
		log(r).WithField("email", body.Email).Warn("Email already exists")
		render.Render(w, r, ErrInvalidRegistration(ErrEmailAlreadyExists))
		return
	}

	// Hash the password
	passwordHash, err := HashPassword(body.Password, DefaultParams())
	if err != nil {
		log(r).Error(err)
		render.Render(w, r, ErrInternalServerError)
		return
	}

	// Create the new account
	account := &Account{
		Email:        body.Email,
		Name:         body.Name,
		Active:       true,
		Roles:        []string{"user"},
		PasswordHash: passwordHash,
		LastLogin:    time.Now(),
	}

	if err := rs.Store.CreateAccount(account); err != nil {
		log(r).Error(err)
		render.Render(w, r, ErrInternalServerError)
		return
	}

	render.Respond(w, r, &registerResponse{
		ID:    account.ID,
		Email: account.Email,
		Name:  account.Name,
	})
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
	ConfirmPassword string `json:"confirm_password"`
}

func (body *changePasswordRequest) Bind(r *http.Request) error {
	return validation.ValidateStruct(body,
		validation.Field(&body.CurrentPassword, validation.Required),
		validation.Field(&body.NewPassword, validation.Required, validation.By(validatePassword)),
		validation.Field(&body.ConfirmPassword, validation.Required, validation.By(func(value interface{}) error {
			pass, _ := value.(string)
			if pass != body.NewPassword {
				return errors.New(PasswordMismatch)
			}
			return nil
		})),
	)
}

func (rs *Resource) changePassword(w http.ResponseWriter, r *http.Request) {
	body := &changePasswordRequest{}
	if err := render.Bind(r, body); err != nil {
		log(r).Warn(err)
		render.Render(w, r, ErrInvalidCredentials(err))
		return
	}

	// Get the user ID from the JWT context
	claims := jwt2.ClaimsFromCtx(r.Context())
	userId := claims.ID

	// Get the user account
	acc, err := rs.Store.GetAccount(userId)
	if err != nil {
		log(r).Warn(err)
		render.Render(w, r, ErrUnknownLogin(errors.New(UnknownLogin)))
		return
	}

	// Verify the current password
	valid, err := VerifyPassword(body.CurrentPassword, acc.PasswordHash)
	if err != nil || !valid {
		log(r).Warn("Invalid current password")
		render.Render(w, r, ErrInvalidCredentials(ErrInvalidPassword))
		return
	}

	// Hash the new password
	passwordHash, err := HashPassword(body.NewPassword, DefaultParams())
	if err != nil {
		log(r).Error(err)
		render.Render(w, r, ErrInternalServerError)
		return
	}

	// Update the password
	if err := rs.Store.UpdateAccountPassword(userId, passwordHash); err != nil {
		log(r).Error(err)
		render.Render(w, r, ErrInternalServerError)
		return
	}

	render.Respond(w, r, http.NoBody)
}

func (rs *Resource) refresh(w http.ResponseWriter, r *http.Request) {
	rt := jwt2.RefreshTokenFromCtx(r.Context())

	token, err := rs.Store.GetToken(rt)
	if err != nil {
		render.Render(w, r, ErrInvalidCredentials(jwt2.ErrTokenExpired))
		return
	}

	if time.Now().After(token.Expiry) {
		rs.Store.DeleteToken(token)
		render.Render(w, r, ErrInvalidCredentials(jwt2.ErrTokenExpired))
		return
	}

	acc, err := rs.Store.GetAccount(token.AccountID)
	if err != nil {
		render.Render(w, r, ErrUnknownLogin(errors.New(UnknownLogin)))
		return
	}

	if !acc.CanLogin() {
		render.Render(w, r, ErrLoginDisabled(errors.New(LoginDisabled)))
		return
	}

	token.Token = uuid.Must(uuid.NewV4()).String()
	token.Expiry = time.Now().Add(rs.TokenAuth.GetRefreshExpiry())
	token.UpdatedAt = time.Now()

	access, refresh, err := rs.TokenAuth.GenTokenPair(acc.Claims(), token.Claims())
	if err != nil {
		log(r).Error(err)
		render.Render(w, r, ErrInternalServerError)
		return
	}

	if err := rs.Store.CreateOrUpdateToken(token); err != nil {
		log(r).Error(err)
		render.Render(w, r, ErrInternalServerError)
		return
	}

	acc.LastLogin = time.Now()
	if err := rs.Store.UpdateAccount(acc); err != nil {
		log(r).Error(err)
		render.Render(w, r, ErrInternalServerError)
		return
	}

	render.Respond(w, r, &tokenResponse{
		Access:  access,
		Refresh: refresh,
	})
}

func (rs *Resource) logout(w http.ResponseWriter, r *http.Request) {
	rt := jwt2.RefreshTokenFromCtx(r.Context())
	token, err := rs.Store.GetToken(rt)
	if err != nil {
		render.Render(w, r, ErrInvalidCredentials(jwt2.ErrTokenExpired))
		return
	}
	rs.Store.DeleteToken(token)

	render.Respond(w, r, http.NoBody)
}