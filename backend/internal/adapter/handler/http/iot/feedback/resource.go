package feedback

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	feedbackSvc "github.com/moto-nrw/project-phoenix/internal/core/service/feedback"
	iotSvc "github.com/moto-nrw/project-phoenix/internal/core/service/iot"
	usersSvc "github.com/moto-nrw/project-phoenix/internal/core/service/users"
)

// Resource defines the Feedback API resource
type Resource struct {
	IoTService      iotSvc.Service
	UsersService    usersSvc.PersonService
	FeedbackService feedbackSvc.Service
}

// NewResource creates a new Feedback resource
func NewResource(iotService iotSvc.Service, usersService usersSvc.PersonService, feedbackService feedbackSvc.Service) *Resource {
	return &Resource{
		IoTService:      iotService,
		UsersService:    usersService,
		FeedbackService: feedbackService,
	}
}

// Router returns a configured router for feedback submission endpoints
// This router is mounted under /iot/ and handles device-based feedback submission
// All routes require device authentication (API key + Staff PIN)
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Feedback submission endpoint
	r.Post("/feedback", rs.deviceSubmitFeedback)

	return r
}
