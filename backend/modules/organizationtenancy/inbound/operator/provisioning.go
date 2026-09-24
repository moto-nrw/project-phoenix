// Package operator serves the operator provisioning routes of Organisation &
// Tenancy (#3232): organisations, schools, devices, persons, school accounts
// with their caregiver capability, and the overview statistics. The operator
// router in api/operator mounts these handlers behind its middleware chain,
// so the operator wire format and authorization stay unchanged.
package operator

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

const (
	internalErrorMessage   = "An error occurred"
	invalidDeviceIDMessage = "invalid device ID"
)

// ProvisioningResource handles operator tenant provisioning endpoints.
type ProvisioningResource struct {
	service                    organizationtenancy.Provisioning
	CaregiverCapabilityService SchoolAccountCaregivers
	appEnv                     string
}

// ProvisioningConfig holds the provisioning routes' dependencies.
type ProvisioningConfig struct {
	Service organizationtenancy.Provisioning
	// CaregiverCapability serves the caregiver capability routes of school
	// accounts. Without it those routes answer 500.
	CaregiverCapability SchoolAccountCaregivers
	// AppEnv decides whether a seeding client may read invitation tokens.
	AppEnv string
}

// NewProvisioningResource creates a new provisioning resource.
func NewProvisioningResource(cfg ProvisioningConfig) *ProvisioningResource {
	return &ProvisioningResource{
		service:                    cfg.Service,
		CaregiverCapabilityService: cfg.CaregiverCapability,
		appEnv:                     cfg.AppEnv,
	}
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
	Hidden         bool   `json:"hidden,omitempty"`
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

type inviteSchoolAdminRequest struct {
	Email            string `json:"email"`
	FirstName        string `json:"first_name,omitempty"`
	LastName         string `json:"last_name,omitempty"`
	Position         string `json:"position,omitempty"`
	CaregiverEnabled bool   `json:"caregiver_enabled,omitempty"`
}

type operatorInvitationResponse struct {
	ID               int64      `json:"id"`
	Email            string     `json:"email"`
	RoleID           int64      `json:"role_id"`
	RoleName         string     `json:"role_name,omitempty"`
	Token            *string    `json:"token,omitempty"`
	ExpiresAt        time.Time  `json:"expires_at"`
	FirstName        *string    `json:"first_name,omitempty"`
	LastName         *string    `json:"last_name,omitempty"`
	Position         *string    `json:"position,omitempty"`
	CaregiverEnabled bool       `json:"caregiver_enabled"`
	CreatedBy        int64      `json:"created_by"`
	Creator          string     `json:"creator,omitempty"`
	DeliveryStatus   string     `json:"delivery_status"`
	EmailSentAt      *time.Time `json:"email_sent_at,omitempty"`
	EmailError       *string    `json:"email_error,omitempty"`
	EmailRetryCount  int        `json:"email_retry_count"`
}

func (req *inviteSchoolAdminRequest) Bind(_ *http.Request) error {
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)
	req.Position = strings.TrimSpace(req.Position)
	return nil
}

type createSchoolAccountRequest struct {
	Email            string         `json:"email"`
	FirstName        string         `json:"first_name"`
	LastName         string         `json:"last_name"`
	Password         string         `json:"password"`
	ConfirmPassword  string         `json:"confirm_password"`
	RoleID           *common.JSONID `json:"role_id,omitempty"`
	Position         string         `json:"position,omitempty"`
	CaregiverEnabled bool           `json:"caregiver_enabled,omitempty"`
}

type updateCaregiverCapabilityRequest struct {
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	Position  string `json:"position,omitempty"`
}

func (req *createSchoolAccountRequest) Bind(_ *http.Request) error {
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)
	req.Position = strings.TrimSpace(req.Position)
	if req.Email == "" {
		return errors.New("email is required")
	}
	if req.FirstName == "" {
		return errors.New("first name is required")
	}
	if req.LastName == "" {
		return errors.New("last name is required")
	}
	if req.Password == "" {
		return errors.New("password is required")
	}
	if req.Password != req.ConfirmPassword {
		return errors.New("passwords do not match")
	}
	return nil
}

func (req *updateCaregiverCapabilityRequest) Bind(_ *http.Request) error {
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)
	req.Position = strings.TrimSpace(req.Position)
	return nil
}

func (rs *ProvisioningResource) CreateOrganization(w http.ResponseWriter, r *http.Request) {
	req := &createOrganizationRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.OperatorInvalidRequest(err))
		return
	}
	operatorID := int64(jwt.ClaimsFromCtx(r.Context()).ID)
	org := &organizationtenancy.CreateOrganization{Name: req.Name, Slug: req.Slug, Active: true}
	created, err := rs.service.CreateOrganization(r.Context(), org, operatorID, common.ParseClientIP(r))
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
		common.RenderError(w, r, common.OperatorInvalidRequest(err))
		return
	}
	operatorID := int64(jwt.ClaimsFromCtx(r.Context()).ID)
	school := &organizationtenancy.CreateSchool{
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
		Hidden:         req.Hidden,
	}
	created, err := rs.service.CreateSchool(r.Context(), school, operatorID, common.ParseClientIP(r))
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
		common.RenderError(w, r, common.OperatorInvalidRequest(err))
		return
	}
	orgID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid organization ID")
	if !ok {
		return
	}
	operatorID := int64(jwt.ClaimsFromCtx(r.Context()).ID)
	svcReq := organizationtenancy.OrganizationChanges{
		Name:   req.Name,
		Slug:   req.Slug,
		Active: req.Active,
	}
	updated, err := rs.service.UpdateOrganization(r.Context(), orgID, svcReq, operatorID, common.ParseClientIP(r))
	if err != nil {
		common.RenderError(w, r, ProvisioningErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, updated, "Organization updated successfully")
}

func (rs *ProvisioningResource) UpdateSchool(w http.ResponseWriter, r *http.Request) {
	req := &updateSchoolRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.OperatorInvalidRequest(err))
		return
	}
	schoolID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid school ID")
	if !ok {
		return
	}
	operatorID := int64(jwt.ClaimsFromCtx(r.Context()).ID)
	svcReq := organizationtenancy.SchoolChanges{
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
		Hidden:         req.Hidden,
		ChildQuota:     req.ChildQuota.change(),
	}
	updated, err := rs.service.UpdateSchool(r.Context(), schoolID, svcReq, operatorID, common.ParseClientIP(r))
	if err != nil {
		common.RenderError(w, r, ProvisioningErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, updated, "School updated successfully")
}

func (rs *ProvisioningResource) SoftDeleteSchool(w http.ResponseWriter, r *http.Request) {
	schoolID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid school ID")
	if !ok {
		return
	}
	operatorID := int64(jwt.ClaimsFromCtx(r.Context()).ID)
	if err := rs.service.SoftDeleteSchool(r.Context(), schoolID, operatorID, common.ParseClientIP(r)); err != nil {
		common.RenderError(w, r, ProvisioningErrorRenderer(err))
		return
	}
	common.RespondNoContent(w, r)
}

func (rs *ProvisioningResource) RestoreSchool(w http.ResponseWriter, r *http.Request) {
	schoolID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid school ID")
	if !ok {
		return
	}
	operatorID := int64(jwt.ClaimsFromCtx(r.Context()).ID)
	if err := rs.service.RestoreSchool(r.Context(), schoolID, operatorID, common.ParseClientIP(r)); err != nil {
		common.RenderError(w, r, ProvisioningErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, nil, "School restored successfully")
}

func (rs *ProvisioningResource) SoftDeleteOrganization(w http.ResponseWriter, r *http.Request) {
	orgID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid organization ID")
	if !ok {
		return
	}
	operatorID := int64(jwt.ClaimsFromCtx(r.Context()).ID)
	if err := rs.service.SoftDeleteOrganization(r.Context(), orgID, operatorID, common.ParseClientIP(r)); err != nil {
		common.RenderError(w, r, ProvisioningErrorRenderer(err))
		return
	}
	common.RespondNoContent(w, r)
}

func (rs *ProvisioningResource) RestoreOrganization(w http.ResponseWriter, r *http.Request) {
	orgID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid organization ID")
	if !ok {
		return
	}
	operatorID := int64(jwt.ClaimsFromCtx(r.Context()).ID)
	if err := rs.service.RestoreOrganization(r.Context(), orgID, operatorID, common.ParseClientIP(r)); err != nil {
		common.RenderError(w, r, ProvisioningErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, nil, "Organization restored successfully")
}

func (rs *ProvisioningResource) InviteSchoolAdmin(w http.ResponseWriter, r *http.Request) {
	req := &inviteSchoolAdminRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.OperatorInvalidRequest(err))
		return
	}
	schoolID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid school ID")
	if !ok {
		return
	}
	operatorID := int64(jwt.ClaimsFromCtx(r.Context()).ID)
	invitationReq := organizationtenancy.SchoolAdminInvitationInput{
		Email:            req.Email,
		CaregiverEnabled: req.CaregiverEnabled,
	}
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
	invitation, err := rs.service.InviteSchoolAdmin(r.Context(), schoolID, operatorID, common.ParseClientIP(r), invitationReq)
	if err != nil {
		common.RenderError(w, r, ProvisioningErrorRenderer(err))
		return
	}
	resp := operatorInvitationResponse{
		ID:               invitation.ID,
		Email:            invitation.Email,
		RoleID:           invitation.RoleID,
		ExpiresAt:        invitation.ExpiresAt,
		FirstName:        invitation.FirstName,
		LastName:         invitation.LastName,
		Position:         invitation.Position,
		CaregiverEnabled: invitation.CaregiverEnabled,
		CreatedBy:        operatorInvitationCreatedByValue(invitation.CreatedBy),
		DeliveryStatus:   operatorInvitationDeliveryStatus(invitation.EmailSentAt, invitation.EmailError),
		EmailSentAt:      invitation.EmailSentAt,
		EmailError:       invitation.EmailError,
		EmailRetryCount:  invitation.EmailRetryCount,
	}
	resp.RoleName = invitation.RoleName
	if shouldExposeSeedInvitationToken(r, rs.appEnv) {
		resp.Token = &invitation.Token
	}
	resp.Creator = invitation.CreatorEmail
	common.Respond(w, r, http.StatusCreated, resp, "School admin invitation created successfully")
}

func (rs *ProvisioningResource) CreateSchoolAccount(w http.ResponseWriter, r *http.Request) {
	req := &createSchoolAccountRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.OperatorInvalidRequest(err))
		return
	}
	schoolID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid school ID")
	if !ok {
		return
	}
	operatorID := int64(jwt.ClaimsFromCtx(r.Context()).ID)
	svcReq := organizationtenancy.SchoolAccountInput{
		Email:            req.Email,
		Password:         req.Password,
		FirstName:        req.FirstName,
		LastName:         req.LastName,
		RoleID:           jsonIDPointer(req.RoleID),
		Position:         req.Position,
		CaregiverEnabled: req.CaregiverEnabled,
	}
	account, err := rs.service.CreateSchoolAccount(r.Context(), schoolID, operatorID, common.ParseClientIP(r), svcReq)
	if err != nil {
		common.RenderError(w, r, ProvisioningErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusCreated, account, "School account created successfully")
}

func (rs *ProvisioningResource) ListSystemRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := rs.service.ListSystemRoles(r.Context())
	if err != nil {
		common.RenderError(w, r, ProvisioningErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, roles, "System roles retrieved successfully")
}

func jsonIDPointer(id *common.JSONID) *int64 {
	if id == nil {
		return nil
	}
	value := id.Int64()
	return &value
}

func (rs *ProvisioningResource) ListSchoolAccounts(w http.ResponseWriter, r *http.Request) {
	common.IDFetch("id", "invalid school ID", rs.service.ListSchoolAccounts, ProvisioningErrorRenderer, "School accounts retrieved successfully")(w, r)
}

func (rs *ProvisioningResource) GetSchoolAccountCaregiverCapability(w http.ResponseWriter, r *http.Request) {
	if rs.CaregiverCapabilityService == nil {
		common.RenderError(w, r, common.OperatorInternal("Caregiver capability service is not configured"))
		return
	}

	schoolID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid school ID")
	if !ok {
		return
	}
	accountID, ok := common.ParseInt64IDWithError(w, r, "accountId", "invalid account ID")
	if !ok {
		return
	}

	state, err := rs.CaregiverCapabilityService.GetSchoolAccountCaregiverCapability(r.Context(), schoolID, accountID)
	if err != nil {
		common.RenderError(w, r, caregiverCapabilityProvisioningErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, state, "Caregiver capability retrieved successfully")
}

func (rs *ProvisioningResource) EnableSchoolAccountCaregiverCapability(w http.ResponseWriter, r *http.Request) {
	if rs.CaregiverCapabilityService == nil {
		common.RenderError(w, r, common.OperatorInternal("Caregiver capability service is not configured"))
		return
	}

	schoolID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid school ID")
	if !ok {
		return
	}
	accountID, ok := common.ParseInt64IDWithError(w, r, "accountId", "invalid account ID")
	if !ok {
		return
	}

	req := &updateCaregiverCapabilityRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.OperatorInvalidRequest(err))
		return
	}

	state, err := rs.CaregiverCapabilityService.EnableSchoolAccountCaregiverCapability(
		r.Context(),
		schoolID,
		accountID,
		req.FirstName,
		req.LastName,
		req.Position,
	)
	if err != nil {
		common.RenderError(w, r, caregiverCapabilityProvisioningErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, state, "Caregiver capability enabled successfully")
}

func (rs *ProvisioningResource) DisableSchoolAccountCaregiverCapability(w http.ResponseWriter, r *http.Request) {
	if rs.CaregiverCapabilityService == nil {
		common.RenderError(w, r, common.OperatorInternal("Caregiver capability service is not configured"))
		return
	}

	schoolID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid school ID")
	if !ok {
		return
	}
	accountID, ok := common.ParseInt64IDWithError(w, r, "accountId", "invalid account ID")
	if !ok {
		return
	}

	state, err := rs.CaregiverCapabilityService.DisableSchoolAccountCaregiverCapability(r.Context(), schoolID, accountID)
	if err != nil {
		common.RenderError(w, r, caregiverCapabilityProvisioningErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, state, "Caregiver capability disabled successfully")
}

func (rs *ProvisioningResource) ListOrganizationAccounts(w http.ResponseWriter, r *http.Request) {
	common.IDFetch("id", "invalid organization ID", rs.service.ListOrganizationAccounts, ProvisioningErrorRenderer, "Organization accounts retrieved successfully")(w, r)
}

func (rs *ProvisioningResource) ListAllAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := rs.service.ListAllAccounts(r.Context())
	if err != nil {
		common.RenderError(w, r, ProvisioningErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, accounts, "All accounts retrieved successfully")
}

// ProvisioningErrorRenderer maps provisioning errors to HTTP responses.
func ProvisioningErrorRenderer(err error) render.Renderer {
	var invalidProvisioningData *organizationtenancy.InvalidProvisioningDataError
	var provisioningConflict *organizationtenancy.ProvisioningConflictError
	var organizationNotFound *organizationtenancy.OrganizationNotFoundError
	var organizationAlreadyDeleted *organizationtenancy.OrganizationAlreadyDeletedError
	var organizationNotDeleted *organizationtenancy.OrganizationNotDeletedError
	var organizationHasSchools *organizationtenancy.OrganizationHasSchoolsError
	var organizationDeleted *organizationtenancy.OrganizationDeletedError
	var schoolNotFound *organizationtenancy.SchoolNotFoundError
	var schoolInactive *organizationtenancy.SchoolInactiveError
	var schoolAlreadyDeleted *organizationtenancy.SchoolAlreadyDeletedError
	var schoolNotDeleted *organizationtenancy.SchoolNotDeletedError
	var operatorDeviceNotFound *organizationtenancy.OperatorDeviceNotFoundError
	var deviceInUse *organizationtenancy.DeviceInUseError
	var deviceProtected *organizationtenancy.DeviceProtectedError
	var deviceTransferProtected *organizationtenancy.DeviceTransferProtectedError
	var deviceTransferBlocked *organizationtenancy.DeviceTransferBlockedError
	var deviceTransferOrganizationMismatch *organizationtenancy.DeviceTransferOrganizationMismatchError
	var deviceTransferSameSchool *organizationtenancy.DeviceTransferSameSchoolError
	var personNotFound *organizationtenancy.PersonNotFoundError
	var personActiveSupervisors *organizationtenancy.PersonHasActiveSupervisionsError
	var identityErr *organizationtenancy.ProvisioningIdentityError

	if errors.As(err, &identityErr) && identityErr.Err != nil {
		switch {
		case errors.Is(identityErr.Err, organizationtenancy.ErrAccountEmailExists):
			return common.OperatorConflict(organizationtenancy.ErrAccountEmailExists.Error())
		case errors.Is(identityErr.Err, organizationtenancy.ErrAccountUsernameExists):
			return common.OperatorConflict(organizationtenancy.ErrAccountUsernameExists.Error())
		case errors.Is(identityErr.Err, organizationtenancy.ErrInvitationNameRequired),
			errors.Is(identityErr.Err, organizationtenancy.ErrPasswordMismatch),
			errors.Is(identityErr.Err, organizationtenancy.ErrPasswordTooWeak),
			identityErr.Op == organizationtenancy.OpCreateInvitation && !errors.Is(identityErr.Err, organizationtenancy.ErrIdentityStoreFailed):
			return common.OperatorInvalidRequest(identityErr.Err)
		default:
			return common.OperatorInternal(internalErrorMessage)
		}
	}

	switch {
	case errors.As(err, &invalidProvisioningData):
		return common.OperatorInvalidRequest(errors.New("invalid input data"))
	case errors.As(err, &provisioningConflict):
		return common.OperatorConflict(provisioningConflict.Err.Error())
	case errors.As(err, &organizationNotFound):
		return common.OperatorNotFound("Organization not found")
	case errors.As(err, &organizationAlreadyDeleted):
		return common.OperatorConflict("Organization is already deleted")
	case errors.As(err, &organizationNotDeleted):
		return common.OperatorConflict("Organization is not deleted")
	case errors.As(err, &organizationHasSchools):
		return common.OperatorConflict(fmt.Sprintf("Organization has %d existing school(s) and cannot be deleted. Delete all schools first.", organizationHasSchools.SchoolCount))
	case errors.As(err, &organizationDeleted):
		return common.OperatorConflict("Organization is deleted and cannot host schools")
	case errors.As(err, &schoolNotFound):
		return common.OperatorNotFound("School not found")
	case errors.As(err, &schoolInactive):
		return common.OperatorForbidden("School is inactive")
	case errors.As(err, &schoolAlreadyDeleted):
		return common.OperatorConflict("School is already deleted")
	case errors.As(err, &schoolNotDeleted):
		return common.OperatorConflict("School is not deleted")
	case errors.As(err, &operatorDeviceNotFound):
		return common.OperatorNotFound("Device not found")
	case errors.As(err, &deviceInUse):
		return common.OperatorConflict("Device is still referenced by attendance or session records and cannot be deleted")
	case errors.As(err, &deviceProtected):
		return common.OperatorForbidden("This system device cannot be deleted")
	case errors.As(err, &deviceTransferProtected):
		return common.OperatorForbidden("This system device cannot be transferred")
	case errors.As(err, &deviceTransferBlocked):
		switch deviceTransferBlocked.Reason {
		case organizationtenancy.DeviceTransferBlockedOnline:
			return common.OperatorConflict("Device is online and cannot be transferred")
		case organizationtenancy.DeviceTransferBlockedActiveSession:
			return common.OperatorConflict("Device has an active session and cannot be transferred")
		default:
			return common.OperatorInternal(internalErrorMessage)
		}
	case errors.As(err, &deviceTransferOrganizationMismatch):
		return common.OperatorForbidden("Device can only be transferred within its organization")
	case errors.As(err, &deviceTransferSameSchool):
		return common.OperatorConflict("Device already belongs to the target school")
	case errors.As(err, &personNotFound):
		return common.OperatorNotFound("Person not found")
	case errors.As(err, &personActiveSupervisors):
		return common.OperatorConflict("Person has active supervisions and cannot be deleted")
	default:
		return common.OperatorInternal(internalErrorMessage)
	}
}

func caregiverCapabilityProvisioningErrorRenderer(err error) render.Renderer {
	if blocked, ok := errors.AsType[caregiverCapabilityBlocked](err); ok {
		return common.NewCaregiverCapabilityBlockedResponse(
			http.StatusConflict,
			blocked.Error(),
			blocked.CaregiverCapabilityBlockers(),
		)
	}
	if _, missing := errors.AsType[caregiverAccountMissing](err); missing || errors.Is(err, organizationtenancy.ErrAccountNotFound) {
		return common.OperatorNotFound("Account not found")
	}
	if invalidProvisioningData, ok := errors.AsType[*organizationtenancy.InvalidProvisioningDataError](err); ok {
		return common.OperatorInvalidRequest(invalidProvisioningData.Err)
	}
	if invalid, ok := errors.AsType[caregiverRequestInvalid](err); ok {
		return common.OperatorInvalidRequest(errors.Unwrap(invalid.CaregiverRequestInvalid()))
	}
	return ProvisioningErrorRenderer(err)
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

func (rs *ProvisioningResource) ListAllDevices(w http.ResponseWriter, r *http.Request) {
	devices, err := rs.service.ListAllDevices(r.Context())
	if err != nil {
		common.RenderError(w, r, ProvisioningErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, devices, "All devices retrieved successfully")
}

func (rs *ProvisioningResource) ListSchoolDevices(w http.ResponseWriter, r *http.Request) {
	common.IDFetch("id", "invalid school ID", rs.service.ListSchoolDevices, ProvisioningErrorRenderer, "School devices retrieved successfully")(w, r)
}

func (rs *ProvisioningResource) ListOrganizationDevices(w http.ResponseWriter, r *http.Request) {
	common.IDFetch("id", "invalid organization ID", rs.service.ListOrganizationDevices, ProvisioningErrorRenderer, "Organization devices retrieved successfully")(w, r)
}

type createDeviceRequest struct {
	SchoolID   int64  `json:"school_id"`
	DeviceID   string `json:"device_id"`
	DeviceType string `json:"device_type"`
	Name       string `json:"name,omitempty"`
	APIKey     string `json:"api_key,omitempty"`
}

func (req *createDeviceRequest) Bind(_ *http.Request) error {
	req.DeviceID = strings.TrimSpace(req.DeviceID)
	req.DeviceType = strings.TrimSpace(req.DeviceType)
	req.Name = strings.TrimSpace(req.Name)
	req.APIKey = strings.TrimSpace(req.APIKey)
	if req.SchoolID <= 0 {
		return errors.New("school_id is required")
	}
	if req.DeviceID == "" {
		return errors.New("device_id is required")
	}
	if req.DeviceType == "" {
		return errors.New("device_type is required")
	}
	return nil
}

type setDeviceAPIKeyRequest struct {
	APIKey string `json:"api_key,omitempty"`
}

type transferDeviceRequest struct {
	TargetSchoolID int64 `json:"target_school_id"`
}

func (req *transferDeviceRequest) Bind(_ *http.Request) error {
	if req.TargetSchoolID <= 0 {
		return errors.New("target_school_id is required")
	}
	return nil
}

func (req *setDeviceAPIKeyRequest) Bind(_ *http.Request) error {
	req.APIKey = strings.TrimSpace(req.APIKey)
	return nil
}

func (rs *ProvisioningResource) CreateDevice(w http.ResponseWriter, r *http.Request) {
	req := &createDeviceRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.OperatorInvalidRequest(err))
		return
	}
	operatorID := int64(jwt.ClaimsFromCtx(r.Context()).ID)
	var name *string
	if req.Name != "" {
		name = &req.Name
	}
	var apiKey *string
	if req.APIKey != "" {
		apiKey = &req.APIKey
	}
	device, err := rs.service.CreateDevice(r.Context(), req.SchoolID, req.DeviceID, req.DeviceType, name, apiKey, operatorID, common.ParseClientIP(r))
	if err != nil {
		common.RenderError(w, r, ProvisioningErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusCreated, device, "Device created successfully")
}

func (rs *ProvisioningResource) SetDeviceAPIKey(w http.ResponseWriter, r *http.Request) {
	deviceID, ok := common.ParseInt64IDWithError(w, r, "id", invalidDeviceIDMessage)
	if !ok {
		return
	}
	req := &setDeviceAPIKeyRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.OperatorInvalidRequest(err))
		return
	}
	operatorID := int64(jwt.ClaimsFromCtx(r.Context()).ID)
	var apiKey *string
	if req.APIKey != "" {
		apiKey = &req.APIKey
	}
	device, err := rs.service.SetDeviceAPIKey(r.Context(), deviceID, apiKey, operatorID, common.ParseClientIP(r))
	if err != nil {
		common.RenderError(w, r, ProvisioningErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, device, "Device API key updated successfully")
}

func (rs *ProvisioningResource) DeleteDevice(w http.ResponseWriter, r *http.Request) {
	deviceID, ok := common.ParseInt64IDWithError(w, r, "id", invalidDeviceIDMessage)
	if !ok {
		return
	}
	operatorID := int64(jwt.ClaimsFromCtx(r.Context()).ID)
	err := rs.service.DeleteDevice(r.Context(), deviceID, operatorID, common.ParseClientIP(r))
	if err != nil {
		common.RenderError(w, r, ProvisioningErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, nil, "Device deleted successfully")
}

func (rs *ProvisioningResource) GetDeviceTransferStatus(w http.ResponseWriter, r *http.Request) {
	deviceID, ok := common.ParseInt64IDWithError(w, r, "id", invalidDeviceIDMessage)
	if !ok {
		return
	}
	status, err := rs.service.GetDeviceTransferStatus(r.Context(), deviceID)
	if err != nil {
		common.RenderError(w, r, ProvisioningErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, status, "Device transfer status retrieved successfully")
}

func (rs *ProvisioningResource) TransferDevice(w http.ResponseWriter, r *http.Request) {
	deviceID, ok := common.ParseInt64IDWithError(w, r, "id", invalidDeviceIDMessage)
	if !ok {
		return
	}
	req := &transferDeviceRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.OperatorInvalidRequest(err))
		return
	}
	operatorID := int64(jwt.ClaimsFromCtx(r.Context()).ID)
	device, err := rs.service.TransferDevice(r.Context(), deviceID, req.TargetSchoolID, operatorID, common.ParseClientIP(r))
	if err != nil {
		common.RenderError(w, r, ProvisioningErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, device, "Device transferred successfully")
}

func (rs *ProvisioningResource) ListSchoolPersons(w http.ResponseWriter, r *http.Request) {
	common.IDFetch("id", "invalid school ID", rs.service.ListSchoolPersons, ProvisioningErrorRenderer, "School persons retrieved successfully")(w, r)
}

func (rs *ProvisioningResource) SoftDeletePerson(w http.ResponseWriter, r *http.Request) {
	personID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid person ID")
	if !ok {
		return
	}
	operatorID := int64(jwt.ClaimsFromCtx(r.Context()).ID)
	err := rs.service.SoftDeletePerson(r.Context(), personID, operatorID, common.ParseClientIP(r))
	if err != nil {
		common.RenderError(w, r, ProvisioningErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, nil, "Person deleted successfully")
}

func shouldExposeSeedInvitationToken(r *http.Request, appEnv string) bool {
	return common.ShouldExposeSeedInvitationToken(r, appEnv)
}
