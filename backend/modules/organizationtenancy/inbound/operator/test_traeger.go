package operator

import (
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

// The test system of a new Träger (#4658): the operator creates a test Träger
// with its test schools; the sample children follow at Data Import.

type createTestTraegerRequest struct {
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	SchoolCount  int    `json:"school_count"`
	ContactEmail string `json:"contact_email"`
}

func (*createTestTraegerRequest) Bind(*http.Request) error { return nil }

// CreateTestTraeger answers POST /operator/organizations/test-traeger.
func (rs *ProvisioningResource) CreateTestTraeger(w http.ResponseWriter, r *http.Request) {
	req := &createTestTraegerRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.OperatorInvalidRequest(err))
		return
	}
	created, err := rs.service.CreateTestTraeger(r.Context(), organizationtenancy.CreateTestTraeger{
		Name: req.Name, Slug: req.Slug, SchoolCount: req.SchoolCount, ContactEmail: req.ContactEmail,
	}, int64(sessiontoken.ClaimsFromCtx(r.Context()).ID), common.ParseClientIP(r))
	if err != nil {
		common.RenderError(w, r, ProvisioningErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusCreated, created, "Test Träger created successfully")
}
