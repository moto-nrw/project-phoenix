// Package internal provides internal-only API endpoints.
// These endpoints are NOT exposed to the public internet.
// Security is enforced via Docker network isolation.
package internal

import (
	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/email"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
)

// Resource handles internal API endpoints.
type Resource struct {
	mailer            email.Mailer
	dispatcher        *email.Dispatcher
	fromEmail         email.Email
	invitationService authService.InvitationService
	accountRepo       authModels.AccountRepository
}

// NewResource creates a new internal API resource.
func NewResource(
	mailer email.Mailer,
	dispatcher *email.Dispatcher,
	fromEmail email.Email,
	invitationService authService.InvitationService,
	accountRepo authModels.AccountRepository,
) *Resource {
	return &Resource{
		mailer:            mailer,
		dispatcher:        dispatcher,
		fromEmail:         fromEmail,
		invitationService: invitationService,
		accountRepo:       accountRepo,
	}
}

// Router returns the chi router for internal API routes.
// NOTE: These routes must NOT be exposed to the public internet.
// They should only be accessible from within the Docker network.
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()

	// No authentication middleware - internal network provides security
	// POST /api/internal/email - Send an email using a template
	r.Post("/email", rs.sendEmail)

	// POST /api/internal/invitations - Create invitation (for SaaS admin console)
	r.Post("/invitations", rs.createInvitation)

	return r
}
