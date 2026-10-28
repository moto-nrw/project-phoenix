package communication

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

// ParentNotesSettingKey switches the school's parent/OGS messaging on.
const ParentNotesSettingKey = "operations.parent_notes_enabled"

// Conversation audiences (#5200). Every conversation is a team conversation
// unless the guardian or a leadership member chose the leadership.
const (
	ParentMessageAudienceTeam       = "team"
	ParentMessageAudienceLeadership = "leadership"
)

// ParentMessageStaffReach is which conversations a staff reader lists, counts
// and opens: AllStudents opens every child of the school (nothing without
// it), Leadership adds the conversations parents addressed to the leadership
// only. Without Leadership those conversations do not exist for the reader.
type ParentMessageStaffReach struct {
	AllStudents bool
	Leadership  bool
}

// NormalizeParentMessageAudience maps an empty audience to the team, keeping
// clients that do not send one on the team conversation. It reports false for
// any other value.
func NormalizeParentMessageAudience(audience string) (string, bool) {
	switch audience {
	case "", ParentMessageAudienceTeam:
		return ParentMessageAudienceTeam, true
	case ParentMessageAudienceLeadership:
		return ParentMessageAudienceLeadership, true
	default:
		return "", false
	}
}

// ParentMessagingSettings resolves the flag outside a tenant transaction.
type ParentMessagingSettings interface {
	ResolveBoolForTenant(context.Context, int64, string) (bool, error)
}

// ParentMessagingEnabledForTenant applies the shared fail-open feature gate.
// This is not authorization: relationship-level permissions remain mandatory.
func ParentMessagingEnabledForTenant(ctx context.Context, settings ParentMessagingSettings, tenantID int64, logger *slog.Logger) bool {
	if logger == nil {
		logger = slog.Default()
	}
	enabled, err := settings.ResolveBoolForTenant(ctx, tenantID, ParentNotesSettingKey)
	if err != nil {
		logger.Warn("parent messaging: resolve enabled for tenant failed, failing open (counting as enabled)",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
		return true
	}
	return enabled
}

// School-side reader kinds of a conversation (#5200).
const (
	// ParentMessageReadersAllStaff: every staff member who reads the child.
	ParentMessageReadersAllStaff = "all_staff"
	// ParentMessageReadersLeadership: only the leadership persons.
	ParentMessageReadersLeadership = "leadership"
)

// ParentMessageSchoolReadersKind names who reads a conversation of the
// audience on the school side.
func ParentMessageSchoolReadersKind(audience string) string {
	if audience == ParentMessageAudienceLeadership {
		return ParentMessageReadersLeadership
	}
	return ParentMessageReadersAllStaff
}

// ParentMessageLeader is one person who reads the leadership conversations:
// a live staff member of the school whose account holds
// parent_messages:leadership, directly through a role or through admin:*.
type ParentMessageLeader struct {
	AccountID int64
	FirstName string
	LastName  string
}

// ShortName is the name parents and staff see for the person: first name and
// the initial of the last name ("Swantje M."). A missing last name leaves the
// first name; a missing first name the last name.
func (l ParentMessageLeader) ShortName() string {
	first := strings.TrimSpace(l.FirstName)
	last := []rune(strings.TrimSpace(l.LastName))
	switch {
	case first == "":
		return string(last)
	case len(last) == 0:
		return first
	default:
		return first + " " + string(last[0]) + "."
	}
}

// ParentMessageLeaderDirectory lists the leadership persons of the school in
// context. The composition root binds it over the school's staff and their
// role permissions; the order is stable (by name).
type ParentMessageLeaderDirectory interface {
	ParentMessageLeaders(context.Context) ([]ParentMessageLeader, error)
}

// ParentMessageUnnamedLeaderLabel names a leadership account without a
// person behind it, such as the school's administration account (#5200).
const ParentMessageUnnamedLeaderLabel = "Schulverwaltung"

// ParentMessageLeaderShortNames returns the short names of the leaders, one
// per reader, so the list matches who reads. Accounts without a name appear
// once as ParentMessageUnnamedLeaderLabel.
func ParentMessageLeaderShortNames(leaders []ParentMessageLeader) []string {
	names := make([]string, 0, len(leaders))
	unnamed := false
	for _, leader := range leaders {
		name := leader.ShortName()
		if name == "" {
			if unnamed {
				continue
			}
			unnamed, name = true, ParentMessageUnnamedLeaderLabel
		}
		names = append(names, name)
	}
	return names
}

// ParentMessageReaders says who can read one conversation: the one guardian
// it belongs to (other guardians of the child do not read it) and, on the
// school side, every staff member or only the leadership.
type ParentMessageReaders struct {
	Guardian ParentMessageGuardianReader
	School   ParentMessageSchoolReaders
}

// ParentMessageGuardianReader is the guardian a conversation belongs to.
type ParentMessageGuardianReader struct {
	AccountID        int64
	Name             string
	RelationshipType string
}

// ParentMessageSchoolReaders is the school side of a conversation. Names
// carries the leadership persons' short names for Kind leadership and is
// empty for all_staff.
type ParentMessageSchoolReaders struct {
	Kind  string
	Names []string
}

// ParentMessageLeadershipSetting resolves the school's leadership switch in
// the caller's tenant transaction.
type ParentMessageLeadershipSetting interface {
	ParentMessageLeadershipEnabled(context.Context) (bool, error)
}

// ParentMessageLeadershipChoice is the leadership option of the school in
// context: whether the school switched it on and who the leadership persons
// are.
type ParentMessageLeadershipChoice struct {
	Enabled bool
	Leaders []ParentMessageLeader
}

// Available reports whether a new leadership conversation may begin: the
// school offers the choice and at least one account holds the permission.
func (c ParentMessageLeadershipChoice) Available() bool {
	return c.Enabled && len(c.Leaders) > 0
}

// ShortNames returns the leadership persons' short names.
func (c ParentMessageLeadershipChoice) ShortNames() []string {
	return ParentMessageLeaderShortNames(c.Leaders)
}

// ResolveParentMessageLeadershipChoice reads the switch and the leadership
// persons of the school in context. While the switch is off the persons are
// listed only with namesWhenOff, for an existing leadership conversation that
// stays readable and names its readers; otherwise nobody needs them. Both
// errors propagate: the caller decides to refuse rather than to offer or hide
// the choice on a failed read.
func ResolveParentMessageLeadershipChoice(ctx context.Context, setting ParentMessageLeadershipSetting, leaders ParentMessageLeaderDirectory, namesWhenOff bool) (ParentMessageLeadershipChoice, error) {
	enabled, err := setting.ParentMessageLeadershipEnabled(ctx)
	if err != nil {
		return ParentMessageLeadershipChoice{}, fmt.Errorf("resolve parent message leadership setting: %w", err)
	}
	if !enabled && !namesWhenOff {
		return ParentMessageLeadershipChoice{}, nil
	}
	persons, err := leaders.ParentMessageLeaders(ctx)
	if err != nil {
		return ParentMessageLeadershipChoice{}, fmt.Errorf("list parent message leadership persons: %w", err)
	}
	return ParentMessageLeadershipChoice{Enabled: enabled, Leaders: persons}, nil
}
