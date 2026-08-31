package data

import (
	"cmp"
	"context"
	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	feedbackModule "github.com/moto-nrw/project-phoenix/modules/feedback"
)

type Feedback interface {
	Available(context.Context) (bool, error)
	Submit(context.Context, feedbackModule.CreateEntry) (feedbackModule.Entry, error)
}

type FeedbackStudentReader interface {
	GetStudentByIDForUpdate(context.Context, int64) (*usersModel.Student, error)
}

// FeedbackResource defines the Feedback API resource
type FeedbackResource struct {
	UsersService    FeedbackStudentReader
	FeedbackService Feedback
	ObserveResponse func(int, string)
	Logger          *slog.Logger
}

// NewFeedbackResource creates a new Feedback resource
func NewFeedbackResource(usersService FeedbackStudentReader, feedbackService Feedback, observeResponse func(int, string), logger *slog.Logger) *FeedbackResource {
	if usersService == nil || feedbackService == nil || observeResponse == nil {
		panic("IoT feedback: all dependencies are required")
	}
	return &FeedbackResource{
		UsersService:    usersService,
		FeedbackService: feedbackService,
		ObserveResponse: observeResponse,
		Logger:          logger,
	}
}

func (rs *FeedbackResource) getLogger() *slog.Logger {
	return cmp.Or(rs.Logger, slog.Default())
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
