package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/domain"
)

// ListDisplays returns every info-point screen of the caller's tenant.
func (s *Service) ListDisplays(ctx context.Context) ([]devicefleet.Display, error) {
	var displays []domain.Display
	err := s.runInTenant(ctx, "list_displays", func(ctx context.Context, stats *domain.OperationStats) error {
		found, queryStats, err := s.deps.Displays.List(ctx)
		stats.Add(queryStats)
		displays = found
		return err
	})
	result := make([]devicefleet.Display, 0, len(displays))
	for _, value := range displays {
		result = append(result, toPublicDisplay(value))
	}
	return result, err
}

// CreateDisplay registers a screen and returns it with its raw access token.
// The raw token is returned exactly once and never stored.
func (s *Service) CreateDisplay(ctx context.Context, name string) (devicefleet.Display, string, error) {
	var created domain.Display
	var rawToken string
	err := s.runWrite(ctx, "create_display", func(ctx context.Context, stats *domain.OperationStats) error {
		validated, err := validateDisplayName(name)
		if err != nil {
			return err
		}
		raw, hash, err := s.deps.Tokens.Mint()
		if err != nil {
			return err
		}
		stored, queryStats, err := s.deps.Displays.Create(ctx, domain.CreateDisplay{Name: validated, TokenHash: hash})
		stats.Add(queryStats)
		if err != nil {
			return fmt.Errorf("failed to create display: %w", err)
		}
		created = stored
		rawToken = raw
		return nil
	})
	if err != nil {
		return devicefleet.Display{}, "", err
	}
	return toPublicDisplay(created), rawToken, nil
}

// UpdateDisplay changes the name and/or the active state of one screen.
func (s *Service) UpdateDisplay(ctx context.Context, id int64, name *string, isActive *bool) (devicefleet.Display, error) {
	if id <= 0 {
		return devicefleet.Display{}, s.reject("update_display", devicefleet.ErrInvalidDisplayInput)
	}
	var result domain.Display
	err := s.runWrite(ctx, "update_display", func(ctx context.Context, stats *domain.OperationStats) error {
		existing, found, queryStats, err := s.deps.Displays.FindByID(ctx, id)
		stats.Add(queryStats)
		if err != nil {
			return fmt.Errorf("failed to load display: %w", err)
		}
		if !found {
			return devicefleet.ErrDisplayNotFound
		}
		input, err := applyDisplayUpdate(&existing, name, isActive)
		if err != nil {
			return err
		}
		if input.Name == nil && input.IsActive == nil {
			result = existing
			return nil
		}
		existing.UpdatedAt = s.deps.Now()
		affected, queryStats, err := s.deps.Displays.Update(ctx, input)
		stats.Add(queryStats)
		if err != nil {
			return fmt.Errorf("failed to update display: %w", err)
		}
		if affected == 0 {
			// The row vanished between the read and the update.
			return devicefleet.ErrDisplayNotFound
		}
		result = existing
		return nil
	})
	return toPublicDisplay(result), err
}

// RegenerateDisplayToken mints a new access token, invalidating the old one.
func (s *Service) RegenerateDisplayToken(ctx context.Context, id int64) (string, error) {
	if id <= 0 {
		return "", s.reject("regenerate_display_token", devicefleet.ErrInvalidDisplayInput)
	}
	var rawToken string
	err := s.runWrite(ctx, "regenerate_display_token", func(ctx context.Context, stats *domain.OperationStats) error {
		existing, found, queryStats, err := s.deps.Displays.FindByID(ctx, id)
		stats.Add(queryStats)
		if err != nil {
			return fmt.Errorf("failed to load display: %w", err)
		}
		if !found {
			return devicefleet.ErrDisplayNotFound
		}
		raw, hash, err := s.deps.Tokens.Mint()
		if err != nil {
			return err
		}
		affected, queryStats, err := s.deps.Displays.Update(ctx, domain.UpdateDisplay{ID: id, TokenHash: &hash})
		stats.Add(queryStats)
		if err != nil {
			return fmt.Errorf("failed to regenerate display token: %w", err)
		}
		if affected == 0 {
			// Never hand out a token that was not persisted.
			return devicefleet.ErrDisplayNotFound
		}
		existing.TokenHash = hash
		rawToken = raw
		return nil
	})
	if err != nil {
		return "", err
	}
	return rawToken, nil
}

// DeleteDisplay removes one screen permanently.
func (s *Service) DeleteDisplay(ctx context.Context, id int64) error {
	if id <= 0 {
		return s.reject("delete_display", devicefleet.ErrInvalidDisplayInput)
	}
	return s.runWrite(ctx, "delete_display", func(ctx context.Context, stats *domain.OperationStats) error {
		_, found, queryStats, err := s.deps.Displays.FindByID(ctx, id)
		stats.Add(queryStats)
		if err != nil {
			return fmt.Errorf("failed to load display: %w", err)
		}
		if !found {
			return devicefleet.ErrDisplayNotFound
		}
		affected, queryStats, err := s.deps.Displays.Delete(ctx, id)
		stats.Add(queryStats)
		if err != nil {
			return fmt.Errorf("failed to delete display: %w", err)
		}
		if affected == 0 {
			return devicefleet.ErrDisplayNotFound
		}
		return nil
	})
}

func validateDisplayName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", &devicefleet.InvalidDisplayError{Reason: "display name is required"}
	}
	if len(name) > domain.MaxDisplayNameLength {
		return "", &devicefleet.InvalidDisplayError{
			Reason: fmt.Sprintf("display name cannot exceed %d characters", domain.MaxDisplayNameLength),
		}
	}
	return name, nil
}

func applyDisplayUpdate(existing *domain.Display, name *string, isActive *bool) (domain.UpdateDisplay, error) {
	input := domain.UpdateDisplay{ID: existing.ID}
	if name != nil {
		validated, err := validateDisplayName(*name)
		if err != nil {
			return domain.UpdateDisplay{}, err
		}
		input.Name = &validated
		existing.Name = validated
	}
	if isActive != nil {
		input.IsActive = isActive
		existing.IsActive = *isActive
	}
	return input, nil
}
