package owners

import (
	"context"
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

const (
	welcomeHelpRoleParent = identityaccess.HelpRoleParent
)

// staffHelpRole binds Identity & Access's help role of an invited staff
// account to Security Runtime's school-lead rule.
func staffHelpRole(schoolPortal bool, roleName string, rolePermissions []string) string {
	return identityaccess.StaffHelpRole(schoolPortal, securityruntime.LeadsSchool(roleName, rolePermissions))
}

// welcomeHelpSettings resolves the three tenant settings that change the
// help instructions for the school in context: tenant override, then
// registry default. The guardian flows resolve inside their request
// transaction; the staff welcome is queued in the invitation transaction
// through schoolSettings.
type welcomeHelpSettings interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveString(ctx context.Context, key string) (string, error)
}

// tenantSettingsResolver resolves a setting for an explicit school on the
// caller's already-open transaction.
type tenantSettingsResolver interface {
	ResolveBoolForTenantInTx(ctx context.Context, tenantID int64, key string) (bool, error)
	ResolveStringForTenantInTx(ctx context.Context, tenantID int64, key string) (string, error)
}

// schoolSettings binds a tenant settings resolver to one school while the
// invitation transaction is open.
type schoolSettings struct {
	resolver tenantSettingsResolver
	tenantID int64
}

func (s schoolSettings) ResolveBool(ctx context.Context, key string) (bool, error) {
	return s.resolver.ResolveBoolForTenantInTx(ctx, s.tenantID, key)
}

func (s schoolSettings) ResolveString(ctx context.Context, key string) (string, error) {
	return s.resolver.ResolveStringForTenantInTx(ctx, s.tenantID, key)
}

// welcomeHelpURL resolves the school settings Identity & Access's help link
// adapts to. A setting that cannot be resolved is logged and passed as
// unresolved; the error joins those failures, and the link is complete
// without them.
func welcomeHelpURL(ctx context.Context, origin, role string, tenantID int64, settings welcomeHelpSettings, logger *slog.Logger) (string, error) {
	var resolved identityaccess.WelcomeHelpSettings
	var failures []error
	fail := func(key string, err error) {
		logWelcomeSettingFailure(logger, tenantID, key, err)
		failures = append(failures, err)
	}
	if settings != nil {
		if nfc, err := settings.ResolveBool(ctx, configModels.KeyModuleNFCAnwesenheit); err != nil {
			fail(configModels.KeyModuleNFCAnwesenheit, err)
		} else {
			resolved.NFCEnabled = &nfc
		}
		if mode, err := settings.ResolveString(ctx, configModels.KeyPresenceMode); err != nil {
			fail(configModels.KeyPresenceMode, err)
		} else {
			presence := configModels.PresenceModeOf(mode)
			resolved.PresenceMode = &presence
		}
		if mode, err := settings.ResolveString(ctx, configModels.KeyGroupMode); err != nil {
			fail(configModels.KeyGroupMode, err)
		} else {
			group := configModels.GroupModeOf(mode)
			resolved.GroupMode = &group
		}
	}
	return identityaccess.WelcomeHelpLink(origin, role, resolved), errors.Join(failures...)
}

func logWelcomeSettingFailure(logger *slog.Logger, tenantID int64, key string, err error) {
	if logger == nil {
		logger = slog.Default()
	}
	logger.Warn("welcome mail: setting unresolved, help link leaves it out",
		slog.Int64("tenant_id", tenantID),
		slog.String("key", key),
		slog.String("error", err.Error()))
}

const (
	welcomeHelpRoleLead      = identityaccess.HelpRoleLead
	welcomeHelpRoleCaregiver = identityaccess.HelpRoleCaregiver
	welcomeHelpRoleTeacher   = identityaccess.HelpRoleTeacher
)
