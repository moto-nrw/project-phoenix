package platform

import (
	"context"
	"log/slog"
	"strings"

	configModels "github.com/moto-nrw/project-phoenix/models/config"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/services/config"
)

// tenantMailIdentityService resolves the reply address of one school (#1936).
//
// Resolution order, first non-empty wins:
//  1. the email.reply_to_address setting — the school's explicit choice
//  2. platform.schools.email — the contact address the school already
//     maintains in its Stammdaten
//  3. nothing — no Reply-To header is written and the mail behaves as before
//
// The fallback to the contact address is what makes this fix reach the schools
// that never open the setting, which is the whole point: the reported failure
// (17.07.2026) was that answers to Eltern-Einladungen landed at moto.
type tenantMailIdentityService struct {
	schoolRepo platformModels.SchoolRepository
	settings   config.SettingsService
	logger     *slog.Logger
}

// NewTenantMailIdentityService builds the resolver. Both dependencies are
// optional at the seams the callers actually have: without a settings service
// it resolves from the school record alone, which is the correct degraded
// behaviour rather than an error.
func NewTenantMailIdentityService(
	schoolRepo platformModels.SchoolRepository,
	settings config.SettingsService,
	logger *slog.Logger,
) platformModels.TenantMailIdentityResolver {
	if logger == nil {
		logger = slog.Default()
	}
	return &tenantMailIdentityService{
		schoolRepo: schoolRepo,
		settings:   settings,
		logger:     logger.With("component", "tenant_mail_identity"),
	}
}

func (s *tenantMailIdentityService) ResolveTenantMailIdentity(
	ctx context.Context,
	tenantID int64,
) (platformModels.TenantMailIdentity, error) {
	if tenantID <= 0 {
		return platformModels.TenantMailIdentity{}, nil
	}

	school, err := s.resolveSchool(ctx, tenantID)
	// A missing school is not a reason to fail the mail. Losing the reply
	// address degrades the mail; returning an error would drop it entirely.
	if err != nil {
		s.logger.Warn("school lookup for mail identity failed, sending without reply-to",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
	}

	name := ""
	if school != nil {
		name = strings.TrimSpace(school.Name)
	}

	if addr := s.resolveConfiguredAddress(ctx, tenantID); addr != "" {
		return platformModels.TenantMailIdentity{ReplyToName: name, ReplyToAddress: addr}, nil
	}

	if school != nil {
		if addr := strings.TrimSpace(school.Email); addr != "" {
			return platformModels.TenantMailIdentity{ReplyToName: name, ReplyToAddress: addr}, nil
		}
	}

	return platformModels.TenantMailIdentity{}, nil
}

func (s *tenantMailIdentityService) resolveSchool(
	ctx context.Context,
	tenantID int64,
) (*platformModels.School, error) {
	if s.schoolRepo == nil {
		return nil, nil
	}
	return s.schoolRepo.FindByID(ctx, tenantID)
}

// resolveConfiguredAddress reads the explicit setting. ResolveStringForTenant
// opens its own tenant transaction, which is required here: the outbox worker
// renders mail on a background goroutine with no tenant middleware.
func (s *tenantMailIdentityService) resolveConfiguredAddress(ctx context.Context, tenantID int64) string {
	if s.settings == nil {
		return ""
	}
	value, err := s.settings.ResolveStringForTenant(ctx, tenantID, configModels.KeyEmailReplyToAddress)
	if err != nil {
		s.logger.Warn("reply-to setting lookup failed, falling back to school contact address",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
		return ""
	}
	return strings.TrimSpace(value)
}
