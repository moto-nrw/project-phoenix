package requestreviewhttp

import (
	chi "github.com/go-chi/chi/v5"
	render "github.com/go-chi/render"
	common "github.com/moto-nrw/project-phoenix/api/common"
	permissions "github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	requestreview "github.com/moto-nrw/project-phoenix/modules/requestreview"
)

type ResourceConfig struct {
	PeopleDirectory     peopleModule.Capability
	RequestReview       requestreview.Query
	RequestReviewAccess ParentRequestReviewAccess
	QueueError          func(error) render.Renderer
}

type Resource struct{ ResourceConfig }

func NewResource(cfg ResourceConfig) *Resource { return &Resource{ResourceConfig: cfg} }

func (rs *Resource) Register(r chi.Router, withTx common.Middleware) {
	r.With(common.RequiresAnyPermission(permissions.UsersUpdate, permissions.UsersAbsence), withTx).Get("/change-requests/pending-count", rs.pendingChangeRequestCount)
	r.With(withTx).Get("/change-requests/access", rs.changeRequestAccess)
	r.With(common.RequiresAnyPermission(permissions.UsersUpdate, permissions.UsersAbsence), withTx).Get("/change-requests", rs.listAggregatedChangeRequests)
}
