package api

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"strconv"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// The welcome mail (#3534) follows an invitation: a greeting and a link to
// the help article that walks the invitee through accepting it. Identity &
// Access owns the article per role; the role mapping mirrors the sidebar's
// help entry (useHelpHref), so the mail and the app open the same article.

// Help roles as /help reads them from ?role=.
const (
	welcomeHelpRoleLead      = identityaccess.HelpRoleLead
	welcomeHelpRoleCaregiver = identityaccess.HelpRoleCaregiver
	welcomeHelpRoleTeacher   = identityaccess.HelpRoleTeacher
	welcomeHelpRoleParent    = identityaccess.HelpRoleParent
)

// staffHelpRole maps an invited staff role onto the help role the sidebar
// would pick for the same account once it signs in.
func staffHelpRole(schoolPortal bool, roleName string, rolePermissions []string) string {
	if schoolPortal {
		return welcomeHelpRoleTeacher
	}
	if securityruntime.LeadsSchool(roleName, rolePermissions) {
		return welcomeHelpRoleLead
	}
	return welcomeHelpRoleCaregiver
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

// welcomeHelpURL builds the help link for role at origin. A setting that
// cannot be resolved stays out of the link, and /help then asks for it
// instead of guessing. The error joins the settings that failed to resolve; the
// link is complete without them.
func welcomeHelpURL(ctx context.Context, origin, role string, tenantID int64, settings welcomeHelpSettings, logger *slog.Logger) (string, error) {
	query := []string{"role=" + url.QueryEscape(role)}
	var failures []error
	if settings != nil {
		for _, parameter := range []func() (string, string, error){
			func() (string, string, error) { return welcomeNFCParameter(ctx, settings) },
			func() (string, string, error) { return welcomePresenceParameter(ctx, settings) },
			func() (string, string, error) { return welcomeGroupParameter(ctx, settings) },
		} {
			value, key, err := parameter()
			if err != nil {
				logWelcomeSettingFailure(logger, tenantID, key, err)
				failures = append(failures, err)
				continue
			}
			query = append(query, value)
		}
	}
	return identityaccess.WelcomeHelpURL(origin, role, query), errors.Join(failures...)
}

// welcomeNFCParameter is the help link's nfc_enabled parameter; key names the
// setting it reads.
func welcomeNFCParameter(ctx context.Context, settings welcomeHelpSettings) (value, key string, err error) {
	key = configModels.KeyAttendanceNFCEnabled
	nfc, err := settings.ResolveBool(ctx, key)
	if err != nil {
		return "", key, err
	}
	return "nfc_enabled=" + strconv.FormatBool(nfc), key, nil
}

// welcomePresenceParameter is the help link's presence_mode parameter.
func welcomePresenceParameter(ctx context.Context, settings welcomeHelpSettings) (value, key string, err error) {
	key = configModels.KeyPresenceMode
	mode, err := settings.ResolveString(ctx, key)
	if err != nil {
		return "", key, err
	}
	return "presence_mode=" + configModels.PresenceModeOf(mode), key, nil
}

// welcomeGroupParameter is the help link's group_mode parameter.
func welcomeGroupParameter(ctx context.Context, settings welcomeHelpSettings) (value, key string, err error) {
	key = configModels.KeyGroupMode
	mode, err := settings.ResolveString(ctx, key)
	if err != nil {
		return "", key, err
	}
	return "group_mode=" + configModels.GroupModeOf(mode), key, nil
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
