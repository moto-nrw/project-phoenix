// Package me serves the /api/me routes: the authenticated caller's account,
// profile, staff and teacher records, groups and room sessions. Every
// decision comes from the Identity & Access caller context; Rows loads the
// retained records the routes render.
package me

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// Rows loads the retained records the read routes render, in the wire shape
// the routes have always had.
type Rows interface {
	StaffRow(ctx context.Context, staffID int64) (any, error)
	TeacherRow(ctx context.Context, teacherID int64) (any, error)
	EducationalGroupRows(ctx context.Context, groups []identityaccess.CallerGroup) (any, error)
	ActivityGroupRows(ctx context.Context, ids []int64) (any, error)
	SessionRows(ctx context.Context, ids []int64) (any, error)
	StudentRows(ctx context.Context, ids []int64) (any, error)
	NavigationRow(ctx context.Context, navigation identityaccess.CallerNavigation) (any, error)
}

// Resource handles the /api/me routes.
type Resource struct {
	uploads  common.UploadStorage
	caller   identityaccess.CallerContext
	rows     Rows
	settings SchoolSettings
	router   chi.Router
}

// NewResource mounts the /api/me routes behind the tenant security chain.
// Every route runs in the request's tenant transaction.
func NewResource(caller identityaccess.CallerContext, rows Rows, configureSession func(chi.Router), uploads common.UploadStorage, settings SchoolSettings) *Resource {
	if uploads == nil {
		panic("me resource: upload storage is required")
	}
	if settings == nil {
		panic("me resource: school settings are required")
	}
	res := &Resource{caller: caller, rows: rows, settings: settings, router: chi.NewRouter(), uploads: uploads}
	configureSession(res.router)
	withTx := common.TenantTxMiddleware

	res.router.With(withTx).Get("/", res.getCurrentUser)
	res.router.With(withTx).Get("/profile", res.getCurrentProfile)
	res.router.With(withTx).Put("/profile", res.updateCurrentProfile)
	res.router.With(withTx).Post("/profile/avatar", res.uploadAvatar)
	res.router.With(withTx).Delete("/profile/avatar", res.deleteAvatar)
	res.router.With(withTx).Get("/profile/avatar/{filename}", res.serveAvatar)
	res.router.With(withTx).Get("/staff", res.getCurrentStaff)
	res.router.With(withTx).Get("/navigation", res.getNavigation)
	res.router.With(withTx).Get("/teacher", res.getCurrentTeacher)

	// Callers always reach their own groups; no permission beyond the chain.
	res.router.Route("/groups", func(router chi.Router) {
		router.With(withTx).Get("/", res.getMyGroups)
		router.With(withTx).Get("/activity", res.getMyActivityGroups)
		router.With(withTx).Get("/active", res.getMyActiveGroups)
		router.With(withTx).Get("/supervised", res.getMySupervisedGroups)
		router.Route("/{groupID}", func(router chi.Router) {
			router.With(withTx).Get("/students", res.getGroupStudents)
			router.With(withTx).Get("/visits", res.getGroupVisits)
		})
	})
	return res
}

// Router returns the router of the /api/me routes.
func (res *Resource) Router() chi.Router {
	return res.router
}

func (res *Resource) respond(w http.ResponseWriter, r *http.Request, data any, err error, message string) {
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}
	render.Status(r, http.StatusOK)
	common.RenderError(w, r, common.NewResponse(data, message))
}

func (res *Resource) getCurrentUser(w http.ResponseWriter, r *http.Request) {
	account, err := res.caller.Account(r.Context())
	res.respond(w, r, currentUserResponse(account), err, "Current user retrieved successfully")
}

func (res *Resource) getCurrentStaff(w http.ResponseWriter, r *http.Request) {
	staff, err := res.staffRow(r.Context())
	res.respond(w, r, staff, err, "Current staff profile retrieved successfully")
}

func (res *Resource) staffRow(ctx context.Context) (any, error) {
	staffID, err := res.caller.StaffID(ctx)
	if err != nil {
		return nil, err
	}
	return res.rows.StaffRow(ctx, staffID)
}

func (res *Resource) getCurrentTeacher(w http.ResponseWriter, r *http.Request) {
	teacher, err := res.teacherRow(r.Context())
	res.respond(w, r, teacher, err, "Current teacher profile retrieved successfully")
}

func (res *Resource) teacherRow(ctx context.Context) (any, error) {
	teacherID, err := res.caller.TeacherID(ctx)
	if err != nil {
		return nil, err
	}
	return res.rows.TeacherRow(ctx, teacherID)
}

func (res *Resource) getNavigation(w http.ResponseWriter, r *http.Request) {
	navigation, err := res.navigationRow(r.Context())
	if errors.Is(err, errSchoolSwitchesUnavailable) {
		// Like the tenant resolve: the settings failure stays in the log.
		common.RenderError(w, r, common.ErrorInternalServerWrap(http.StatusText(http.StatusInternalServerError), err))
		return
	}
	res.respond(w, r, navigation, err, "Navigation context retrieved successfully")
}

func (res *Resource) navigationRow(ctx context.Context) (any, error) {
	navigation, err := res.caller.Navigation(ctx)
	if err != nil {
		return nil, err
	}
	row, err := res.rows.NavigationRow(ctx, navigation)
	if err != nil {
		return nil, err
	}
	switches, err := loadSchoolSwitches(ctx, res.settings)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errSchoolSwitchesUnavailable, err)
	}
	return withSchoolSwitches(row, switches)
}

// getMyGroups renders the caller's educational groups. The substitution
// lookup is best effort: when it fails, every group renders with
// via_substitution false instead of hiding the list.
func (res *Resource) getMyGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := res.myGroupRows(r.Context())
	res.respond(w, r, groups, err, "Educational groups retrieved successfully")
}

func (res *Resource) myGroupRows(ctx context.Context) (any, error) {
	ids, err := res.caller.MyGroupIDs(ctx)
	if err != nil {
		return nil, err
	}
	substituted, _ := res.caller.SubstitutedGroupIDs(ctx)
	groups := make([]identityaccess.CallerGroup, 0, len(ids))
	for _, id := range ids {
		groups = append(groups, identityaccess.CallerGroup{ID: id, ViaSubstitution: substituted[id]})
	}
	return res.rows.EducationalGroupRows(ctx, groups)
}

func (res *Resource) getMyActivityGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := res.idRows(r.Context(), res.caller.MyActivityGroupIDs, res.rows.ActivityGroupRows)
	res.respond(w, r, groups, err, "Activity groups retrieved successfully")
}

func (res *Resource) getMyActiveGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := res.idRows(r.Context(), res.caller.MyActiveSessionIDs, res.rows.SessionRows)
	res.respond(w, r, groups, err, "Active groups retrieved successfully")
}

func (res *Resource) getMySupervisedGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := res.idRows(r.Context(), res.caller.MySupervisedSessionIDs, res.rows.SessionRows)
	res.respond(w, r, groups, err, "Supervised groups retrieved successfully")
}

func (res *Resource) idRows(
	ctx context.Context,
	ids func(context.Context) ([]int64, error),
	rows func(context.Context, []int64) (any, error),
) (any, error) {
	resolved, err := ids(ctx)
	if err != nil {
		return nil, err
	}
	return rows(ctx, resolved)
}

func (res *Resource) getGroupStudents(w http.ResponseWriter, r *http.Request) {
	groupID, err := common.ParseIDParam(r, "groupID")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	students, err := res.idRows(r.Context(), func(ctx context.Context) ([]int64, error) {
		return res.caller.GroupStudentIDs(ctx, groupID)
	}, res.rows.StudentRows)
	res.respond(w, r, students, err, "Group students retrieved successfully")
}

func (res *Resource) getGroupVisits(w http.ResponseWriter, r *http.Request) {
	groupID, err := common.ParseIDParam(r, "groupID")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	visits, err := res.caller.GroupVisits(r.Context(), groupID)
	res.respond(w, r, groupVisitResponses(visits), err, "Group visits retrieved successfully")
}

// ErrorRenderer maps caller-context errors to HTTP responses. Matched via
// errors.Is against the full error and rendered with the wrapper text.
var ErrorRenderer = common.RulesRenderer([]common.ErrorRule{
	{Target: identityaccess.ErrCallerNotAuthenticated, Render: common.ErrorUnauthorized},
	{Target: identityaccess.ErrCallerNotAuthorized, Render: common.ErrorForbidden},
	{Target: identityaccess.ErrCallerNotFound, Render: common.ErrorNotFound},
	{Target: identityaccess.ErrCallerNotLinkedToPerson, Render: common.ErrorNotFound},
	{Target: identityaccess.ErrCallerNotLinkedToStaff, Render: common.ErrorNotFound},
	{Target: identityaccess.ErrCallerNotLinkedToTeacher, Render: common.ErrorNotFound},
	{Target: identityaccess.ErrCallerGroupNotFound, Render: common.ErrorNotFound},
	{Target: identityaccess.ErrCallerSessionNotFound, Render: common.ErrorNotFound},
}, common.ErrorInternalServer)

var errAuthenticationRequired = errors.New("authentication required")
