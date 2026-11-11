package importpkg

import (
	"context"

	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	"github.com/moto-nrw/project-phoenix/modules/dataimport/ports"
)

// BatchWriteConfig locks the staff rows a chunk updates, ascending, before
// its first row (#6702). Staff rows are one ascending class of the unit of
// work's lock order and come before every caregiver profile, master data and
// qualification lock. Locked row by row, the second update of a chunk would
// ask for its staff row after the first one's caregiver profile and be
// refused. An upsert takes the staff person merge gate first, which comes
// before the staff rows.
//
// The locked rows replace each update's own read of its staff row, and the
// e-mail matches found here are kept for the chunk, so the chunk issues no
// statement more than before.
func (c *StaffImportConfig) BatchWriteConfig(ctx context.Context, rows []importModels.StaffImportRow) (ports.ImportConfig[importModels.StaffImportRow], error) {
	c.lockedStaff, c.emailMatches = nil, nil
	mode := importModeFromContext(ctx)
	if c.Membership == nil || (mode != importModels.ImportModeUpdate && mode != importModels.ImportModeUpsert) {
		return c, nil
	}
	c.emailMatches = make(map[string]int64)
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		// A match error is the row's own failure; applyBatchRow reports it.
		if id, err := c.FindExisting(ctx, row); err == nil && id != nil {
			ids = append(ids, *id)
		}
	}
	if len(ids) == 0 {
		return c, nil
	}
	if err := c.lockStaffPersonMerge(ctx); err != nil {
		return nil, err
	}
	staff, err := c.Membership.ListStaff(ctx, ports.StaffFilter{IDs: ids, ForMutation: true})
	if err != nil {
		return nil, copyErrorf("Mitarbeiter sperren: %w", err)
	}
	c.lockedStaff = make(map[int64]ports.Staff, len(staff))
	// Missing rows are the row's own update failure, as before.
	for _, member := range staff {
		c.lockedStaff[member.ID] = member
	}
	return c, nil
}

// lockedStaffRow hands an update the staff row its chunk locked, once: a
// second row of the same person reads the row its predecessor wrote.
func (c *StaffImportConfig) lockedStaffRow(ctx context.Context, staffID int64) (ports.Staff, error) {
	if staff, ok := c.lockedStaff[staffID]; ok {
		delete(c.lockedStaff, staffID)
		return staff, nil
	}
	return c.Membership.FindStaff(ctx, staffID)
}

// matchedEmail returns the staff member a login e-mail matched earlier in
// the chunk. Only matches are kept: a row of the chunk may still create the
// staff member a later row's e-mail names.
func (c *StaffImportConfig) matchedEmail(ctx context.Context, email string) (*int64, error) {
	if id, ok := c.emailMatches[email]; ok {
		return &id, nil
	}
	id, err := c.findStaffByLoginEmail(ctx, email)
	if err == nil && id != nil && c.emailMatches != nil {
		c.emailMatches[email] = *id
	}
	return id, err
}
