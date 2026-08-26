package platform

import (
	"context"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/uptrace/bun"
)

const tablePlatformSchoolInvoices = "platform.school_invoices"

// SchoolInvoiceRepository is the data access for the per-school payment
// schedule. Everything except the ordered listing comes from the generic
// base.Repository — there are deliberately no FindByStatus/FindByPeriod
// clusters; callers filter through QueryOptions when they need to.
type SchoolInvoiceRepository struct {
	*base.Repository[*platform.SchoolInvoice]
	db *bun.DB
}

// NewSchoolInvoiceRepository builds the tenant-scoped invoice repository.
// TenantScoped adds the defense-in-depth WHERE tenant_id = ? on top of the
// RLS policy provisioned in migration 1.15.335.
func NewSchoolInvoiceRepository(db *bun.DB) platform.SchoolInvoiceRepository {
	repo := base.NewRepository[*platform.SchoolInvoice](db, tablePlatformSchoolInvoices, "SchoolInvoice")
	repo.TenantScoped = true
	return &SchoolInvoiceRepository{Repository: repo, db: db}
}

// ListForTenant returns the tenant's invoices, newest due date first.
//
// Custom rather than the plain equality-filter List because the order is part
// of the contract: the school reads this as a schedule, and an unordered list
// of dates is unreadable. id DESC keeps the order stable when two invoices
// share a due date.
func (r *SchoolInvoiceRepository) ListForTenant(ctx context.Context) ([]*platform.SchoolInvoice, error) {
	options := modelBase.NewQueryOptions()
	sorting := &modelBase.Sorting{}
	sorting.AddField("due_date", modelBase.SortDesc)
	sorting.AddField("id", modelBase.SortDesc)
	options.Sorting = sorting

	return r.ListWithOptions(ctx, options)
}
