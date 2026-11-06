package owners

import (
	"context"
	"errors"

	settingsValues "github.com/moto-nrw/project-phoenix/modules/settings"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// ParentChildReadsForBookedSchools hides the children of a school that has
// not booked Eltern-App from the parents portal (#4895). A guardian with
// children at two schools keeps the portal while one of them books the
// module; the other school's children, their messages, Elternbriefe and
// requests stay out of it. Every per-child flow resolves the child here, so
// such a child reads as not linked. A failed read is an error, never a
// booking decision. Inside a parents-portal request the request cache
// already holds the switch: ParentScopeMiddleware read it for every
// guardian school.
func ParentChildReadsForBookedSchools(reads parentportal.ChildReads, reader settingsValues.ModuleBookingReader) (parentportal.ChildReads, error) {
	if reads == nil {
		return nil, nil
	}
	booking, err := settingsValues.NewModuleBooking(reader)
	if err != nil {
		return nil, err
	}
	return bookedParentChildReads{reads: reads, booked: func(ctx context.Context, tenantID int64) (bool, error) {
		err := booking.RequireForTenant(ctx, tenantID, settingsValues.ModuleElternApp)
		var notBooked *settingsValues.ModuleNotBookedError
		if errors.As(err, &notBooked) {
			return false, nil
		}
		return err == nil, err
	}}, nil
}

type bookedParentChildReads struct {
	reads  parentportal.ChildReads
	booked func(context.Context, int64) (bool, error)
}

func (r bookedParentChildReads) FindForAccount(ctx context.Context, accountID, studentID int64) (*parentportal.ChildSummary, error) {
	child, err := r.reads.FindForAccount(ctx, accountID, studentID)
	if err != nil {
		return nil, err
	}
	if child == nil {
		return nil, nil
	}
	booked, err := r.booked(ctx, child.TenantID)
	if err != nil {
		return nil, err
	}
	if !booked {
		// Not linked as far as the portal is concerned.
		return nil, nil
	}
	return child, nil
}

func (r bookedParentChildReads) ListByAccount(ctx context.Context, accountID int64) ([]*parentportal.ChildSummary, error) {
	children, err := r.reads.ListByAccount(ctx, accountID)
	if err != nil {
		return children, err
	}
	kept := make([]*parentportal.ChildSummary, 0, len(children))
	bookedBySchool := map[int64]bool{}
	for _, child := range children {
		booked, seen := bookedBySchool[child.TenantID]
		if !seen {
			booked, err = r.booked(ctx, child.TenantID)
			if err != nil {
				return nil, err
			}
			bookedBySchool[child.TenantID] = booked
		}
		if booked {
			kept = append(kept, child)
		}
	}
	return kept, nil
}
