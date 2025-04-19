package settings

import (
	"encoding/json"
	"errors"
	"github.com/moto-nrw/project-phoenix/database"
	"github.com/moto-nrw/project-phoenix/logging"
	"github.com/moto-nrw/project-phoenix/models"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/sirupsen/logrus"
)

// Resource handles settings-related API endpoints
type Resource struct {
	Store  database.SettingsStore
	Logger *logrus.Logger
}

// NewResource creates a new settings resource
func NewResource(store database.SettingsStore, authStore interface{}) *Resource {
	return &Resource{
		Store:  store,
		Logger: logging.NewLogger(),
	}
}

// Router provides routes for the settings API
func (rs *Resource) Router() *chi.Mux {
	r := chi.NewRouter()

	r.Group(func(r chi.Router) {
		r.Get("/", rs.List)
		r.Get("/category/{category}", rs.GetByCategory)
		r.Get("/{id}", rs.Get)
		r.Get("/key/{key}", rs.GetByKey)

		// Admin-only routes
		r.Group(func(r chi.Router) {
			r.Post("/", rs.Create)
			r.Put("/{id}", rs.Update)
			r.Patch("/{key}", rs.UpdateByKey)
			r.Delete("/{id}", rs.Delete)
		})
	})

	return r
}

// Create creates a new setting
func (rs *Resource) Create(w http.ResponseWriter, r *http.Request) {
	data := &models.SettingRequest{}

	// Parse request body
	if err := render.Bind(r, data); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Check if setting with key already exists
	_, err := rs.Store.GetByKey(r.Context(), data.Key)
	if err == nil {
		render.Render(w, r, ErrConflict(ErrSettingAlreadyExists))
		return
	}

	// Create setting
	setting := &models.Setting{
		Key:             data.Key,
		Value:           data.Value,
		Category:        data.Category,
		Description:     data.Description,
		RequiresRestart: data.RequiresRestart,
		RequiresDBReset: data.RequiresDBReset,
	}

	if err := rs.Store.Create(r.Context(), setting); err != nil {
		rs.Logger.WithField("error", err).Error("Failed to create setting")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	// Return created setting
	response := &models.SettingResponse{
		ID:              setting.ID,
		Key:             setting.Key,
		Value:           setting.Value,
		Category:        setting.Category,
		Description:     setting.Description,
		RequiresRestart: setting.RequiresRestart,
		RequiresDBReset: setting.RequiresDBReset,
		CreatedAt:       setting.CreatedAt,
		ModifiedAt:      setting.ModifiedAt,
	}
	render.Status(r, http.StatusCreated)
	render.JSON(w, r, response)
}

// Update updates an existing setting
func (rs *Resource) Update(w http.ResponseWriter, r *http.Request) {
	// Get setting ID from URL
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(ErrInvalidSettingID))
		return
	}

	// Get existing setting
	existingSetting, err := rs.Store.Get(r.Context(), id)
	if err != nil {
		render.Render(w, r, ErrNotFound())
		return
	}

	// Parse request body
	data := &models.SettingRequest{}
	if err := render.Bind(r, data); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// If key is changing, check if new key already exists
	if data.Key != existingSetting.Key {
		_, err := rs.Store.GetByKey(r.Context(), data.Key)
		if err == nil {
			render.Render(w, r, ErrConflict(ErrSettingAlreadyExists))
			return
		}
	}

	// Update setting
	existingSetting.Key = data.Key
	existingSetting.Value = data.Value
	existingSetting.Category = data.Category
	existingSetting.Description = data.Description
	existingSetting.RequiresRestart = data.RequiresRestart
	existingSetting.RequiresDBReset = data.RequiresDBReset

	if err := rs.Store.Update(r.Context(), id, existingSetting); err != nil {
		rs.Logger.WithField("error", err).Error("Failed to update setting")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	// Return updated setting
	response := &models.SettingResponse{
		ID:              existingSetting.ID,
		Key:             existingSetting.Key,
		Value:           existingSetting.Value,
		Category:        existingSetting.Category,
		Description:     existingSetting.Description,
		RequiresRestart: existingSetting.RequiresRestart,
		RequiresDBReset: existingSetting.RequiresDBReset,
		CreatedAt:       existingSetting.CreatedAt,
		ModifiedAt:      existingSetting.ModifiedAt,
	}
	render.JSON(w, r, response)
}

// UpdateByKey updates a setting by its key
func (rs *Resource) UpdateByKey(w http.ResponseWriter, r *http.Request) {
	// Get setting key from URL
	key := chi.URLParam(r, "key")

	// Check if setting exists
	_, err := rs.Store.GetByKey(r.Context(), key)
	if err != nil {
		render.Render(w, r, ErrNotFound())
		return
	}

	// Parse request to get the new value
	req := make(map[string]string)

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	value, ok := req["value"]
	if !ok {
		render.Render(w, r, ErrInvalidRequest(errors.New("value is required")))
		return
	}

	// Validate value
	if value == "" {
		render.Render(w, r, ErrInvalidRequest(errors.New("value is required")))
		return
	}

	// Update setting value
	if err := rs.Store.UpdateByKey(r.Context(), key, value); err != nil {
		rs.Logger.WithField("error", err).Error("Failed to update setting by key")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	// Get updated setting
	updatedSetting, err := rs.Store.GetByKey(r.Context(), key)
	if err != nil {
		rs.Logger.WithField("error", err).Error("Failed to retrieve updated setting")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	// Return updated setting
	response := &models.SettingResponse{
		ID:              updatedSetting.ID,
		Key:             updatedSetting.Key,
		Value:           updatedSetting.Value,
		Category:        updatedSetting.Category,
		Description:     updatedSetting.Description,
		RequiresRestart: updatedSetting.RequiresRestart,
		RequiresDBReset: updatedSetting.RequiresDBReset,
		CreatedAt:       updatedSetting.CreatedAt,
		ModifiedAt:      updatedSetting.ModifiedAt,
	}
	render.JSON(w, r, response)
}

// Get retrieves a setting by ID
func (rs *Resource) Get(w http.ResponseWriter, r *http.Request) {
	// Get setting ID from URL
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(ErrInvalidSettingID))
		return
	}

	// Get setting
	setting, err := rs.Store.Get(r.Context(), id)
	if err != nil {
		render.Render(w, r, ErrNotFound())
		return
	}

	// Return setting
	response := &models.SettingResponse{
		ID:              setting.ID,
		Key:             setting.Key,
		Value:           setting.Value,
		Category:        setting.Category,
		Description:     setting.Description,
		RequiresRestart: setting.RequiresRestart,
		RequiresDBReset: setting.RequiresDBReset,
		CreatedAt:       setting.CreatedAt,
		ModifiedAt:      setting.ModifiedAt,
	}
	render.JSON(w, r, response)
}

// GetByKey retrieves a setting by its key
func (rs *Resource) GetByKey(w http.ResponseWriter, r *http.Request) {
	// Get setting key from URL
	key := chi.URLParam(r, "key")

	// Get setting
	setting, err := rs.Store.GetByKey(r.Context(), key)
	if err != nil {
		render.Render(w, r, ErrNotFound())
		return
	}

	// Return setting
	response := &models.SettingResponse{
		ID:              setting.ID,
		Key:             setting.Key,
		Value:           setting.Value,
		Category:        setting.Category,
		Description:     setting.Description,
		RequiresRestart: setting.RequiresRestart,
		RequiresDBReset: setting.RequiresDBReset,
		CreatedAt:       setting.CreatedAt,
		ModifiedAt:      setting.ModifiedAt,
	}
	render.JSON(w, r, response)
}

// GetByCategory retrieves all settings by category
func (rs *Resource) GetByCategory(w http.ResponseWriter, r *http.Request) {
	// Get category from URL
	category := chi.URLParam(r, "category")

	// Get settings
	settings, err := rs.Store.GetByCategory(r.Context(), category)
	if err != nil {
		rs.Logger.WithField("error", err).Error("Failed to get settings by category")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	// Convert to response objects
	var responses []*models.SettingResponse
	for _, setting := range settings {
		responses = append(responses, &models.SettingResponse{
			ID:              setting.ID,
			Key:             setting.Key,
			Value:           setting.Value,
			Category:        setting.Category,
			Description:     setting.Description,
			RequiresRestart: setting.RequiresRestart,
			RequiresDBReset: setting.RequiresDBReset,
			CreatedAt:       setting.CreatedAt,
			ModifiedAt:      setting.ModifiedAt,
		})
	}

	// Return settings
	render.JSON(w, r, responses)
}

// List retrieves all settings
func (rs *Resource) List(w http.ResponseWriter, r *http.Request) {
	// Get all settings
	settings, err := rs.Store.List(r.Context())
	if err != nil {
		rs.Logger.WithField("error", err).Error("Failed to list settings")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	// Convert to response objects
	var responses []*models.SettingResponse
	for _, setting := range settings {
		responses = append(responses, &models.SettingResponse{
			ID:              setting.ID,
			Key:             setting.Key,
			Value:           setting.Value,
			Category:        setting.Category,
			Description:     setting.Description,
			RequiresRestart: setting.RequiresRestart,
			RequiresDBReset: setting.RequiresDBReset,
			CreatedAt:       setting.CreatedAt,
			ModifiedAt:      setting.ModifiedAt,
		})
	}

	// Return settings
	render.JSON(w, r, responses)
}

// Delete deletes a setting
func (rs *Resource) Delete(w http.ResponseWriter, r *http.Request) {
	// Get setting ID from URL
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(ErrInvalidSettingID))
		return
	}

	// Check if setting exists
	_, err = rs.Store.Get(r.Context(), id)
	if err != nil {
		render.Render(w, r, ErrNotFound())
		return
	}

	// Delete setting
	if err := rs.Store.Delete(r.Context(), id); err != nil {
		rs.Logger.WithField("error", err).Error("Failed to delete setting")
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	// Return success
	render.NoContent(w, r)
}
