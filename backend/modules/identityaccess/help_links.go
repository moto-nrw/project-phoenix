package identityaccess

import (
	"net/url"
	"strconv"
	"strings"
)

// The welcome mails (#3534) link to the help article that walks the invitee
// through accepting the invitation. The link mirrors buildHelpHref in
// frontend/src/lib/help-topics.ts, so the mail and the app open the same
// article.

// Help roles as /help reads them from ?role=.
const (
	HelpRoleLead      = "lead"
	HelpRoleCaregiver = "caregiver"
	HelpRoleTeacher   = "teacher"
	HelpRoleParent    = "parent"
)

// welcomeHelpTopics is the accept-invitation article of each role. The
// staff article is written for caregivers and leads; teachers and parents
// accept on their own portals and have their own article.
var welcomeHelpTopics = map[string]string{
	HelpRoleLead:      "einladung-annehmen-und-konto-einrichten",
	HelpRoleCaregiver: "einladung-annehmen-und-konto-einrichten",
	HelpRoleTeacher:   "zugang-zu-moto-schule-einrichten",
	HelpRoleParent:    "eltern-konto-einrichten",
}

// StaffHelpRole is the help role of an invited staff account, the one the
// sidebar picks for the same account once it signs in (useHelpHref): the
// school portal's teachers, then the accounts that lead the school, then
// the caregivers.
func StaffHelpRole(schoolPortal, leadsSchool bool) string {
	if schoolPortal {
		return HelpRoleTeacher
	}
	if leadsSchool {
		return HelpRoleLead
	}
	return HelpRoleCaregiver
}

// WelcomeHelpSettings are the school settings the accept-invitation article
// adapts to, already normalized by Settings. A nil value could not be
// resolved and stays out of the link, so /help asks for it instead of
// guessing.
type WelcomeHelpSettings struct {
	NFCEnabled   *bool
	PresenceMode *string
	GroupMode    *string
}

// WelcomeHelpLink is the accept-invitation article of role at origin with
// the school's settings in the parameter format buildHelpHref writes.
func WelcomeHelpLink(origin, role string, settings WelcomeHelpSettings) string {
	query := []string{"role=" + url.QueryEscape(role)}
	if settings.NFCEnabled != nil {
		query = append(query, "nfc_enabled="+strconv.FormatBool(*settings.NFCEnabled))
	}
	if settings.PresenceMode != nil {
		query = append(query, "presence_mode="+*settings.PresenceMode)
	}
	if settings.GroupMode != nil {
		query = append(query, "group_mode="+*settings.GroupMode)
	}
	return WelcomeHelpURL(origin, role, query)
}

// parentInfoPath is the Elterninfo PDF the frontend serves without a login
// (frontend/public/downloads). Its source is the elterninfo page in the
// collaterals repository.
const parentInfoPath = "/downloads/moto-elterninfo.pdf"

// WelcomeHelpURL is the accept-invitation article of role at origin with
// the given query parameters (already escaped). return_to is left out: the
// reader comes from a mail, not from the app.
func WelcomeHelpURL(origin, role string, query []string) string {
	return MailOrigin(origin) + "/help/" + welcomeHelpTopics[role] + "?" + strings.Join(query, "&")
}

// ParentInfoURL is the Elterninfo PDF at origin.
func ParentInfoURL(origin string) string {
	return MailOrigin(origin) + parentInfoPath
}
