package traegerstaff

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/render"

	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/modules/workforce"
	"github.com/moto-ogs/project-phoenix/tenant"
)

// The kinds of a release change the operator audit records.
const (
	ReleaseGranted   = "granted"
	ReleaseWithdrawn = "withdrawn"
)

// ReleaseAudit records the operator audit entry of a release change in the
// caller's administrative transaction and answers false after writing the
// error response. The root binds it to the operator audit of Organisation &
// Tenancy.
type ReleaseAudit func(w http.ResponseWriter, r *http.Request, kind, representation string) bool

// ReleaseResource serves the release „Personal beim Träger“ of one Träger to
// the moto team (ADR 0089 section 2): its state, the release with the
// statement the Träger confirmed, and the withdrawal. Every call runs in one
// administrative transaction; a change writes its operator audit entry in
// that transaction before it changes anything.
type ReleaseResource struct {
	releases   workforce.TraegerStaffReleases
	audit      ReleaseAudit
	operatorID func(context.Context) int64
}

// NewReleaseResource builds the resource; operatorID names the operator of
// the session.
func NewReleaseResource(releases workforce.TraegerStaffReleases, audit ReleaseAudit, operatorID func(context.Context) int64) *ReleaseResource {
	if releases == nil || audit == nil || operatorID == nil {
		panic("workforce traeger staff: the releases, their operator audit and the operator are required")
	}
	return &ReleaseResource{releases: releases, audit: audit, operatorID: operatorID}
}

// State answers GET /operator/organizations/{id}/staff-release. The root
// wraps it in the operator read audit.
func (rs *ReleaseResource) State(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid organization ID")
	if !ok {
		return
	}
	var state workforce.TraegerStaffReleaseState
	err := tenant.WithinAdmin(r.Context(), func(adminCtx context.Context) error {
		var err error
		state, err = rs.releases.ReleaseState(adminCtx, organizationID)
		return err
	})
	if err != nil {
		common.RenderError(w, r, releaseErrors(err))
		return
	}
	common.Respond(w, r, http.StatusOK, state, "Staff release retrieved successfully")
}

// grantRequest names the statement the Träger confirmed.
type grantRequest struct {
	Representation string `json:"representation"`
}

func (req *grantRequest) Bind(*http.Request) error {
	if req.Representation != workforce.TraegerStaffRepresentationAgreement && req.Representation != workforce.TraegerStaffRepresentationNone {
		return workforce.ErrTraegerStaffReleaseInvalid
	}
	return nil
}

// Grant answers POST /operator/organizations/{id}/staff-release.
func (rs *ReleaseResource) Grant(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid organization ID")
	if !ok {
		return
	}
	req := &grantRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidOnField(err, common.CodeWorkforceTraegerStaffReleaseInvalid, "representation"))
		return
	}
	var release workforce.TraegerStaffRelease
	rs.write(w, r, ReleaseGranted, req.Representation, func(ctx context.Context, operatorID int64) error {
		var err error
		release, err = rs.releases.GrantRelease(ctx, organizationID, operatorID, req.Representation)
		return err
	}, func() { common.Respond(w, r, http.StatusCreated, release, "Staff release granted successfully") })
}

// Withdraw answers DELETE /operator/organizations/{id}/staff-release.
func (rs *ReleaseResource) Withdraw(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid organization ID")
	if !ok {
		return
	}
	rs.write(w, r, ReleaseWithdrawn, "", func(ctx context.Context, _ int64) error {
		return rs.releases.WithdrawRelease(ctx, organizationID)
	}, func() {
		common.Respond(w, r, http.StatusOK, map[string]int64{"organization_id": organizationID}, "Staff release withdrawn successfully")
	})
}

// write runs the audit entry and the change in one administrative
// transaction. A refused change rolls the audit entry back with it.
func (rs *ReleaseResource) write(w http.ResponseWriter, r *http.Request, kind, representation string, change func(context.Context, int64) error, respond func()) {
	operatorID := rs.operatorID(r.Context())
	audited := false
	err := tenant.WithinAdmin(r.Context(), func(adminCtx context.Context) error {
		if !rs.audit(w, r.WithContext(adminCtx), kind, representation) {
			return errAuditRefused
		}
		audited = true
		return change(adminCtx, operatorID)
	})
	switch {
	case errors.Is(err, errAuditRefused):
		return
	case err != nil:
		common.RenderError(w, r, releaseErrors(err))
	case audited:
		respond()
	}
}

// errAuditRefused ends the transaction after the audit wrote its response.
var errAuditRefused = errors.New("workforce traeger staff: the operator audit refused the change")

var releaseErrors = common.RulesRenderer([]common.ErrorRule{
	{Target: workforce.ErrTraegerStaffReleaseInvalid, Render: onField(common.CodeWorkforceTraegerStaffReleaseInvalid, "representation")},
	{Target: workforce.ErrTraegerStaffReleaseExists, Render: coded(common.ErrorConflictWithCode, common.CodeWorkforceTraegerStaffReleaseExists)},
	{Target: workforce.ErrTraegerStaffReleaseInUse, Render: coded(common.ErrorConflictWithCode, common.CodeWorkforceTraegerStaffReleaseInUse)},
	{Target: workforce.ErrTraegerStaffReleaseNotFound, Render: coded(common.ErrorNotFoundWithCode, common.CodeWorkforceTraegerStaffReleaseNotFound)},
}, common.ErrorInternalServer)
