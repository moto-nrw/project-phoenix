// Package schoolsearchviewhttp serves the school search (ADR 0227, #6954) at
// POST /api/search inside the protected tenant routes.
//
// The query travels in the body, not in the URL: a phone number or a name
// must not land in access logs, proxies or browser histories. The handler
// never logs the query, and an error names the failed read, not the text.
package schoolsearchviewhttp

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/auth/authorize/permissions"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-ogs/project-phoenix/modules/schoolsearchview"
	"github.com/moto-ogs/project-phoenix/modules/securityruntime"
)

// StaffCheck reports whether the caller has a staff record at the school.
type StaffCheck interface {
	HasCurrentStaff(ctx context.Context) (bool, error)
}

// Resource serves the search route.
type Resource struct {
	search schoolsearchview.Search
	staff  StaffCheck
}

// NewResource builds the resource; both dependencies are required.
func NewResource(search schoolsearchview.Search, staff StaffCheck) *Resource {
	if search == nil || staff == nil {
		panic("school search view http: search and staff check are required")
	}
	return &Resource{search: search, staff: staff}
}

// Router mounts POST / inside the protected tenant routes. The route needs no
// single permission: the Scope decides per kind what the caller may find.
func (rs *Resource) Router(protected func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protected(r, func(r chi.Router, withTx common.Middleware) {
		r.With(withTx).Post("/", rs.searchSchool)
	})
	return r
}

type searchRequest struct {
	Query string `json:"query"`
}

func (req *searchRequest) Bind(*http.Request) error {
	_, err := schoolsearchview.ValidateQueryText(req.Query)
	return err
}

func (rs *Resource) searchSchool(w http.ResponseWriter, r *http.Request) {
	req := &searchRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("query must have between 2 and 100 characters")))
		return
	}
	scope, err := rs.scope(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("failed to resolve search scope", err))
		return
	}
	result, err := rs.search.SearchSchool(r.Context(), schoolsearchview.Query{Text: req.Query, Scope: scope})
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("failed to search", err))
		return
	}
	common.Respond(w, r, http.StatusOK, result, "Search completed")
}

// scope maps the caller's permissions onto what the search may read. The
// person kinds follow the child read scope (#2329): an administrator, or
// staff of this school. Each kind needs the permission its own page needs.
func (rs *Resource) scope(ctx context.Context) (schoolsearchview.Scope, error) {
	granted := sessiontoken.PermissionsFromCtx(ctx)
	has := func(permissions ...string) bool {
		if securityruntime.HasAdminWildcard(granted) {
			return true
		}
		for _, permission := range permissions {
			if securityruntime.HasPermission(permission, granted) {
				return true
			}
		}
		return false
	}
	readsChildren := has(permissions.UsersRead, permissions.StudentsRead)
	readsDetails := has(permissions.UsersRead)
	if readsChildren {
		isStaff, err := rs.staff.HasCurrentStaff(ctx)
		if err != nil {
			return schoolsearchview.Scope{}, err
		}
		if !securityruntime.HasAdminWildcard(granted) && !isStaff {
			readsChildren, readsDetails = false, false
		}
	}
	return schoolsearchview.Scope{
		Children:     readsChildren,
		ChildDetails: readsDetails && readsChildren,
		Contacts:     readsDetails && readsChildren,
		Staff:        has(permissions.UsersRead, permissions.StaffManage, permissions.StaffStammdaten),
		Rooms:        has(permissions.RoomsRead),
		Groups:       has(permissions.GroupsRead) && readsChildren,
	}, nil
}
