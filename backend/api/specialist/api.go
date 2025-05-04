// Package specialist provides the pedagogical specialist management API
package specialist

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/logging"
	models2 "github.com/moto-nrw/project-phoenix/models"
	"github.com/sirupsen/logrus"
)

// Resource defines the pedagogical specialist management resource
type Resource struct {
	Store     SpecialistStore
	AuthStore AuthTokenStore
	UserStore UserStore
}

// SpecialistStore defines database operations for pedagogical specialist management
type SpecialistStore interface {
	// Specialist operations
	CreateSpecialist(ctx context.Context, specialist *models2.PedagogicalSpecialist, tagID *string, accountID *int64) error
	GetSpecialistByID(ctx context.Context, id int64) (*models2.PedagogicalSpecialist, error)
	UpdateSpecialist(ctx context.Context, specialist *models2.PedagogicalSpecialist) error
	DeleteSpecialist(ctx context.Context, id int64) error
	ListSpecialists(ctx context.Context, filters map[string]interface{}) ([]models2.PedagogicalSpecialist, error)
	ListSpecialistsWithoutSupervision(ctx context.Context) ([]models2.PedagogicalSpecialist, error)

	// Group supervision operations
	AssignToGroup(ctx context.Context, specialistID, groupID int64) error
	RemoveFromGroup(ctx context.Context, specialistID, groupID int64) error
	ListAssignedGroups(ctx context.Context, specialistID int64) ([]models2.Group, error)
}

// AuthTokenStore defines operations for the auth token store
type AuthTokenStore interface {
	GetToken(t string) (*jwt.Token, error)
}

// UserStore defines operations for user management
type UserStore interface {
	GetCustomUserByID(ctx context.Context, id int64) (*models2.CustomUser, error)
	CreateCustomUser(ctx context.Context, user *models2.CustomUser) error
	UpdateCustomUser(ctx context.Context, user *models2.CustomUser) error
	UpdateTagID(ctx context.Context, userID int64, tagID string) error
}

// NewResource creates a new specialist management resource
func NewResource(store SpecialistStore, authStore AuthTokenStore, userStore UserStore) *Resource {
	return &Resource{
		Store:     store,
		AuthStore: authStore,
		UserStore: userStore,
	}
}

// Router creates a router for specialist management
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()

	// JWT protected routes
	r.Group(func(r chi.Router) {
		r.Use(jwt.Authenticator)

		// Specialist routes
		r.Route("/", func(r chi.Router) {
			r.Get("/", rs.listSpecialists)
			r.Post("/", rs.createSpecialist)
			r.Get("/available", rs.listAvailableSpecialists)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", rs.getSpecialist)
				r.Put("/", rs.updateSpecialist)
				r.Delete("/", rs.deleteSpecialist)
				r.Get("/groups", rs.getSpecialistGroups)
				r.Post("/groups/{groupId}", rs.assignToGroup)
				r.Delete("/groups/{groupId}", rs.removeFromGroup)
			})
		})
	})

	return r
}

// ======== Request/Response Models ========

// SpecialistRequest is the request payload for Specialist data
type SpecialistRequest struct {
	*models2.PedagogicalSpecialist
	FirstName  string `json:"first_name,omitempty"`
	SecondName string `json:"second_name,omitempty"`
	TagID      string `json:"tag_id,omitempty"`
	AccountID  int64  `json:"account_id,omitempty"`
}

// Bind preprocesses a SpecialistRequest
func (req *SpecialistRequest) Bind(r *http.Request) error {
	if req.PedagogicalSpecialist == nil {
		return errors.New("missing specialist data")
	}
	return nil
}

// ======== Specialist Handlers ========

// listSpecialists returns a list of all specialists with optional filtering
func (rs *Resource) listSpecialists(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)
	ctx := r.Context()

	// Parse query parameters for filtering
	filters := make(map[string]interface{})

	if roleFilter := r.URL.Query().Get("role"); roleFilter != "" {
		filters["role"] = roleFilter
	}

	if searchTerm := r.URL.Query().Get("search"); searchTerm != "" {
		filters["search"] = searchTerm
	}

	specialists, err := rs.Store.ListSpecialists(ctx, filters)
	if err != nil {
		logger.WithError(err).Error("Failed to list specialists")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithField("count", len(specialists)).Info("Listed specialists")
	render.JSON(w, r, specialists)
}

// createSpecialist creates a new pedagogical specialist
func (rs *Resource) createSpecialist(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	data := &SpecialistRequest{}
	if err := render.Bind(r, data); err != nil {
		logger.WithError(err).Warn("Invalid specialist creation request")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Validate the specialist data
	if err := ValidateSpecialist(data.PedagogicalSpecialist); err != nil {
		logger.WithError(err).Warn("Specialist validation failed")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Check if we need to create a new CustomUser or use an existing one
	var tagID *string
	if data.TagID != "" {
		tagID = &data.TagID
	}

	var accountID *int64
	if data.AccountID != 0 {
		accountID = &data.AccountID
	}

	ctx := r.Context()
	if err := rs.Store.CreateSpecialist(ctx, data.PedagogicalSpecialist, tagID, accountID); err != nil {
		logger.WithError(err).Error("Failed to create specialist")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithFields(logrus.Fields{
		"specialist_id": data.PedagogicalSpecialist.ID,
		"role":          data.PedagogicalSpecialist.Role,
	}).Info("Specialist created successfully")

	// Get the created specialist with all relations
	specialist, err := rs.Store.GetSpecialistByID(ctx, data.ID)
	if err != nil {
		logger.WithError(err).Error("Failed to retrieve newly created specialist")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, specialist)
}

// getSpecialist returns a specific specialist
func (rs *Resource) getSpecialist(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid specialist ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	ctx := r.Context()
	specialist, err := rs.Store.GetSpecialistByID(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to get specialist by ID")
		render.Render(w, r, ErrNotFound())
		return
	}

	render.JSON(w, r, specialist)
}

// updateSpecialist updates a specific specialist
func (rs *Resource) updateSpecialist(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid specialist ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	data := &SpecialistRequest{}
	if err := render.Bind(r, data); err != nil {
		logger.WithError(err).Warn("Invalid specialist update request")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Validate the specialist data
	if err := ValidateSpecialist(data.PedagogicalSpecialist); err != nil {
		logger.WithError(err).Warn("Specialist validation failed")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Ensure the ID in the URL matches the ID in the request body
	data.ID = id

	ctx := r.Context()
	if err := rs.Store.UpdateSpecialist(ctx, data.PedagogicalSpecialist); err != nil {
		logger.WithError(err).Error("Failed to update specialist")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	// Update user information if provided
	if data.CustomUser != nil && data.CustomUser.ID > 0 {
		// If first name or second name are provided in the request, update them
		if data.FirstName != "" || data.SecondName != "" {
			user, err := rs.UserStore.GetCustomUserByID(ctx, data.CustomUser.ID)
			if err != nil {
				logger.WithError(err).Error("Failed to get custom user for update")
				render.Render(w, r, ErrInternalServerError(err))
				return
			}

			if data.FirstName != "" {
				user.FirstName = data.FirstName
			}
			if data.SecondName != "" {
				user.SecondName = data.SecondName
			}

			if err := rs.UserStore.UpdateCustomUser(ctx, user); err != nil {
				logger.WithError(err).Error("Failed to update custom user")
				render.Render(w, r, ErrInternalServerError(err))
				return
			}
		}

		// Update tag ID if provided
		if data.TagID != "" {
			if err := rs.UserStore.UpdateTagID(ctx, data.CustomUser.ID, data.TagID); err != nil {
				logger.WithError(err).Error("Failed to update tag ID")
				render.Render(w, r, ErrInternalServerError(err))
				return
			}
		}
	}

	// Get the updated specialist with all relations
	specialist, err := rs.Store.GetSpecialistByID(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to retrieve updated specialist")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithField("specialist_id", id).Info("Specialist updated successfully")
	render.JSON(w, r, specialist)
}

// deleteSpecialist deletes a specific specialist
func (rs *Resource) deleteSpecialist(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid specialist ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	ctx := r.Context()
	if err := rs.Store.DeleteSpecialist(ctx, id); err != nil {
		logger.WithError(err).Error("Failed to delete specialist")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithField("specialist_id", id).Info("Specialist deleted successfully")
	render.NoContent(w, r)
}

// listAvailableSpecialists returns specialists not assigned to any group
func (rs *Resource) listAvailableSpecialists(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)
	ctx := r.Context()

	specialists, err := rs.Store.ListSpecialistsWithoutSupervision(ctx)
	if err != nil {
		logger.WithError(err).Error("Failed to list available specialists")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithField("count", len(specialists)).Info("Listed available specialists")
	render.JSON(w, r, specialists)
}

// getSpecialistGroups returns all groups a specialist is assigned to
func (rs *Resource) getSpecialistGroups(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid specialist ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	ctx := r.Context()
	groups, err := rs.Store.ListAssignedGroups(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to list specialist's groups")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithFields(logrus.Fields{
		"specialist_id": id,
		"count":         len(groups),
	}).Info("Listed groups for specialist")

	render.JSON(w, r, groups)
}

// assignToGroup assigns a specialist to a group
func (rs *Resource) assignToGroup(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	specialistIDStr := chi.URLParam(r, "id")
	specialistID, err := strconv.ParseInt(specialistIDStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid specialist ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid specialist ID format")))
		return
	}

	groupIDStr := chi.URLParam(r, "groupId")
	groupID, err := strconv.ParseInt(groupIDStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid group ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid group ID format")))
		return
	}

	ctx := r.Context()
	if err := rs.Store.AssignToGroup(ctx, specialistID, groupID); err != nil {
		logger.WithError(err).Error("Failed to assign specialist to group")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithFields(logrus.Fields{
		"specialist_id": specialistID,
		"group_id":      groupID,
	}).Info("Specialist assigned to group successfully")

	// Return success response
	render.JSON(w, r, map[string]interface{}{
		"success":       true,
		"specialist_id": specialistID,
		"group_id":      groupID,
		"assigned_at":   time.Now(),
	})
}

// removeFromGroup removes a specialist from a group
func (rs *Resource) removeFromGroup(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	specialistIDStr := chi.URLParam(r, "id")
	specialistID, err := strconv.ParseInt(specialistIDStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid specialist ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid specialist ID format")))
		return
	}

	groupIDStr := chi.URLParam(r, "groupId")
	groupID, err := strconv.ParseInt(groupIDStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid group ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid group ID format")))
		return
	}

	ctx := r.Context()
	if err := rs.Store.RemoveFromGroup(ctx, specialistID, groupID); err != nil {
		logger.WithError(err).Error("Failed to remove specialist from group")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithFields(logrus.Fields{
		"specialist_id": specialistID,
		"group_id":      groupID,
	}).Info("Specialist removed from group successfully")

	render.NoContent(w, r)
}
