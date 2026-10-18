package config

import (
	"context"

	config "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

func (s *settingsService) homeLayoutService() (*HomeLayoutService, error) {
	if s == nil || s.homeLayouts == nil {
		return nil, config.ErrHomeLayoutUnavailable
	}
	return s.homeLayouts, nil
}

// HomeLayout returns the caller's start-page composition plus the school's
// prescription. It is an optional public capability of the settings platform
// so unrelated settings test doubles remain narrow.
func (s *settingsService) HomeLayout(ctx context.Context, tenantID, accountID int64, permissions []string) (any, error) {
	homeLayouts, err := s.homeLayoutService()
	if err != nil {
		return nil, err
	}
	return homeLayouts.View(ctx, tenantID, accountID, permissions)
}

func (s *settingsService) SetHomeLayout(ctx context.Context, tenantID, accountID int64, overrides map[string]bool, order []string, spans map[string]int, cols map[string]int, rows map[string]int, replaceBlocks bool) error {
	homeLayouts, err := s.homeLayoutService()
	if err != nil {
		return err
	}
	if !replaceBlocks {
		return homeLayouts.SetOverridesKeepingBlocks(ctx, tenantID, accountID, overrides)
	}
	// Order, width, column and row arrive apart from the HTTP layer and become
	// the stored arrangement here. A key without a width keeps span 0 and is
	// rejected by the placement validation rather than silently defaulting to
	// some size; a key without a cell lands in the top-left corner.
	blocks := make([]config.HomeBlockPlacement, 0, len(order))
	for _, key := range order {
		blocks = append(blocks, config.HomeBlockPlacement{Key: key, Span: spans[key], Col: cols[key], Row: rows[key]})
	}
	return homeLayouts.SetOverrides(ctx, tenantID, accountID, overrides, blocks)
}

func (s *settingsService) ResetHomeLayout(ctx context.Context, tenantID, accountID int64) error {
	homeLayouts, err := s.homeLayoutService()
	if err != nil {
		return err
	}
	return homeLayouts.ResetOverrides(ctx, tenantID, accountID)
}

func (s *settingsService) SetHomeBlockPolicies(ctx context.Context, tenantID, accountID int64, permissions []string, policies map[string]string) error {
	homeLayouts, err := s.homeLayoutService()
	if err != nil {
		return err
	}
	parsed := make(map[string]config.BlockPolicy, len(policies))
	for key, policy := range policies {
		parsed[key] = config.BlockPolicy(policy)
	}
	return homeLayouts.SetPolicies(ctx, tenantID, accountID, permissions, parsed)
}
