package user

import (
	"context"
	"errors"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	jwt2 "github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/logging"
	"github.com/moto-nrw/project-phoenix/models"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/sirupsen/logrus"
)

// Resource defines the user management resource
type Resource struct {
	Store     UserStore
	AuthStore AuthTokenStore
}

// UserStore defines database operations for user management
type UserStore interface {
	GetCustomUserByID(ctx context.Context, id int64) (*models.CustomUser, error)
	GetCustomUserByTagID(ctx context.Context, tagID string) (*models.CustomUser, error)
	CreateCustomUser(ctx context.Context, user *models.CustomUser) error
	UpdateCustomUser(ctx context.Context, user *models.CustomUser) error
	DeleteCustomUser(ctx context.Context, id int64) error
	UpdateTagID(ctx context.Context, userID int64, tagID string) error
	ListCustomUsers(ctx context.Context) ([]models.CustomUser, error)

	GetSpecialistByID(ctx context.Context, id int64) (*models.PedagogicalSpecialist, error)
	GetSpecialistByUserID(ctx context.Context, userID int64) (*models.PedagogicalSpecialist, error)
	CreateSpecialist(ctx context.Context, specialist *models.PedagogicalSpecialist) error
	UpdateSpecialist(ctx context.Context, specialist *models.PedagogicalSpecialist) error
	DeleteSpecialist(ctx context.Context, id int64) error
	ListSpecialists(ctx context.Context) ([]models.PedagogicalSpecialist, error)
	ListSpecialistsWithoutSupervision(ctx context.Context) ([]models.PedagogicalSpecialist, error)

	CreateDevice(ctx context.Context, device *models.Device) error
	GetDeviceByID(ctx context.Context, id int64) (*models.Device, error)
	GetDeviceByDeviceID(ctx context.Context, deviceID string) (*models.Device, error)
	DeleteDevice(ctx context.Context, id int64) error
	ListDevicesByUserID(ctx context.Context, userID int64) ([]models.Device, error)
}

// AuthTokenStore defines operations for the auth token store
type AuthTokenStore interface {
	GetToken(t string) (*jwt2.Token, error)
}

// NewResource creates a new user management resource
func NewResource(store UserStore, authStore AuthTokenStore) *Resource {
	return &Resource{
		Store:     store,
		AuthStore: authStore,
	}
}

// Router creates a router for user management
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()

	// Public routes
	r.Route("/public", func(r chi.Router) {
		r.Get("/users", rs.listUsersPublic)
		r.Get("/supervisors", rs.listSupervisorsPublic)
	})

	// JWT protected routes
	r.Group(func(r chi.Router) {
		r.Use(jwt2.Authenticator)
		r.Use(authorize.RequiresRole("admin"))

		// User routes
		r.Route("/users", func(r chi.Router) {
			r.Get("/", rs.listUsers)
			r.Post("/", rs.createUser)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", rs.getUser)
				r.Put("/", rs.updateUser)
				r.Delete("/", rs.deleteUser)
			})
		})

		// Specialist routes
		r.Route("/specialists", func(r chi.Router) {
			r.Get("/", rs.listSpecialists)
			r.Get("/without-supervision", rs.listSpecialistsWithoutSupervision)
			r.Post("/", rs.createSpecialist)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", rs.getSpecialist)
				r.Put("/", rs.updateSpecialist)
				r.Delete("/", rs.deleteSpecialist)
			})
		})

		// Device routes
		r.Route("/devices", func(r chi.Router) {
			r.Post("/", rs.createDevice)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", rs.getDevice)
				r.Delete("/", rs.deleteDevice)
			})
		})

		// Special operations
		r.Post("/change-tag-id", rs.changeTagID)
		r.Post("/process-tag-scan", rs.processTagScan)
	})

	return r
}

// UserRequest is the request payload for User data
type UserRequest struct {
	*models.CustomUser
}

// Bind preprocesses a UserRequest
func (req *UserRequest) Bind(r *http.Request) error {
	return nil
}

// SpecialistRequest is the request payload for PedagogicalSpecialist data
type SpecialistRequest struct {
	*models.PedagogicalSpecialist
}

// Bind preprocesses a SpecialistRequest
func (req *SpecialistRequest) Bind(r *http.Request) error {
	return nil
}

// DeviceRequest is the request payload for Device data
type DeviceRequest struct {
	*models.Device
}

// Bind preprocesses a DeviceRequest
func (req *DeviceRequest) Bind(r *http.Request) error {
	return nil
}

// ChangeTagIDRequest is the request payload for changing a tag ID
type ChangeTagIDRequest struct {
	UserID int64  `json:"user_id"`
	TagID  string `json:"tag_id"`
}

// Bind preprocesses a ChangeTagIDRequest
func (req *ChangeTagIDRequest) Bind(r *http.Request) error {
	return nil
}

// TagScanRequest is the request payload for processing a tag scan
type TagScanRequest struct {
	TagID    string `json:"tag_id"`
	DeviceID string `json:"device_id"`
}

// Bind preprocesses a TagScanRequest
func (req *TagScanRequest) Bind(r *http.Request) error {
	return nil
}

// ======== User Handlers ========

// listUsers returns a list of all users
func (rs *Resource) listUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	users, err := rs.Store.ListCustomUsers(ctx)
	if err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.JSON(w, r, users)
}

// createUser creates a new user
func (rs *Resource) createUser(w http.ResponseWriter, r *http.Request) {
	data := &UserRequest{}
	if err := render.Bind(r, data); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	ctx := r.Context()
	if err := rs.Store.CreateCustomUser(ctx, data.CustomUser); err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, data.CustomUser)
}

// getUser returns a specific user
func (rs *Resource) getUser(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	ctx := r.Context()
	user, err := rs.Store.GetCustomUserByID(ctx, id)
	if err != nil {
		render.Render(w, r, ErrNotFound)
		return
	}

	render.JSON(w, r, user)
}

// updateUser updates a specific user
func (rs *Resource) updateUser(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	data := &UserRequest{}
	if err := render.Bind(r, data); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	ctx := r.Context()
	user, err := rs.Store.GetCustomUserByID(ctx, id)
	if err != nil {
		render.Render(w, r, ErrNotFound)
		return
	}

	// Update user fields
	user.FirstName = data.FirstName
	user.SecondName = data.SecondName
	if data.TagID != nil {
		user.TagID = data.TagID
	}

	if err := rs.Store.UpdateCustomUser(ctx, user); err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.JSON(w, r, user)
}

// deleteUser deletes a specific user
func (rs *Resource) deleteUser(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	ctx := r.Context()
	if err := rs.Store.DeleteCustomUser(ctx, id); err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.NoContent(w, r)
}

// ======== Specialist Handlers ========

// listSpecialists returns a list of all specialists
func (rs *Resource) listSpecialists(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	specialists, err := rs.Store.ListSpecialists(ctx)
	if err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.JSON(w, r, specialists)
}

// listSpecialistsWithoutSupervision returns specialists without supervision duties
func (rs *Resource) listSpecialistsWithoutSupervision(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	specialists, err := rs.Store.ListSpecialistsWithoutSupervision(ctx)
	if err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.JSON(w, r, specialists)
}

// createSpecialist creates a new specialist
func (rs *Resource) createSpecialist(w http.ResponseWriter, r *http.Request) {
	data := &SpecialistRequest{}
	if err := render.Bind(r, data); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	ctx := r.Context()
	if err := rs.Store.CreateSpecialist(ctx, data.PedagogicalSpecialist); err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	// Get the created specialist with its related user
	specialist, err := rs.Store.GetSpecialistByID(ctx, data.PedagogicalSpecialist.ID)
	if err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, specialist)
}

// getSpecialist returns a specific specialist
func (rs *Resource) getSpecialist(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	ctx := r.Context()
	specialist, err := rs.Store.GetSpecialistByID(ctx, id)
	if err != nil {
		render.Render(w, r, ErrNotFound)
		return
	}

	render.JSON(w, r, specialist)
}

// updateSpecialist updates a specific specialist
func (rs *Resource) updateSpecialist(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	data := &SpecialistRequest{}
	if err := render.Bind(r, data); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	ctx := r.Context()
	specialist, err := rs.Store.GetSpecialistByID(ctx, id)
	if err != nil {
		render.Render(w, r, ErrNotFound)
		return
	}

	// Update specialist fields
	specialist.Role = data.Role
	specialist.IsPasswordOTP = data.IsPasswordOTP

	if err := rs.Store.UpdateSpecialist(ctx, specialist); err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.JSON(w, r, specialist)
}

// deleteSpecialist deletes a specific specialist
func (rs *Resource) deleteSpecialist(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	ctx := r.Context()
	if err := rs.Store.DeleteSpecialist(ctx, id); err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.NoContent(w, r)
}

// ======== Device Handlers ========

// createDevice creates a new device
func (rs *Resource) createDevice(w http.ResponseWriter, r *http.Request) {
	data := &DeviceRequest{}
	if err := render.Bind(r, data); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	ctx := r.Context()
	if err := rs.Store.CreateDevice(ctx, data.Device); err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, data.Device)
}

// getDevice returns a specific device
func (rs *Resource) getDevice(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	ctx := r.Context()
	device, err := rs.Store.GetDeviceByID(ctx, id)
	if err != nil {
		render.Render(w, r, ErrNotFound)
		return
	}

	render.JSON(w, r, device)
}

// deleteDevice deletes a specific device
func (rs *Resource) deleteDevice(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	ctx := r.Context()
	if err := rs.Store.DeleteDevice(ctx, id); err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.NoContent(w, r)
}

// ======== Special Operations ========

// changeTagID changes the RFID tag ID for a user
func (rs *Resource) changeTagID(w http.ResponseWriter, r *http.Request) {
	data := &ChangeTagIDRequest{}
	if err := render.Bind(r, data); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	ctx := r.Context()
	if err := rs.Store.UpdateTagID(ctx, data.UserID, data.TagID); err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.JSON(w, r, map[string]bool{"success": true})
}

// processTagScan processes an RFID tag scan event
func (rs *Resource) processTagScan(w http.ResponseWriter, r *http.Request) {
	data := &TagScanRequest{}
	if err := render.Bind(r, data); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	ctx := r.Context()

	// Get user by tag ID
	user, err := rs.Store.GetCustomUserByTagID(ctx, data.TagID)
	if err != nil {
		// If no user found with this tag, just log the tag read
		log := logging.GetLogEntry(r)
		log.WithFields(logrus.Fields{
			"tag_id":    data.TagID,
			"device_id": data.DeviceID,
		}).Info("Tag scanned but no user associated")

		render.JSON(w, r, map[string]interface{}{
			"success": false,
			"message": "No user found with this tag ID",
		})
		return
	}

	// Here you would implement the business logic for what happens when a user is scanned
	// This could be check-in, check-out, room access, etc.

	render.JSON(w, r, map[string]interface{}{
		"success": true,
		"user_id": user.ID,
		"message": "Tag scan processed successfully",
	})
}

// ======== Public Endpoints ========

// listUsersPublic returns a limited list of users for public access
func (rs *Resource) listUsersPublic(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	users, err := rs.Store.ListCustomUsers(ctx)
	if err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	// Create a limited view of the users for public access
	type PublicUser struct {
		ID         int64   `json:"id"`
		FirstName  string  `json:"first_name"`
		SecondName string  `json:"second_name"`
		TagID      *string `json:"tag_id,omitempty"`
	}

	publicUsers := make([]PublicUser, len(users))
	for i, user := range users {
		publicUsers[i] = PublicUser{
			ID:         user.ID,
			FirstName:  user.FirstName,
			SecondName: user.SecondName,
			TagID:      user.TagID,
		}
	}

	render.JSON(w, r, publicUsers)
}

// listSupervisorsPublic returns a limited list of supervisors for public access
func (rs *Resource) listSupervisorsPublic(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	specialists, err := rs.Store.ListSpecialists(ctx)
	if err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	// Create a limited view of the specialists for public access
	type PublicSupervisor struct {
		ID         int64  `json:"id"`
		Role       string `json:"role"`
		UserID     int64  `json:"user_id"`
		FirstName  string `json:"first_name"`
		SecondName string `json:"second_name"`
	}

	publicSupervisors := make([]PublicSupervisor, 0, len(specialists))
	for _, specialist := range specialists {
		if specialist.CustomUser != nil {
			publicSupervisors = append(publicSupervisors, PublicSupervisor{
				ID:         specialist.ID,
				Role:       specialist.Role,
				UserID:     specialist.UserID,
				FirstName:  specialist.CustomUser.FirstName,
				SecondName: specialist.CustomUser.SecondName,
			})
		}
	}

	render.JSON(w, r, publicSupervisors)
}
