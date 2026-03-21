package operator

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	"github.com/spf13/viper"
)

const seedTokenHeader = "X-Phoenix-Seed-Token"

// ProvisioningResource handles operator tenant provisioning endpoints.
type ProvisioningResource struct {
	service platformSvc.OperatorProvisioningService
}

// NewProvisioningResource creates a new provisioning resource.
func NewProvisioningResource(service platformSvc.OperatorProvisioningService) *ProvisioningResource {
	return &ProvisioningResource{service: service}
}

type createOrganizationRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

func (req *createOrganizationRequest) Bind(_ *http.Request) error {
	req.Name = strings.TrimSpace(req.Name)
	req.Slug = strings.TrimSpace(req.Slug)
	return nil
}

type createSchoolRequest struct {
	OrganizationID int64  `json:"organization_id"`
	Name           string `json:"name"`
	Slug           string `json:"slug"`
	Subdomain      string `json:"subdomain"`
	Address        string `json:"address,omitempty"`
	City           string `json:"city,omitempty"`
	Zip            string `json:"zip,omitempty"`
	Phone          string `json:"phone,omitempty"`
	Email          string `json:"email,omitempty"`
}

func (req *createSchoolRequest) Bind(_ *http.Request) error {
	req.Name = strings.TrimSpace(req.Name)
	req.Slug = strings.TrimSpace(req.Slug)
	req.Subdomain = strings.TrimSpace(req.Subdomain)
	req.Address = strings.TrimSpace(req.Address)
	req.City = strings.TrimSpace(req.City)
	req.Zip = strings.TrimSpace(req.Zip)
	req.Phone = strings.TrimSpace(req.Phone)
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	return nil
}

type updateOrganizationRequest struct {
	Name   string `json:"name"`
	Slug   string `json:"slug"`
	Active bool   `json:"active"`
}

func (req *updateOrganizationRequest) Bind(_ *http.Request) error {
	req.Name = strings.TrimSpace(req.Name)
	req.Slug = strings.TrimSpace(req.Slug)
	if req.Name == "" {
		return errors.New("name is required")
	}
	if req.Slug == "" {
		return errors.New("slug is required")
	}
	return nil
}

type updateSchoolRequest struct {
	OrganizationID int64  `json:"organization_id"`
	Name           string `json:"name"`
	Slug           string `json:"slug"`
	Subdomain      string `json:"subdomain"`
	Address        string `json:"address"`
	City           string `json:"city"`
	Zip            string `json:"zip"`
	Phone          string `json:"phone"`
	Email          string `json:"email"`
	Active         bool   `json:"active"`
}

func (req *updateSchoolRequest) Bind(_ *http.Request) error {
	req.Name = strings.TrimSpace(req.Name)
	req.Slug = strings.TrimSpace(req.Slug)
	req.Subdomain = strings.TrimSpace(req.Subdomain)
	req.Address = strings.TrimSpace(req.Address)
	req.City = strings.TrimSpace(req.City)
	req.Zip = strings.TrimSpace(req.Zip)
	req.Phone = strings.TrimSpace(req.Phone)
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Name == "" {
		return errors.New("name is required")
	}
	if req.Slug == "" {
		return errors.New("slug is required")
	}
	if req.Subdomain == "" {
		return errors.New("subdomain is required")
	}
	return nil
}

type inviteSchoolAdminRequest struct {
	Email     string `json:"email"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	Position  string `json:"position,omitempty"`
}

type operatorInvitationResponse struct {
	ID              int64      `json:"id"`
	Email           string     `json:"email"`
	RoleID          int64      `json:"role_id"`
	RoleName        string     `json:"role_name,omitempty"`
	Token           *string    `json:"token,omitempty"`
	ExpiresAt       time.Time  `json:"expires_at"`
	FirstName       *string    `json:"first_name,omitempty"`
	LastName        *string    `json:"last_name,omitempty"`
	Position        *string    `json:"position,omitempty"`
	CreatedBy       int64      `json:"created_by"`
	Creator         string     `json:"creator,omitempty"`
	DeliveryStatus  string     `json:"delivery_status"`
	EmailSentAt     *time.Time `json:"email_sent_at,omitempty"`
	EmailError      *string    `json:"email_error,omitempty"`
	EmailRetryCount int        `json:"email_retry_count"`
}

func (req *inviteSchoolAdminRequest) Bind(_ *http.Request) error {
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)
	req.Position = strings.TrimSpace(req.Position)
	return nil
}

func (rs *ProvisioningResource) CreateOrganization(w http.ResponseWriter, r *http.Request) {
	req := &createOrganizationRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}
	operatorID := int64(jwt.ClaimsFromCtx(r.Context()).ID)
	org := &platformModels.Organization{Name: req.Name, Slug: req.Slug, Active: true}
	created, err := rs.service.CreateOrganization(r.Context(), org, operatorID, getClientIP(r))
	if err != nil {
		common.RenderError(w, r, ProvisioningErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusCreated, created, "Organization created successfully")
}

func (rs *ProvisioningResource) ListOrganizations(w http.ResponseWriter, r *http.Request) {
	organizations, err := rs.service.ListOrganizations(r.Context())
	if err != nil {
		common.RenderError(w, r, ProvisioningErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, organizations, "Organizations retrieved successfully")
}

func (rs *ProvisioningResource) CreateSchool(w http.ResponseWriter, r *http.Request) {
	req := &createSchoolRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}
	operatorID := int64(jwt.ClaimsFromCtx(r.Context()).ID)
	school := &platformModels.School{
		OrganizationID: req.OrganizationID,
		Name:           req.Name,
		Slug:           req.Slug,
		Subdomain:      req.Subdomain,
		Address:        req.Address,
		City:           req.City,
		Zip:            req.Zip,
		Phone:          req.Phone,
		Email:          req.Email,
		Active:         true,
	}
	created, err := rs.service.CreateSchool(r.Context(), school, operatorID, getClientIP(r))
	if err != nil {
		common.RenderError(w, r, ProvisioningErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusCreated, created, "School created successfully")
}

func (rs *ProvisioningResource) ListSchools(w http.ResponseWriter, r *http.Request) {
	schools, err := rs.service.ListSchools(r.Context())
	if err != nil {
		common.RenderError(w, r, ProvisioningErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, schools, "Schools retrieved successfully")
}

func (rs *ProvisioningResource) UpdateOrganization(w http.ResponseWriter, r *http.Request) {
	req := &updateOrganizationRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}
	orgID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid organization ID")
	if !ok {
		return
	}
	operatorID := int64(jwt.ClaimsFromCtx(r.Context()).ID)
	svcReq := platformSvc.UpdateOrganizationRequest{
		Name:   req.Name,
		Slug:   req.Slug,
		Active: req.Active,
	}
	updated, err := rs.service.UpdateOrganization(r.Context(), orgID, svcReq, operatorID, getClientIP(r))
	if err != nil {
		common.RenderError(w, r, ProvisioningErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, updated, "Organization updated successfully")
}

func (rs *ProvisioningResource) UpdateSchool(w http.ResponseWriter, r *http.Request) {
	req := &updateSchoolRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}
	schoolID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid school ID")
	if !ok {
		return
	}
	operatorID := int64(jwt.ClaimsFromCtx(r.Context()).ID)
	svcReq := platformSvc.UpdateSchoolRequest{
		OrganizationID: req.OrganizationID,
		Name:           req.Name,
		Slug:           req.Slug,
		Subdomain:      req.Subdomain,
		Address:        req.Address,
		City:           req.City,
		Zip:            req.Zip,
		Phone:          req.Phone,
		Email:          req.Email,
		Active:         req.Active,
	}
	updated, err := rs.service.UpdateSchool(r.Context(), schoolID, svcReq, operatorID, getClientIP(r))
	if err != nil {
		common.RenderError(w, r, ProvisioningErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, updated, "School updated successfully")
}

func (rs *ProvisioningResource) InviteSchoolAdmin(w http.ResponseWriter, r *http.Request) {
	req := &inviteSchoolAdminRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}
	schoolID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid school ID")
	if !ok {
		return
	}
	operatorID := int64(jwt.ClaimsFromCtx(r.Context()).ID)
	invitationReq := authSvc.InvitationRequest{Email: req.Email}
	if req.FirstName != "" {
		first := req.FirstName
		invitationReq.FirstName = &first
	}
	if req.LastName != "" {
		last := req.LastName
		invitationReq.LastName = &last
	}
	if req.Position != "" {
		position := req.Position
		invitationReq.Position = &position
	}
	invitation, err := rs.service.InviteSchoolAdmin(r.Context(), schoolID, operatorID, getClientIP(r), invitationReq)
	if err != nil {
		common.RenderError(w, r, ProvisioningErrorRenderer(err))
		return
	}
	resp := operatorInvitationResponse{
		ID:              invitation.ID,
		Email:           invitation.Email,
		RoleID:          invitation.RoleID,
		ExpiresAt:       invitation.ExpiresAt,
		FirstName:       invitation.FirstName,
		LastName:        invitation.LastName,
		Position:        invitation.Position,
		CreatedBy:       operatorInvitationCreatedByValue(invitation.CreatedBy),
		DeliveryStatus:  operatorInvitationDeliveryStatus(invitation.EmailSentAt, invitation.EmailError),
		EmailSentAt:     invitation.EmailSentAt,
		EmailError:      invitation.EmailError,
		EmailRetryCount: invitation.EmailRetryCount,
	}
	if invitation.Role != nil {
		resp.RoleName = invitation.Role.Name
	}
	if shouldExposeSeedInvitationToken(r) {
		resp.Token = &invitation.Token
	}
	if invitation.Creator != nil {
		resp.Creator = invitation.Creator.Email
	}
	common.Respond(w, r, http.StatusCreated, resp, "School admin invitation created successfully")
}

// ProvisioningErrorRenderer maps provisioning errors to HTTP responses.
func ProvisioningErrorRenderer(err error) render.Renderer {
	var invalidData *platformSvc.InvalidDataError
	var conflictErr *platformSvc.ConflictError
	var organizationNotFound *platformSvc.OrganizationNotFoundError
	var schoolNotFound *platformSvc.SchoolNotFoundError
	var authErr *authSvc.AuthError

	if errors.As(err, &authErr) && authErr.Err != nil {
		var dbErr *modelBase.DatabaseError
		switch {
		case errors.Is(authErr.Err, authSvc.ErrEmailAlreadyExists):
			return ErrConflict(authSvc.ErrEmailAlreadyExists.Error())
		case errors.Is(authErr.Err, authSvc.ErrInvitationNameRequired),
			errors.Is(authErr.Err, authSvc.ErrPasswordMismatch),
			errors.Is(authErr.Err, authSvc.ErrPasswordTooWeak),
			authErr.Op == "create invitation" && !errors.As(authErr.Err, &dbErr):
			return ErrInvalidRequest(authErr.Err)
		default:
			return ErrInternal("An error occurred")
		}
	}

	switch {
	case errors.As(err, &invalidData):
		return ErrInvalidRequest(errors.New("invalid input data"))
	case errors.As(err, &conflictErr):
		return ErrConflict(conflictErr.Err.Error())
	case errors.As(err, &organizationNotFound):
		return ErrNotFound("Organization not found")
	case errors.As(err, &schoolNotFound):
		return ErrNotFound("School not found")
	default:
		return ErrInternal("An error occurred")
	}
}

func operatorInvitationCreatedByValue(createdBy *int64) int64 {
	if createdBy == nil {
		return 0
	}
	return *createdBy
}

func operatorInvitationDeliveryStatus(sentAt *time.Time, emailError *string) string {
	if sentAt != nil {
		return "sent"
	}
	if emailError != nil && strings.TrimSpace(*emailError) != "" {
		return "failed"
	}
	return "pending"
}

func shouldExposeSeedInvitationToken(r *http.Request) bool {
	if !strings.EqualFold(strings.TrimSpace(r.Header.Get(seedTokenHeader)), "true") {
		return false
	}
	return strings.ToLower(strings.TrimSpace(viper.GetString("app_env"))) != "production"
}
