package studentdeletionhttp

import (
	chi "github.com/go-chi/chi/v5"
	common "github.com/moto-nrw/project-phoenix/api/common"
	permissions "github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	careplan "github.com/moto-nrw/project-phoenix/modules/careplan"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	studentdeletion "github.com/moto-nrw/project-phoenix/workflows/studentdeletion"
)

type ResourceConfig struct {
	CareLifecycleService careplan.CareLifecycle
	ConstraintViolation  func(error) bool
	PeopleDirectory      peopleModule.Capability
	StudentDeletion      *studentdeletion.Workflow
	UserContextService   CallerContext
}

type Resource struct{ ResourceConfig }

func NewResource(cfg ResourceConfig) *Resource { return &Resource{ResourceConfig: cfg} }

func (rs *Resource) Register(r chi.Router, withTx common.Middleware) {
	r.With(common.RequiresPermission(permissions.UsersDelete), withTx).Get("/care-withdrawals/{completionId}/deletion-impact", rs.previewWithdrawalDeletion)
	r.With(common.RequiresPermission(permissions.UsersDelete), withTx).Delete("/care-withdrawals/{completionId}", rs.deleteWithdrawalStudent)
	r.With(common.RequiresPermission(permissions.UsersDelete), withTx).Delete("/{id}", rs.deleteStudent)
	r.With(common.RequiresPermission(permissions.UsersDelete), withTx).Get("/{id}/delete-impact", rs.getStudentDeleteImpact)
	r.With(common.RequiresPermission(permissions.UsersDelete), withTx).Delete("/{id}/purge", rs.purgeGraduatedStudent)
}
