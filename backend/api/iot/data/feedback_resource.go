package data

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	feedbackSvc "github.com/moto-nrw/project-phoenix/services/feedback"
	iotSvc "github.com/moto-nrw/project-phoenix/services/iot"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
)

// FeedbackResource defines the Feedback API resource
type FeedbackResource struct {
	IoTService      iotSvc.Service
	UsersService    usersSvc.PersonService
	FeedbackService feedbackSvc.Service
	SettingsService configSvc.SettingsService
}

// NewFeedbackResource creates a new Feedback resource
func NewFeedbackResource(iotService iotSvc.Service, usersService usersSvc.PersonService, feedbackService feedbackSvc.Service, settingsService configSvc.SettingsService) *FeedbackResource {
	return &FeedbackResource{
		IoTService:      iotService,
		UsersService:    usersService,
		FeedbackService: feedbackService,
		SettingsService: settingsService,
	}
}

// Router returns a configured router for feedback submission endpoints
// This router is mounted under /iot/ and handles device-based feedback submission
// All routes require device authentication (API key + Staff PIN)
func (rs *FeedbackResource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Feedback submission endpoint
	r.Post("/feedback", rs.deviceSubmitFeedback)

	return r
}
