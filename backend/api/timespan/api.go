// Package timespan provides the timespan management API
package timespan

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

// Resource defines the timespan management resource
type Resource struct {
	Store     TimespanStore
	AuthStore AuthTokenStore
}

// TimespanStore defines database operations for timespan management
type TimespanStore interface {
	CreateTimespan(ctx context.Context, startTime time.Time, endTime *time.Time) (*models2.Timespan, error)
	GetTimespan(ctx context.Context, id int64) (*models2.Timespan, error)
	UpdateTimespanEndTime(ctx context.Context, id int64, endTime time.Time) error
	DeleteTimespan(ctx context.Context, id int64) error
}

// AuthTokenStore defines operations for the auth token store
type AuthTokenStore interface {
	GetToken(t string) (*jwt.Token, error)
}

// NewResource creates a new timespan management resource
func NewResource(store TimespanStore, authStore AuthTokenStore) *Resource {
	return &Resource{
		Store:     store,
		AuthStore: authStore,
	}
}

// Router creates a router for timespan management
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()

	// JWT protected routes
	r.Group(func(r chi.Router) {
		r.Use(jwt.Authenticator)

		r.Post("/", rs.createTimespan)
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", rs.getTimespan)
			r.Put("/end", rs.updateTimespanEndTime)
			r.Delete("/", rs.deleteTimespan)
		})
	})

	return r
}

// ======== Request/Response Models ========

// TimespanRequest is the request payload for Timespan data
type TimespanRequest struct {
	StartTime time.Time  `json:"start_time"`
	EndTime   *time.Time `json:"end_time,omitempty"`
}

// TimespanEndTimeRequest is the request payload for updating end time
type TimespanEndTimeRequest struct {
	EndTime time.Time `json:"end_time"`
}

// Bind preprocesses a TimespanRequest
func (req *TimespanRequest) Bind(r *http.Request) error {
	// Basic validation
	if req.StartTime.IsZero() {
		return errors.New("start time is required")
	}
	return nil
}

// Bind preprocesses a TimespanEndTimeRequest
func (req *TimespanEndTimeRequest) Bind(r *http.Request) error {
	// Basic validation
	if req.EndTime.IsZero() {
		return errors.New("end time is required")
	}
	return nil
}

// ======== Timespan Handlers ========

// createTimespan creates a new timespan
func (rs *Resource) createTimespan(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	var data TimespanRequest
	if err := render.Bind(r, &data); err != nil {
		logger.WithError(err).Warn("Invalid timespan creation request")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	ctx := r.Context()
	timespan, err := rs.Store.CreateTimespan(ctx, data.StartTime, data.EndTime)
	if err != nil {
		logger.WithError(err).Error("Failed to create timespan")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithFields(logrus.Fields{
		"timespan_id": timespan.ID,
		"start_time":  timespan.StartTime,
		"end_time":    timespan.EndTime,
	}).Info("Timespan created successfully")

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, timespan)
}

// getTimespan returns a specific timespan
func (rs *Resource) getTimespan(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid timespan ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	ctx := r.Context()
	timespan, err := rs.Store.GetTimespan(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to get timespan by ID")
		render.Render(w, r, ErrNotFound())
		return
	}

	render.JSON(w, r, timespan)
}

// updateTimespanEndTime updates the end time of a specific timespan
func (rs *Resource) updateTimespanEndTime(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid timespan ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	var data TimespanEndTimeRequest
	if err := render.Bind(r, &data); err != nil {
		logger.WithError(err).Warn("Invalid end time update request")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	if data.EndTime.IsZero() {
		logger.Warn("End time is required")
		render.Render(w, r, ErrInvalidRequest(errors.New("end time is required")))
		return
	}

	ctx := r.Context()
	if err := rs.Store.UpdateTimespanEndTime(ctx, id, data.EndTime); err != nil {
		logger.WithError(err).Error("Failed to update timespan end time")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	// Get the updated timespan
	timespan, err := rs.Store.GetTimespan(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to retrieve updated timespan")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithField("timespan_id", id).Info("Timespan end time updated successfully")
	render.JSON(w, r, timespan)
}

// deleteTimespan deletes a specific timespan
func (rs *Resource) deleteTimespan(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid timespan ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	ctx := r.Context()
	if err := rs.Store.DeleteTimespan(ctx, id); err != nil {
		logger.WithError(err).Error("Failed to delete timespan")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithField("timespan_id", id).Info("Timespan deleted successfully")
	render.NoContent(w, r)
}

// Use the error handling functions from errors.go
