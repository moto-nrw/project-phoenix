package facilities

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
)

type WCService interface {
	EnsureInfrastructure(context.Context) (*SystemActivity, error)
}

type wcService struct {
	rooms      Service
	activities ActivityCatalog
	logger     *slog.Logger
}

var errWCActivityNotFound = errors.New("WC activity not found")

func NewWCService(rooms Service, activities ActivityCatalog, logger *slog.Logger) WCService {
	if rooms == nil || activities == nil {
		panic("WC service: rooms and activities are required")
	}
	return &wcService{rooms: rooms, activities: activities, logger: logger}
}

func (s *wcService) getLogger() *slog.Logger { return loggerOrDefault(s.logger) }

func (s *wcService) EnsureInfrastructure(ctx context.Context) (*SystemActivity, error) {
	activity, err := s.findActivity(ctx)
	if err == nil {
		return activity, nil
	}
	if !errors.Is(err, errWCActivityNotFound) {
		return nil, fmt.Errorf("failed to look up WC activity: %w", err)
	}
	room, err := s.ensureRoom(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure WC room: %w", err)
	}
	category, err := s.ensureCategory(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure WC category: %w", err)
	}
	created, err := s.activities.CreateActivity(ctx, SystemActivity{
		Name: facilitiesModule.WCActivityName, MaxParticipants: facilitiesModule.WCMaxParticipants,
		IsOpen: true, CategoryID: category.ID, PlannedRoomID: &room.ID, IsSystem: true,
	})
	if err != nil {
		if retry, retryErr := s.findActivity(ctx); retryErr == nil {
			return retry, nil
		}
		return nil, fmt.Errorf("failed to create WC activity: %w", err)
	}
	s.getLogger().Info(
		"successfully created WC infrastructure",
		"component", "wc",
		"room_id", room.ID,
		"category_id", category.ID,
		"activity_id", created.ID,
	)
	return &created, nil
}

func (s *wcService) findActivity(ctx context.Context) (*SystemActivity, error) {
	groups, err := s.activities.ListActivities(ctx, facilitiesModule.WCActivityName)
	if err != nil {
		return nil, fmt.Errorf("failed to query WC activity: %w", err)
	}
	if len(groups) == 0 {
		return nil, errWCActivityNotFound
	}
	return &groups[0], nil
}

func (s *wcService) ensureRoom(ctx context.Context) (*facilitiesModule.Room, error) {
	room, err := s.rooms.FindToiletRoom(ctx, 0)
	if err == nil {
		return room, nil
	}
	if !errors.Is(err, ErrRoomNotFound) {
		return nil, fmt.Errorf("failed to look up WC room: %w", err)
	}
	room = &facilitiesModule.Room{
		Name: facilitiesModule.WCRoomName, Capacity: pointer(facilitiesModule.WCRoomCapacity),
		Category: pointer(facilitiesModule.WCCategoryName), Color: pointer(facilitiesModule.WCColor), IsSystem: true,
	}
	if err := s.rooms.CreateRoom(ctx, room); err != nil {
		if retry, retryErr := s.rooms.FindToiletRoom(ctx, 0); retryErr == nil {
			return retry, nil
		}
		return nil, fmt.Errorf("failed to create WC room: %w", err)
	}
	return room, nil
}

func (s *wcService) ensureCategory(ctx context.Context) (*SystemCategory, error) {
	categories, err := s.activities.ListCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list activity categories: %w", err)
	}
	for index := range categories {
		if categories[index].Name == facilitiesModule.WCCategoryName {
			return &categories[index], nil
		}
	}
	created, err := s.activities.CreateCategory(ctx, SystemCategory{
		Name: facilitiesModule.WCCategoryName, Description: facilitiesModule.WCCategoryDescription,
		Color: facilitiesModule.WCColor, IsSystem: true,
	})
	if err != nil {
		if retry, retryErr := s.activities.ListCategories(ctx); retryErr == nil {
			for index := range retry {
				if retry[index].Name == facilitiesModule.WCCategoryName {
					return &retry[index], nil
				}
			}
		}
		return nil, fmt.Errorf("failed to create WC category: %w", err)
	}
	return &created, nil
}

func pointer[T any](value T) *T { return &value }

func loggerOrDefault(logger *slog.Logger) *slog.Logger {
	if logger != nil {
		return logger
	}
	return slog.Default()
}
