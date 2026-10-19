package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// NewParentCareOfferingReads binds the public catalog to the workflow's
// booked-offering values. The owner retains decoding and lookup errors.
func NewParentCareOfferingReads(source enrollment.ParentCareOfferingQueries) parentportal.CareOfferingReads {
	if source == nil {
		return nil
	}
	return parentCareOfferingReads{source: source}
}

type parentCareOfferingReads struct {
	source enrollment.ParentCareOfferingQueries
}

func (r parentCareOfferingReads) ListByIDs(ctx context.Context, ids []int64) ([]*parentportal.CareOffering, error) {
	rows, err := r.source.CareOfferingsByIDs(ctx, ids)
	if rows == nil {
		return nil, err
	}
	out := make([]*parentportal.CareOffering, len(rows))
	for i, row := range rows {
		if row != nil {
			out[i] = &parentportal.CareOffering{
				ID: row.ID, Name: row.Name, Description: row.Description,
				FixedDays:     row.HasFixedDays(),
				AvailableDays: row.AvailableDays, IncludesHolidayCare: row.IncludesHolidayCare,
				IncludesLunch: row.IncludesLunch, PriceCents: row.PriceCents, SortOrder: row.SortOrder,
			}
		}
	}
	return out, err
}

func NewParentOfferingChangeReads(source enrollment.ParentOfferingChangeQueries) parentportal.OfferingChangeRequestReads {
	if source == nil {
		return nil
	}
	return parentOfferingChangeReads{source: source}
}

type parentOfferingChangeReads struct {
	source enrollment.ParentOfferingChangeQueries
}

func (r parentOfferingChangeReads) FindByID(ctx context.Context, id int64) (*parentportal.OfferingChangeIdentity, error) {
	row, err := r.source.OfferingChangeIdentity(ctx, id)
	if row == nil {
		return nil, err
	}
	out := parentportal.OfferingChangeIdentity(*row)
	return &out, err
}
