package services

import (
	"context"
	"log/slog"
	"net/url"
	"strconv"
	"strings"

	configModels "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
)

// The welcome mail (#3534) follows an invitation: a greeting and a link to
// the help article that walks the invitee through accepting it. Both
// audiences share this file: the help link mirrors buildHelpHref in
// frontend/src/lib/help-topics.ts and the role mapping mirrors the sidebar's
// help entry (useHelpHref), so the mail and the app open the same article.

// Help roles as /help reads them from ?role=.
const (
	welcomeHelpRoleLead      = "lead"
	welcomeHelpRoleCaregiver = "caregiver"
	welcomeHelpRoleTeacher   = "teacher"
	welcomeHelpRoleParent    = "parent"
)

// welcomeHelpTopics is the accept-invitation article of each role. The
// staff article is written for caregivers and leads; teachers and parents
// accept on their own portals and have their own article.
var welcomeHelpTopics = map[string]string{
	welcomeHelpRoleLead:      "einladung-annehmen-und-konto-einrichten",
	welcomeHelpRoleCaregiver: "einladung-annehmen-und-konto-einrichten",
	welcomeHelpRoleTeacher:   "zugang-zu-moto-schule-einrichten",
	welcomeHelpRoleParent:    "eltern-konto-einrichten",
}

// parentInfoPath is the Elterninfo PDF the frontend serves without a login
// (frontend/public/downloads). Its source is the elterninfo page in the
// collaterals repository.
const parentInfoPath = "/downloads/moto-elterninfo.pdf"

// staffWelcomeLeadPermission is the permission the frontend's leadsSchool
// reads next to the admin scope: a school may create its own lead role.
const staffWelcomeLeadPermission = "config:manage"

// staffHelpRole maps an invited staff role onto the help role the sidebar
// would pick for the same account once it signs in.
func staffHelpRole(schoolPortal bool, roleName string, rolePermissions []string) string {
	if schoolPortal {
		return welcomeHelpRoleTeacher
	}
	if strings.EqualFold(strings.TrimSpace(roleName), "admin") ||
		securityruntime.HasPermission(staffWelcomeLeadPermission, rolePermissions) {
		return welcomeHelpRoleLead
	}
	return welcomeHelpRoleCaregiver
}

// welcomeHelpSettings resolves the three tenant settings that change the
// help instructions for the school in context: tenant override, then
// registry default. The guardian flows resolve inside their request
// transaction; the staff welcome, sent after commit, binds its school with
// schoolSettings.
type welcomeHelpSettings interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveString(ctx context.Context, key string) (string, error)
}

// tenantSettingsResolver resolves a setting for an explicit school in its
// own transaction.
type tenantSettingsResolver interface {
	ResolveBoolForTenant(ctx context.Context, tenantID int64, key string) (bool, error)
	ResolveStringForTenant(ctx context.Context, tenantID int64, key string) (string, error)
}

// schoolSettings binds a tenant settings resolver to one school, for work
// that runs outside the request transaction.
type schoolSettings struct {
	resolver tenantSettingsResolver
	tenantID int64
}

func (s schoolSettings) ResolveBool(ctx context.Context, key string) (bool, error) {
	return s.resolver.ResolveBoolForTenant(ctx, s.tenantID, key)
}

func (s schoolSettings) ResolveString(ctx context.Context, key string) (string, error) {
	return s.resolver.ResolveStringForTenant(ctx, s.tenantID, key)
}

// welcomeHelpURL builds the help link for role at origin. A setting that
// cannot be resolved stays out of the link, and /help then asks for it
// instead of guessing. return_to is left out: the reader comes from a mail,
// not from the app.
func welcomeHelpURL(ctx context.Context, origin, role string, tenantID int64, settings welcomeHelpSettings, logger *slog.Logger) string {
	query := []string{"role=" + url.QueryEscape(role)}
	if settings != nil {
		if nfc, err := settings.ResolveBool(ctx, configModels.KeyAttendanceNFCEnabled); err == nil {
			query = append(query, "nfc_enabled="+strconv.FormatBool(nfc))
		} else {
			logWelcomeSettingFailure(logger, tenantID, configModels.KeyAttendanceNFCEnabled, err)
		}
		if mode, err := settings.ResolveString(ctx, configModels.KeyPresenceMode); err == nil {
			presence := configModels.PresenceModeDetailed
			if mode == configModels.PresenceModeBinary {
				presence = configModels.PresenceModeBinary
			}
			query = append(query, "presence_mode="+presence)
		} else {
			logWelcomeSettingFailure(logger, tenantID, configModels.KeyPresenceMode, err)
		}
		if mode, err := settings.ResolveString(ctx, configModels.KeyGroupMode); err == nil {
			group := configModels.GroupModeFixedGroups
			if mode == configModels.GroupModeOpenCare {
				group = configModels.GroupModeOpenCare
			}
			query = append(query, "group_mode="+group)
		} else {
			logWelcomeSettingFailure(logger, tenantID, configModels.KeyGroupMode, err)
		}
	}
	return portalOrigin(origin) + "/help/" + welcomeHelpTopics[role] + "?" + strings.Join(query, "&")
}

// portalOrigin trims the configured portal URL; an unset one points at the
// local frontend, like the invitation links.
func portalOrigin(origin string) string {
	origin = strings.TrimRight(strings.TrimSpace(origin), "/")
	if origin == "" {
		return "http://localhost:3000"
	}
	return origin
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
