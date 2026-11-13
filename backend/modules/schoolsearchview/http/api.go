// Package schoolsearchviewhttp serves the school search (ADR 0230, #6954) at
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

func (req *searchRequest) Bind(*http.Request) error { return nil }

func (rs *Resource) searchSchool(w http.ResponseWriter, r *http.Request) {
	req := &searchRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("the body must be a JSON object with a query")))
		return
	}
	if _, err := schoolsearchview.ValidateQueryText(req.Query); err != nil {
		rendered := common.ErrorInvalidRequest(err)
		if resp, ok := rendered.(*common.ErrResponse); ok {
			resp.Details = map[string]any{"min": schoolsearchview.MinQueryRunes, "max": schoolsearchview.MaxQueryRunes}
		}
		common.RenderError(w, r, rendered)
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
	granted := grantedPermissions(sessiontoken.PermissionsFromCtx(ctx))
	readsChildren, err := rs.readsChildren(ctx, granted)
	if err != nil {
		return schoolsearchview.Scope{}, err
	}
	readsDetails := readsChildren && granted.any(permissions.UsersRead)
	return schoolsearchview.Scope{
		Children:     readsChildren,
		ChildDetails: readsDetails,
		Contacts:     readsDetails,
		Staff:        granted.any(permissions.UsersRead, permissions.StaffManage, permissions.StaffStammdaten),
		Rooms:        granted.any(permissions.RoomsRead),
		Groups:       readsChildren && granted.any(permissions.GroupsRead),
	}, nil
}

// readsChildren reports the child read: users:read or students:read, held by
// an administrator or by staff of this school.
func (rs *Resource) readsChildren(ctx context.Context, granted grantedPermissions) (bool, error) {
	if !granted.any(permissions.UsersRead, permissions.StudentsRead) {
		return false, nil
	}
	if securityruntime.HasAdminWildcard(granted) {
		return true, nil
	}
	return rs.staff.HasCurrentStaff(ctx)
}

// grantedPermissions are the caller's permissions; an administrator holds
// every one.
type grantedPermissions []string

func (granted grantedPermissions) any(wanted ...string) bool {
	if securityruntime.HasAdminWildcard(granted) {
		return true
	}
	for _, permission := range wanted {
		if securityruntime.HasPermission(permission, granted) {
			return true
		}
	}
	return false
}
