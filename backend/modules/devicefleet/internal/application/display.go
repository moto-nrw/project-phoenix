package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/domain"
)

// ListDisplays returns every info-point screen of the caller's tenant.
func (s *Service) ListDisplays(ctx context.Context) ([]domain.Display, error) {
	var displays []domain.Display
	err := s.run(ctx, "list_displays", func(ctx context.Context, stats *domain.OperationStats) error {
		found, queryStats, err := s.deps.Displays.List(ctx)
		stats.Add(queryStats)
		displays = found
		return err
	})
	return displays, err
}

// CreateDisplay registers a screen and returns it with its raw access token.
// The raw token is returned exactly once and never stored.
func (s *Service) CreateDisplay(ctx context.Context, name string) (domain.Display, string, error) {
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
		return domain.Display{}, "", err
	}
	return created, rawToken, nil
}

// UpdateDisplay changes the name and/or the active state of one screen.
func (s *Service) UpdateDisplay(ctx context.Context, id int64, name *string, isActive *bool) (domain.Display, error) {
	var result domain.Display
	err := s.runWrite(ctx, "update_display", func(ctx context.Context, stats *domain.OperationStats) error {
		existing, found, queryStats, err := s.deps.Displays.FindByID(ctx, id)
		stats.Add(queryStats)
		if err != nil {
			return fmt.Errorf("failed to load display: %w", err)
		}
		if !found {
			return domain.ErrDisplayNotFound
		}
		input := domain.UpdateDisplay{ID: id}
		if name != nil {
			validated, err := validateDisplayName(*name)
			if err != nil {
				return err
			}
			input.Name = &validated
			existing.Name = validated
		}
		if isActive != nil {
			input.IsActive = isActive
			existing.IsActive = *isActive
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
			return domain.ErrDisplayNotFound
		}
		result = existing
		return nil
	})
	return result, err
}

// RegenerateDisplayToken mints a new access token, invalidating the old one.
func (s *Service) RegenerateDisplayToken(ctx context.Context, id int64) (domain.Display, string, error) {
	var display domain.Display
	var rawToken string
	err := s.runWrite(ctx, "regenerate_display_token", func(ctx context.Context, stats *domain.OperationStats) error {
		existing, found, queryStats, err := s.deps.Displays.FindByID(ctx, id)
		stats.Add(queryStats)
		if err != nil {
			return fmt.Errorf("failed to load display: %w", err)
		}
		if !found {
			return domain.ErrDisplayNotFound
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
			return domain.ErrDisplayNotFound
		}
		existing.TokenHash = hash
		display = existing
		rawToken = raw
		return nil
	})
	if err != nil {
		return domain.Display{}, "", err
	}
	return display, rawToken, nil
}

// DeleteDisplay removes one screen permanently.
func (s *Service) DeleteDisplay(ctx context.Context, id int64) error {
	return s.runWrite(ctx, "delete_display", func(ctx context.Context, stats *domain.OperationStats) error {
		_, found, queryStats, err := s.deps.Displays.FindByID(ctx, id)
		stats.Add(queryStats)
		if err != nil {
			return fmt.Errorf("failed to load display: %w", err)
		}
		if !found {
			return domain.ErrDisplayNotFound
		}
		affected, queryStats, err := s.deps.Displays.Delete(ctx, id)
		stats.Add(queryStats)
		if err != nil {
			return fmt.Errorf("failed to delete display: %w", err)
		}
		if affected == 0 {
			return domain.ErrDisplayNotFound
		}
		return nil
	})
}

func validateDisplayName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", &domain.InvalidDisplayError{Reason: "display name is required"}
	}
	if len(name) > domain.MaxDisplayNameLength {
		return "", &domain.InvalidDisplayError{
			Reason: fmt.Sprintf("display name cannot exceed %d characters", domain.MaxDisplayNameLength),
		}
	}
	return name, nil
}
