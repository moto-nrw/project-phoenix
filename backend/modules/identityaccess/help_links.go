package identityaccess

import "strings"

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
