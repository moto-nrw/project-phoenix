package catererportalhttp

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/workflows/catererportal"
)

// InvitationResponse is a school's invitation to its caterer. The link is
// never part of it: it goes to the caterer by mail only.
type InvitationResponse struct {
	ID              string     `json:"id"`
	Email           string     `json:"email"`
	CatererName     string     `json:"caterer_name"`
	DeliveryAddress string     `json:"delivery_address"`
	Status          string     `json:"status"`
	ExpiresAt       time.Time  `json:"expires_at"`
	AcceptedAt      *time.Time `json:"accepted_at,omitempty"`
	RevokedAt       *time.Time `json:"revoked_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// SupplyResponse is the school's running supply.
type SupplyResponse struct {
	CatererName     string    `json:"caterer_name"`
	DeliveryAddress string    `json:"delivery_address"`
	StartedAt       time.Time `json:"started_at"`
}

// SchoolCatererResponse is the school's caterer page.
type SchoolCatererResponse struct {
	Supply      *SupplyResponse      `json:"supply"`
	Invitations []InvitationResponse `json:"invitations"`
}

type inviteRequest struct {
	Email           string `json:"email"`
	CatererName     string `json:"caterer_name"`
	DeliveryAddress string `json:"delivery_address"`
}

func invitationResponse(invitation identityaccess.CatererInvitation, now time.Time) InvitationResponse {
	status := "open"
	switch {
	case invitation.AcceptedAt != nil:
		status = "accepted"
	case invitation.RevokedAt != nil:
		status = "revoked"
	case !invitation.ExpiresAt.After(now):
		status = "expired"
	}
	return InvitationResponse{
		ID: strconv.FormatInt(invitation.ID, 10), Email: invitation.Email, CatererName: invitation.CatererName,
		DeliveryAddress: invitation.DeliveryAddress, Status: status, ExpiresAt: invitation.ExpiresAt,
		AcceptedAt: invitation.AcceptedAt, RevokedAt: invitation.RevokedAt, CreatedAt: invitation.CreatedAt,
	}
}

func schoolCatererResponse(caterer catererportal.SchoolCaterer, now time.Time) SchoolCatererResponse {
	response := SchoolCatererResponse{Invitations: make([]InvitationResponse, 0, len(caterer.Invitations))}
	for _, invitation := range caterer.Invitations {
		response.Invitations = append(response.Invitations, invitationResponse(invitation, now))
	}
	if caterer.Supply != nil {
		response.Supply = &SupplyResponse{
			CatererName: caterer.Supply.CatererName, DeliveryAddress: caterer.Supply.DeliveryAddress, StartedAt: caterer.Supply.StartedAt,
		}
	}
	return response
}

// getSchoolCaterer returns the school's running supply and its invitations.
func (rs *Resource) getSchoolCaterer(w http.ResponseWriter, r *http.Request) {
	caterer, err := rs.portal.SchoolCaterer(r.Context())
	if err != nil {
		renderFailure(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, schoolCatererResponse(caterer, time.Now()), "Caterer retrieved")
}

// inviteCaterer mails the school's invitation to its caterer.
func (rs *Resource) inviteCaterer(w http.ResponseWriter, r *http.Request) {
	var request inviteRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}
	invitation, err := rs.portal.CreateCatererInvitation(r.Context(), identityaccess.CatererInvitationRequest{
		Email: request.Email, CatererName: request.CatererName, DeliveryAddress: request.DeliveryAddress,
		CreatedBy: rs.currentAccountID(r.Context()),
	})
	if err != nil {
		renderFailure(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, invitationResponse(invitation, time.Now()), "Caterer invited")
}

// revokeInvitation takes an open invitation back.
func (rs *Resource) revokeInvitation(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "invitationId")
	if !ok {
		common.RenderError(w, r, common.ErrorInvalidRequest(errInvalidID))
		return
	}
	if err := rs.portal.RevokeCatererInvitation(r.Context(), id); err != nil {
		renderFailure(w, r, err)
		return
	}
	common.RespondNoContent(w, r)
}

// endSupply ends the school's supply; the caterer loses the school at once.
func (rs *Resource) endSupply(w http.ResponseWriter, r *http.Request) {
	if _, err := rs.portal.EndSupply(r.Context(), rs.currentAccountID(r.Context())); err != nil {
		renderFailure(w, r, err)
		return
	}
	common.RespondNoContent(w, r)
}
