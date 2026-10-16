package data

import (
	"context"
	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	feedbackModule "github.com/moto-nrw/project-phoenix/modules/feedback"
)

type Feedback interface {
	Available(context.Context) (bool, error)
	Submit(context.Context, feedbackModule.CreateEntry) (feedbackModule.Entry, error)
}

type FeedbackStudent = devicescan.FeedbackStudent
type FeedbackStudentReader = devicescan.FeedbackStudents

// FeedbackResource defines the Feedback API resource
type FeedbackResource struct {
	runtime         Runtime
	Students        FeedbackStudentReader
	FeedbackService Feedback
	ObserveResponse func(int, string)
	Logger          *slog.Logger
}

// NewFeedbackResource creates a new Feedback resource
func NewFeedbackResource(students FeedbackStudentReader, feedbackService Feedback, observeResponse func(int, string), runtime Runtime, logger *slog.Logger) *FeedbackResource {
	if students == nil || feedbackService == nil || observeResponse == nil || !runtime.valid() {
		panic("IoT feedback: all dependencies are required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &FeedbackResource{
		runtime:         runtime,
		Students:        students,
		FeedbackService: feedbackService,
		ObserveResponse: observeResponse,
		Logger:          logger,
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
