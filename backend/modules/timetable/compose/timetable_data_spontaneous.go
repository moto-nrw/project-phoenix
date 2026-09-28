package compose

import (
	"context"
	"errors"
	"fmt"
	"strings"

	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// spontaneousCategoryName is the category every activity of a web
// spontaneous start is filed under.
const spontaneousCategoryName = "Spontan"

func (d *timetableData) SpontaneousRoomExists(ctx context.Context, roomID int64) (bool, error) {
	_, ok, err := d.deps.Rooms.RoomName(ctx, roomID)
	if err != nil && modelBase.IsNoRows(err) {
		return false, nil
	}
	return ok, err
}

func (d *timetableData) SpontaneousRoomOccupied(ctx context.Context, roomID int64) (bool, error) {
	return d.deps.RoomOccupancy.RoomOccupied(ctx, roomID)
}

// LockSpontaneousStartRoom serializes spontaneous starts into one room, so
// its existence and occupancy checks describe the same state.
func (d *timetableData) LockSpontaneousStartRoom(ctx context.Context, roomID int64) error {
	return d.acquireSpontaneousLock(ctx, fmt.Sprintf("timetable:spontaneous-start-room:%d:%d", tenant.FromContext(ctx), roomID))
}

// ResolveSpontaneousActivity returns the requested activity or the activity
// named after the title; a missing one is created in the "Spontan" category
// under a name lock, so two starts with the same title share it.
func (d *timetableData) ResolveSpontaneousActivity(ctx context.Context, title string, requestedID *int64, createdBy int64) (*int64, error) {
	if requestedID != nil {
		return requestedID, nil
	}
	nameKey := strings.ToLower(strings.TrimSpace(title))
	if err := d.acquireSpontaneousLock(ctx, fmt.Sprintf("timetable:spontaneous-activity-name:%d:%s", tenant.FromContext(ctx), nameKey)); err != nil {
		return nil, err
	}
	if existing, err := d.deps.Templates.FindByName(ctx, title); err == nil && existing != nil {
		return &existing.ID, nil
	} else if err != nil && !modelBase.IsNoRows(err) {
		return nil, err
	}
	category, err := d.ensureSpontaneousActivityCategory(ctx)
	if err != nil {
		return nil, err
	}
	group := &activitiesModels.Group{
		Name:            title,
		CategoryID:      category.ID,
		MaxParticipants: 0,
		IsOpen:          true,
		CreatedBy:       &createdBy,
		Type:            activitiesModels.GroupTypeActivity,
		IsTemplate:      false,
	}
	group.SetTenantID(tenant.FromContext(ctx))
	if err := d.deps.Templates.Create(ctx, group); err != nil {
		return nil, err
	}
	return &group.ID, nil
}

// ensureSpontaneousActivityCategory returns the "Spontan" category, creating
// it on first use; an archived one blocks the start until it is restored.
func (d *timetableData) ensureSpontaneousActivityCategory(ctx context.Context) (*activitiesModels.Category, error) {
	if err := d.acquireSpontaneousLock(ctx, fmt.Sprintf("timetable:spontaneous-activity-category:%d", tenant.FromContext(ctx))); err != nil {
		return nil, err
	}
	if existing, err := d.deps.Categories.FindByNameIncludingArchivedForShare(ctx, spontaneousCategoryName); err == nil && existing != nil {
		if existing.IsArchived() {
			return nil, timetable.ErrSpontaneousCategoryArchived
		}
		return existing, nil
	} else if err != nil && !modelBase.IsNoRows(err) {
		return nil, err
	}
	category := &activitiesModels.Category{
		Name:        spontaneousCategoryName,
		Description: "Automatisch angelegte Aktivitäten aus spontanen Web-Starts",
		Color:       "#83CD2D",
	}
	category.SetTenantID(tenant.FromContext(ctx))
	if err := d.deps.Categories.Create(ctx, category); err != nil {
		return nil, err
	}
	return category, nil
}

// acquireSpontaneousLock takes a transaction-scoped advisory lock. A missing
// tenant or transaction is an error, except in compositions without
// advisory locks (unit tests), which skip the lock.
func (d *timetableData) acquireSpontaneousLock(ctx context.Context, key string) error {
	if tenant.FromContext(ctx) <= 0 {
		return errors.New("tenant id is required")
	}
	if tx, ok := tenant.TransactionFromContext(ctx); !ok || tx == nil {
		if d.deps.Advisory == nil {
			return nil
		}
		return errors.New("tenant transaction is required")
	}
	if d.deps.Advisory == nil {
		return nil
	}
	return d.deps.Advisory.AcquireXactLock(ctx, key)
}
