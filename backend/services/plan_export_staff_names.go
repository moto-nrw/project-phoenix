package services

import (
	"context"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/planexport"
)

// staffWithPersonReader is the slice of the retained staff repository the
// plan export's staff names read.
type staffWithPersonReader interface {
	FindWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*usersModels.Staff, error)
}

// planExportStaffNames serves the plan export's staff-name port from the
// retained staff rows. The Document Rendering adapter may not name the
// People Directory rows itself, so the translation sits at the root (#3550).
type planExportStaffNames struct {
	staff staffWithPersonReader
}

func (n planExportStaffNames) StaffByIDs(ctx context.Context, ids []int64) (map[int64]*planexport.StaffMember, error) {
	members, err := n.staff.FindWithPersonByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]*planexport.StaffMember, len(members))
	for id, member := range members {
		if member == nil {
			// A staff row without a record keeps its slot and prints as
			// "Unbekannt", exactly as the retained service did.
			out[id] = nil
			continue
		}
		record := &planexport.StaffMember{ID: member.ID}
		if member.Person != nil {
			record.FirstName, record.LastName = member.Person.FirstName, member.Person.LastName
		}
		out[id] = record
	}
	return out, nil
}
