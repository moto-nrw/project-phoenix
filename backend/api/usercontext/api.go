package usercontext

import (
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
)

// Resource handles the user context-related endpoints
type Resource struct {
	service usercontext.UserContextService
	router  chi.Router
}

// NewResource creates a new user context resource
func NewResource(service usercontext.UserContextService) *Resource {
	r := &Resource{
		service: service,
		router:  chi.NewRouter(),
	}

	// Create JWT auth instance for middleware
	tokenAuth, _ := jwt.NewTokenAuth()

	// Setup routes with proper authentication chain
	r.router.Use(tokenAuth.Verifier())
	r.router.Use(jwt.Authenticator)

	// User profile endpoints
	r.router.Get("/", r.getCurrentUser)
	r.router.Get("/profile", r.getCurrentPerson)
	r.router.Get("/staff", r.getCurrentStaff)
	r.router.Get("/teacher", r.getCurrentTeacher)

	// Group endpoints
	r.router.Route("/groups", func(router chi.Router) {
		router.Use(authorize.RequiresPermission(permissions.UsersRead))
		router.Get("/", r.getMyGroups)
		router.Get("/activity", r.getMyActivityGroups)
		router.Get("/active", r.getMyActiveGroups)
		router.Get("/supervised", r.getMySupervisedGroups)

		// Group details (requires group ID)
		router.Route("/{groupID}", func(router chi.Router) {
			router.Get("/students", r.getGroupStudents)
			router.Get("/visits", r.getGroupVisits)
		})
	})

	return r
}

// Router returns the router for this resource
func (r *Resource) Router() chi.Router {
	return r.router
}

// getCurrentUser returns the current authenticated user
func (res *Resource) getCurrentUser(w http.ResponseWriter, r *http.Request) {
	user, err := res.service.GetCurrentUser(r.Context())
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}
	render.Status(r, http.StatusOK)
	if err := render.Render(w, r, common.NewResponse(user, "Current user retrieved successfully")); err != nil {
		log.Printf("Error rendering response: %v", err)
	}
}

// getCurrentPerson returns the current user's person profile
func (res *Resource) getCurrentPerson(w http.ResponseWriter, r *http.Request) {
	person, err := res.service.GetCurrentPerson(r.Context())
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}
	render.Status(r, http.StatusOK)
	if err := render.Render(w, r, common.NewResponse(person, "Current profile retrieved successfully")); err != nil {
		log.Printf("Error rendering response: %v", err)
	}
}

// getCurrentStaff returns the current user's staff profile
func (res *Resource) getCurrentStaff(w http.ResponseWriter, r *http.Request) {
	staff, err := res.service.GetCurrentStaff(r.Context())
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}
	render.Status(r, http.StatusOK)
	if err := render.Render(w, r, common.NewResponse(staff, "Current staff profile retrieved successfully")); err != nil {
		log.Printf("Error rendering error response: %v", err)
	}
}

// getCurrentTeacher returns the current user's teacher profile
func (res *Resource) getCurrentTeacher(w http.ResponseWriter, r *http.Request) {
	teacher, err := res.service.GetCurrentTeacher(r.Context())
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}
	render.Status(r, http.StatusOK)
	if err := render.Render(w, r, common.NewResponse(teacher, "Current teacher profile retrieved successfully")); err != nil {
		log.Printf("Error rendering error response: %v", err)
	}
}

// getMyGroups returns the educational groups associated with the current user
func (res *Resource) getMyGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := res.service.GetMyGroups(r.Context())
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}
	render.Status(r, http.StatusOK)
	if err := render.Render(w, r, common.NewResponse(groups, "Educational groups retrieved successfully")); err != nil {
		log.Printf("Error rendering error response: %v", err)
	}
}

// getMyActivityGroups returns the activity groups associated with the current user
func (res *Resource) getMyActivityGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := res.service.GetMyActivityGroups(r.Context())
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}
	render.Status(r, http.StatusOK)
	if err := render.Render(w, r, common.NewResponse(groups, "Activity groups retrieved successfully")); err != nil {
		log.Printf("Error rendering error response: %v", err)
	}
}

// getMyActiveGroups returns the active groups associated with the current user
func (res *Resource) getMyActiveGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := res.service.GetMyActiveGroups(r.Context())
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}
	render.Status(r, http.StatusOK)
	if err := render.Render(w, r, common.NewResponse(groups, "Active groups retrieved successfully")); err != nil {
		log.Printf("Error rendering error response: %v", err)
	}
}

// getMySupervisedGroups returns the active groups supervised by the current user
func (res *Resource) getMySupervisedGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := res.service.GetMySupervisedGroups(r.Context())
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}
	render.Status(r, http.StatusOK)
	if err := render.Render(w, r, common.NewResponse(groups, "Supervised groups retrieved successfully")); err != nil {
		log.Printf("Error rendering error response: %v", err)
	}
}

// getGroupStudents returns the students in a specific group where the current user has access
func (res *Resource) getGroupStudents(w http.ResponseWriter, r *http.Request) {
	groupID, err := strconv.ParseInt(chi.URLParam(r, "groupID"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, common.ErrorInvalidRequest(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	students, err := res.service.GetGroupStudents(r.Context(), groupID)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}
	render.Status(r, http.StatusOK)
	if err := render.Render(w, r, common.NewResponse(students, "Group students retrieved successfully")); err != nil {
		log.Printf("Error rendering error response: %v", err)
	}
}

// getGroupVisits returns the active visits for a specific group where the current user has access
func (res *Resource) getGroupVisits(w http.ResponseWriter, r *http.Request) {
	groupID, err := strconv.ParseInt(chi.URLParam(r, "groupID"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, common.ErrorInvalidRequest(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	visits, err := res.service.GetGroupVisits(r.Context(), groupID)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}
	render.Status(r, http.StatusOK)
	if err := render.Render(w, r, common.NewResponse(visits, "Group visits retrieved successfully")); err != nil {
		log.Printf("Error rendering error response: %v", err)
	}
}
