package traegerportal

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// The names of the Träger-Büro (#4693). A person with "Konten verwalten"
// names a colleague at /traeger/accounts/{accountId}/name; every signed-in
// person reads and changes their own name at /traeger/profile without that
// right. The export protocol and the history of an invoice show the name
// instead of the address.

var errProfilesUnavailable = errors.New("the Träger-Büro names are not composed")

// profileRouter returns the routes mounted at /traeger/profile. They need
// the Träger session only.
func (rs *Resource) profileRouter(protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	protectedTraegerRoutes(r, func(r chi.Router, _ common.Middleware) {
		r.Get("/", rs.ownProfile)
		r.Put("/name", rs.changeOwnName)
	})
	return r
}

// NameRequest is the body of PUT /traeger/profile/name and PUT
// /traeger/accounts/{accountId}/name. Identity & Access validates the name.
type NameRequest struct {
	Name string `json:"name"`
}

// Bind accepts the body.
func (req *NameRequest) Bind(_ *http.Request) error { return nil }

// ProfileResponse is the signed-in person at the Träger of the session.
type ProfileResponse struct {
	AccountID int64  `json:"account_id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
}

func (rs *Resource) ownProfile(w http.ResponseWriter, r *http.Request) {
	actor, ok := rs.profileActor(w, r)
	if !ok {
		return
	}
	member, err := rs.profiles.OwnTraegerProfile(r.Context(), actor)
	if err != nil {
		common.RenderError(w, r, renderAccountError(err))
		return
	}
	common.Respond(w, r, http.StatusOK, profileResponse(member), "Profile retrieved successfully")
}

func (rs *Resource) changeOwnName(w http.ResponseWriter, r *http.Request) {
	actor, ok := rs.profileActor(w, r)
	if !ok {
		return
	}
	req := &NameRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	member, err := rs.profiles.ChangeOwnTraegerName(r.Context(), actor, req.Name)
	if err != nil {
		common.RenderError(w, r, renderAccountError(err))
		return
	}
	common.Respond(w, r, http.StatusOK, profileResponse(member), "Name changed")
}

func (rs *Resource) changeMemberName(w http.ResponseWriter, r *http.Request) {
	actor, ok := rs.profileActor(w, r)
	if !ok {
		return
	}
	accountID, err := common.ParseIDParam(r, "accountId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	req := &NameRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	member, err := rs.profiles.ChangeTraegerMemberName(r.Context(), actor, accountID, req.Name)
	if err != nil {
		common.RenderError(w, r, renderAccountError(err))
		return
	}
	common.Respond(w, r, http.StatusOK, memberResponse(member), "Name changed")
}

// profileActor reads the acting person from the Träger session.
func (rs *Resource) profileActor(w http.ResponseWriter, r *http.Request) (identityaccess.TraegerActor, bool) {
	if rs.profiles == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errProfilesUnavailable))
		return identityaccess.TraegerActor{}, false
	}
	return sessionActor(w, r)
}

func profileResponse(member identityaccess.TraegerMember) ProfileResponse {
	return ProfileResponse{AccountID: member.AccountID, Email: member.Email, Name: member.Name}
}
