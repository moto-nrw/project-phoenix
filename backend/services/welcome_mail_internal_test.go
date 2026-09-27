package services

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The welcome mail's help role follows the sidebar's help entry (#3534):
// a school-portal invitee is a teacher, a lead is recognized by the admin
// role or the config:manage permission, everyone else is a caregiver. The
// guardian flow always asks for the parent article.
func TestStaffHelpRoleMatchesTheSidebar(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name         string
		schoolPortal bool
		roleName     string
		permissions  []string
		want         string
	}{
		{"admin role", false, "admin", nil, welcomeHelpRoleLead},
		{"admin role, any case", false, " Admin ", nil, welcomeHelpRoleLead},
		{"own lead role", false, "Leitung", []string{"users:read", "config:manage"}, welcomeHelpRoleLead},
		{"admin wildcard", false, "Verwaltung", []string{"admin:*"}, welcomeHelpRoleLead},
		{"caregiver system role", false, "user", []string{"users:read"}, welcomeHelpRoleCaregiver},
		{"own staff role", false, "Honorarkraft", []string{"groups:read"}, welcomeHelpRoleCaregiver},
		{"role without permissions", false, "Praktikant", nil, welcomeHelpRoleCaregiver},
		{"school portal", true, "lehrkraft", nil, welcomeHelpRoleTeacher},
		{"school portal wins over permissions", true, "lehrkraft", []string{"config:manage"}, welcomeHelpRoleTeacher},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, staffHelpRole(tc.schoolPortal, tc.roleName, tc.permissions))
		})
	}
}

type fakeWelcomeSettings struct {
	bools   map[string]bool
	strings map[string]string
	err     error
}

func (f fakeWelcomeSettings) ResolveBool(_ context.Context, key string) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return f.bools[key], nil
}

func (f fakeWelcomeSettings) ResolveString(_ context.Context, key string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.strings[key], nil
}

// The link opens the role's accept-invitation article with the school's
// settings in the parameter format buildHelpHref writes, without return_to.
func TestWelcomeHelpURLCarriesRoleArticleAndSchoolSettings(t *testing.T) {
	t.Parallel()
	settings := fakeWelcomeSettings{
		bools: map[string]bool{"attendance.nfc_enabled": true},
		strings: map[string]string{
			"operations.presence_mode": "binary",
			"operations.group_mode":    "open_care",
		},
	}
	for _, tc := range []struct {
		role string
		want string
	}{
		{welcomeHelpRoleLead, "https://moto.test/help/einladung-annehmen-und-konto-einrichten?role=lead&nfc_enabled=true&presence_mode=binary&group_mode=open_care"},
		{welcomeHelpRoleCaregiver, "https://moto.test/help/einladung-annehmen-und-konto-einrichten?role=caregiver&nfc_enabled=true&presence_mode=binary&group_mode=open_care"},
		{welcomeHelpRoleTeacher, "https://moto.test/help/zugang-zu-moto-schule-einrichten?role=teacher&nfc_enabled=true&presence_mode=binary&group_mode=open_care"},
		{welcomeHelpRoleParent, "https://moto.test/help/eltern-konto-einrichten?role=parent&nfc_enabled=true&presence_mode=binary&group_mode=open_care"},
	} {
		t.Run(tc.role, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, welcomeHelpURL(context.Background(), "https://moto.test/", tc.role, 7, settings, nil))
		})
	}
}

// Unknown setting values fall back the way the app's help entry does.
func TestWelcomeHelpURLNormalizesUnknownModes(t *testing.T) {
	t.Parallel()
	settings := fakeWelcomeSettings{strings: map[string]string{
		"operations.presence_mode": "",
		"operations.group_mode":    "something-else",
	}}
	assert.Equal(t,
		"https://moto.test/help/eltern-konto-einrichten?role=parent&nfc_enabled=false&presence_mode=detailed&group_mode=fixed_groups",
		welcomeHelpURL(context.Background(), "https://moto.test", welcomeHelpRoleParent, 7, settings, nil))
}

// A setting that cannot be resolved stays out of the link, so /help asks
// for it instead of showing the wrong variant.
func TestWelcomeHelpURLLeavesUnresolvedSettingsOut(t *testing.T) {
	t.Parallel()
	failing := fakeWelcomeSettings{err: errors.New("database unavailable")}
	assert.Equal(t, "https://moto.test/help/einladung-annehmen-und-konto-einrichten?role=lead",
		welcomeHelpURL(context.Background(), "https://moto.test", welcomeHelpRoleLead, 7, failing, nil))
	assert.Equal(t, "http://localhost:3000/help/einladung-annehmen-und-konto-einrichten?role=caregiver",
		welcomeHelpURL(context.Background(), "", welcomeHelpRoleCaregiver, 7, nil, nil),
		"without settings and without a configured portal the link still opens the article locally")
}
