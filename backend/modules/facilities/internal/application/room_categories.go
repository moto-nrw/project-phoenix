package application

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/facilities/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/facilities/internal/ports"
)

// NewCategories wires the service for the room category catalog of the
// caller's school (#4352). It serves no room operation.
func NewCategories(store ports.CategoryStore, tx ports.Transaction, observe ports.Observer) *Service {
	if store == nil || tx == nil || observe == nil {
		panic("facilities application: all dependencies are required")
	}
	return &Service{categories: store, tx: tx, observe: observe}
}

// ListRoomCategories returns the catalog with the number of ordinary rooms
// that name each category.
func (s *Service) ListRoomCategories(ctx context.Context) (result []facilities.RoomCategory, err error) {
	err = s.runWrite(ctx, "list_room_categories", func(txCtx context.Context, _ *domain.OperationStats) error {
		categories, listErr := s.categories.List(txCtx)
		if listErr != nil {
			return listErr
		}
		rooms, countErr := s.categories.CountRoomsByCategory(txCtx)
		if countErr != nil {
			return countErr
		}
		for index := range categories {
			categories[index].RoomCount = rooms[strings.ToLower(categories[index].Name)]
		}
		result = categories
		return nil
	})
	if result == nil {
		result = []facilities.RoomCategory{}
	}
	return result, err
}

func (s *Service) CreateRoomCategory(ctx context.Context, input facilities.RoomCategoryInput) (result facilities.RoomCategory, err error) {
	if err := normalizeCategory(&input); err != nil {
		s.ObserveRejection("create_room_category", 0, err)
		return facilities.RoomCategory{}, err
	}
	err = s.runWrite(ctx, "create_room_category", func(txCtx context.Context, _ *domain.OperationStats) error {
		if err := s.tx.AcquireLock(txCtx, roomCategoryNameLock(input.Name)); err != nil {
			return err
		}
		if err := s.ensureCategoryNameAvailable(txCtx, input.Name, 0); err != nil {
			return err
		}
		result, err = s.categories.Create(txCtx, input)
		return err
	})
	return result, err
}

// UpdateRoomCategory renames and recolors a category. A new name moves the
// rooms of the old name with it, so they keep their category and color.
func (s *Service) UpdateRoomCategory(ctx context.Context, id int64, input facilities.RoomCategoryInput) (result facilities.RoomCategory, err error) {
	if id <= 0 {
		err := &facilities.InvalidRoomError{Reason: "room category ID is required"}
		s.ObserveRejection("update_room_category", 0, err)
		return facilities.RoomCategory{}, err
	}
	if err := normalizeCategory(&input); err != nil {
		s.ObserveRejection("update_room_category", 0, err)
		return facilities.RoomCategory{}, err
	}
	err = s.runWrite(ctx, "update_room_category", func(txCtx context.Context, _ *domain.OperationStats) error {
		if err := s.tx.AcquireLock(txCtx, roomCategoryNameLock(input.Name)); err != nil {
			return err
		}
		existing, found, findErr := s.categories.FindForUpdate(txCtx, id)
		if findErr != nil {
			return findErr
		}
		if !found {
			return facilities.ErrRoomCategoryNotFound
		}
		if err := s.ensureCategoryNameAvailable(txCtx, input.Name, id); err != nil {
			return err
		}
		result, err = s.categories.Update(txCtx, id, input)
		if err != nil || existing.Name == input.Name {
			return err
		}
		return s.categories.RenameRooms(txCtx, existing.Name, input.Name)
	})
	return result, err
}

// DeleteRoomCategory removes a category no room names any more.
func (s *Service) DeleteRoomCategory(ctx context.Context, id int64) error {
	if id <= 0 {
		err := &facilities.InvalidRoomError{Reason: "room category ID is required"}
		s.ObserveRejection("delete_room_category", 0, err)
		return err
	}
	return s.runWrite(ctx, "delete_room_category", func(txCtx context.Context, _ *domain.OperationStats) error {
		existing, found, err := s.categories.FindForUpdate(txCtx, id)
		if err != nil {
			return err
		}
		if !found {
			return facilities.ErrRoomCategoryNotFound
		}
		rooms, err := s.categories.CountRoomsByCategory(txCtx)
		if err != nil {
			return err
		}
		if rooms[strings.ToLower(existing.Name)] > 0 {
			return facilities.ErrRoomCategoryInUse
		}
		return s.categories.Delete(txCtx, id)
	})
}

// SeedDefaultRoomCategories creates the defaults the school does not have.
// It checks before it writes, so a name the school already has never hits
// the unique index and aborts the caller's transaction (school provisioning
// runs it inside its own).
func (s *Service) SeedDefaultRoomCategories(ctx context.Context) error {
	return s.runWrite(ctx, "seed_room_categories", func(txCtx context.Context, _ *domain.OperationStats) error {
		existing, err := s.categories.List(txCtx)
		if err != nil {
			return err
		}
		have := make(map[string]struct{}, len(existing))
		for _, category := range existing {
			have[strings.ToLower(category.Name)] = struct{}{}
		}
		for _, category := range facilities.DefaultRoomCategories {
			if _, ok := have[strings.ToLower(category.Name)]; ok {
				continue
			}
			if _, err := s.categories.Create(txCtx, category); err != nil {
				return err
			}
		}
		return nil
	})
}

func roomCategoryNameLock(name string) string {
	return "facilities:room-category:" + strings.ToLower(name)
}

// ensureCategoryNameAvailable refuses a name another category of the school
// already has, ignoring case. Checking before the write keeps the unique
// index from aborting the caller's transaction; the name lock serializes
// concurrent writers of the same name.
func (s *Service) ensureCategoryNameAvailable(ctx context.Context, name string, ownID int64) error {
	categories, err := s.categories.List(ctx)
	if err != nil {
		return err
	}
	for _, category := range categories {
		if category.ID != ownID && strings.EqualFold(category.Name, name) {
			return facilities.ErrDuplicateRoomCategory
		}
	}
	return nil
}

func normalizeCategory(input *facilities.RoomCategoryInput) error {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return &facilities.InvalidRoomError{Reason: "room category name is required"}
	}
	if utf8.RuneCountInString(input.Name) > facilities.MaxRoomCategoryNameLength {
		return &facilities.InvalidRoomError{Reason: "room category name is too long"}
	}
	if strings.TrimSpace(input.Color) == "" {
		return &facilities.InvalidRoomError{Reason: "room category color is required"}
	}
	color, err := normalizeColor(input.Color)
	if err != nil {
		return err
	}
	input.Color = color
	return nil
}
