// Notification type catalogue.
//
// Every notification a person can agree to is declared here once: its key, the
// German label and description the profile page shows, the group it appears
// under, which portal it belongs to, and the tenant setting (if any) that can
// switch it off school-wide.
//
// The catalogue lives in Go rather than in the database so adding a type costs
// no migration, and so the labels sit next to the code that produces the
// notification. It mirrors the settings registry (models/config/registry.go):
// a mutex-guarded map filled from init(), panicking on a duplicate key so a
// collision surfaces at process start rather than at runtime.
package notifications

import (
	"sort"
	"sync"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
)

// Portals a notification type can belong to.
const (
	PortalStaff  = "staff"
	PortalParent = "parent"
)

// Groups the profile page renders as headings, in this order.
const (
	GroupPickup       = "abholung"
	GroupActivities   = "aktivitaeten"
	GroupChildren     = "kinder"
	GroupMessages     = "mitteilungen"
	GroupAppointments = "termine"
)

// groupOrder fixes the order of the headings independently of registration
// order, so the profile page never reshuffles between releases.
var groupOrder = map[string]int{
	GroupPickup:       1,
	GroupActivities:   2,
	GroupChildren:     3,
	GroupMessages:     4,
	GroupAppointments: 5,
}

// Notification type keys.
//
// The four reminder keys are the same strings services/reminders emits as
// Reminder.Type and the frontend switches on. They are declared separately
// rather than imported to keep this package free of a dependency on reminders;
// TestNotificationTypeKeysMatchReminders pins them against each other.
const (
	TypePickupUpcoming         = "pickup_upcoming"
	TypePickupOverdue          = "pickup_overdue"
	TypeActivityStart          = "activity_start"
	TypeActivityOverdue        = "activity_overdue"
	TypeMyActivityStarting     = "my_activity_starting"
	TypeStudentAbsenceReported = "student_absence_reported"
	TypeStaffParentMessage     = "staff_parent_message"
	// TypeStaffMessage is a message from a colleague in the OGS-internal chat
	// (#2598) — staff to staff, no parent involved.
	TypeStaffMessage       = "staff_message"
	TypeParentAnnouncement = "parent_announcement"

	// Parent-facing types added with issue #1671. Each one mirrors a channel the
	// parents app already has, and none of them carries a child's name: the copy
	// says what happened and the deep link leads to the authenticated screen.
	TypeParentMessage             = "parent_message"
	TypeParentAppointment         = "parent_appointment"
	TypeParentAppointmentReminder = "parent_appointment_reminder"
	TypeParentRequestDecided      = "parent_request_decided"
	// TypeParentCareCancelled announces that a care block one of the guardian's
	// children was booked into has been cancelled (#2601). The feed entry is
	// a system-authored parent announcement; this type only governs the push.
	TypeParentCareCancelled = "parent_care_cancelled"
)

// TypeDefinition describes one notification a person can agree to.
type TypeDefinition struct {
	Key         string
	Label       string // German, shown as the row title
	Description string // German, shown under the title
	Group       string
	Portal      string
	// TenantGate is the settings key a school can use to switch this type off
	// for everyone. Empty means the type has no school-wide gate, because
	// whether a person wants to hear about their own shift is nobody else's
	// decision.
	TenantGate string
	SortOrder  int
}

var (
	typeMu       sync.RWMutex
	typeRegistry = make(map[string]TypeDefinition)
)

// RegisterType adds a definition to the catalogue. It panics on a duplicate
// key or an incomplete definition: both are programming errors that must not
// survive process start.
func RegisterType(def TypeDefinition) {
	if def.Key == "" || def.Label == "" || def.Group == "" || def.Portal == "" {
		panic("notifications: incomplete type definition for key " + def.Key)
	}
	if def.Portal != PortalStaff && def.Portal != PortalParent {
		panic("notifications: unknown portal " + def.Portal + " for key " + def.Key)
	}
	if _, ok := groupOrder[def.Group]; !ok {
		panic("notifications: unknown group " + def.Group + " for key " + def.Key)
	}

	typeMu.Lock()
	defer typeMu.Unlock()
	if _, exists := typeRegistry[def.Key]; exists {
		panic("notifications: duplicate type key " + def.Key)
	}
	typeRegistry[def.Key] = def
}

// GetType returns one definition.
func GetType(key string) (TypeDefinition, bool) {
	typeMu.RLock()
	defer typeMu.RUnlock()
	def, ok := typeRegistry[key]
	return def, ok
}

// TypesForPortal returns the catalogue of one portal, ordered by group and
// then by SortOrder.
func TypesForPortal(portal string) []TypeDefinition {
	typeMu.RLock()
	defs := make([]TypeDefinition, 0, len(typeRegistry))
	for _, def := range typeRegistry {
		if def.Portal == portal {
			defs = append(defs, def)
		}
	}
	typeMu.RUnlock()

	sort.Slice(defs, func(i, j int) bool {
		if groupOrder[defs[i].Group] != groupOrder[defs[j].Group] {
			return groupOrder[defs[i].Group] < groupOrder[defs[j].Group]
		}
		if defs[i].SortOrder != defs[j].SortOrder {
			return defs[i].SortOrder < defs[j].SortOrder
		}
		return defs[i].Key < defs[j].Key
	})
	return defs
}

func init() {
	RegisterType(TypeDefinition{
		Key:         TypePickupUpcoming,
		Label:       "Anstehende Abholung",
		Description: "Kurz bevor ein Kind abgeholt wird, das Sie gerade betreuen.",
		Group:       GroupPickup,
		Portal:      PortalStaff,
		TenantGate:  configModel.KeyRemindersPickupUpcomingEnabled,
		SortOrder:   1,
	})
	RegisterType(TypeDefinition{
		Key:         TypePickupOverdue,
		Label:       "Überfällige Abholung",
		Description: "Wenn ein Kind nach seiner Abholzeit noch da ist.",
		Group:       GroupPickup,
		Portal:      PortalStaff,
		TenantGate:  configModel.KeyRemindersPickupOverdueEnabled,
		SortOrder:   2,
	})

	// The two activity types read almost alike, so their labels name the
	// relation rather than the event: one is about the room you are watching,
	// the other about the slot you are assigned to.
	RegisterType(TypeDefinition{
		Key:         TypeActivityStart,
		Label:       "Aktivität in Ihrem Raum",
		Description: "Kurz bevor eine geplante Aktivität in einem Raum startet, den Sie gerade beaufsichtigen.",
		Group:       GroupActivities,
		Portal:      PortalStaff,
		TenantGate:  configModel.KeyRemindersActivityStartEnabled,
		SortOrder:   1,
	})
	RegisterType(TypeDefinition{
		Key:         TypeActivityOverdue,
		Label:       "Aktivität nicht gestartet",
		Description: "Wenn eine geplante Aktivität überfällig ist und noch niemand sie gestartet hat.",
		Group:       GroupActivities,
		Portal:      PortalStaff,
		TenantGate:  configModel.KeyRemindersActivityOverdueEnabled,
		SortOrder:   2,
	})
	RegisterType(TypeDefinition{
		Key:         TypeMyActivityStarting,
		Label:       "Ihr nächster Einsatz",
		Description: "Kurz bevor ein Einsatz aus Ihrem Betreuungsplan beginnt, auch wenn Sie gerade keine Aufsicht führen.",
		Group:       GroupActivities,
		Portal:      PortalStaff,
		SortOrder:   3,
	})

	RegisterType(TypeDefinition{
		Key:         TypeStudentAbsenceReported,
		Label:       "Krankmeldung eines Kindes",
		Description: "Wenn für ein Kind aus Ihren Gruppen eine Krankmeldung oder Entschuldigung für heute eingetragen wird.",
		Group:       GroupChildren,
		Portal:      PortalStaff,
		TenantGate:  configModel.KeyNotificationsAbsenceReportedEnabled,
		SortOrder:   1,
	})
	RegisterType(TypeDefinition{
		Key:         TypeStaffParentMessage,
		Label:       "Nachrichten von Eltern",
		Description: "Wenn Eltern der OGS eine neue Nachricht schreiben.",
		Group:       GroupMessages,
		Portal:      PortalStaff,
		TenantGate:  configModel.KeyParentNotesEnabled,
		SortOrder:   1,
	})

	// OGS-internal colleague chat (#2598). Gated on the school's own feature
	// switch: a school that has the internal messenger off has no conversation
	// to announce, so the type disappears from the preference catalogue too.
	RegisterType(TypeDefinition{
		Key:         TypeStaffMessage,
		Label:       "Nachrichten von Kolleginnen und Kollegen",
		Description: "Wenn Ihnen jemand aus dem Team eine Nachricht im Team-Chat schreibt.",
		Group:       GroupMessages,
		Portal:      PortalStaff,
		TenantGate:  configModel.KeyStaffMessagingEnabled,
		SortOrder:   2,
	})

	RegisterType(TypeDefinition{
		Key:         TypeParentAnnouncement,
		Label:       "Neue Mitteilung der Schule",
		Description: "Wenn die Schule eine neue Mitteilung im Elternportal veröffentlicht.",
		Group:       GroupMessages,
		Portal:      PortalParent,
		SortOrder:   1,
	})

	// The gate is the messaging feature flag itself: a school with parent
	// messaging switched off has no conversation to announce. Note the direction
	// differs from parentmessaging.MessagingEnabled, which fails OPEN on a
	// settings blip so the inbox never half-disables. Here a blip fails the whole
	// FilterOptedIn call and the producer skips the push — the message is already
	// safely stored and visible; a missing push is the harmless side.
	RegisterType(TypeDefinition{
		Key:         TypeParentMessage,
		Label:       "Neue Nachricht der OGS",
		Description: "Wenn die OGS Ihnen zu einem Kind schreibt.",
		Group:       GroupMessages,
		Portal:      PortalParent,
		TenantGate:  configModel.KeyParentNotesEnabled,
		SortOrder:   2,
	})

	// No tenant gate: whether guardians hear about an appointment is decided per
	// appointment by its organizer ("Eltern benachrichtigen"), not school-wide.
	RegisterType(TypeDefinition{
		Key:         TypeParentAppointment,
		Label:       "Neuer oder geänderter Termin",
		Description: "Wenn ein Termin für Sie angelegt, geändert oder abgesagt wird.",
		Group:       GroupAppointments,
		Portal:      PortalParent,
		SortOrder:   1,
	})

	RegisterType(TypeDefinition{
		Key:         TypeParentAppointmentReminder,
		Label:       "Terminerinnerung",
		Description: "Kurz vor einem Termin, zu dem Sie eingeladen sind.",
		Group:       GroupAppointments,
		Portal:      PortalParent,
		TenantGate:  configModel.KeyCalendarAppointmentReminderEnabled,
		SortOrder:   2,
	})

	// Gated by the school-wide cancellation-notice switch, never by the news
	// flag: the feed entry exists whenever the notice was sent, this only
	// decides whether the phone rings for it.
	RegisterType(TypeDefinition{
		Key:         TypeParentCareCancelled,
		Label:       "Betreuung fällt aus",
		Description: "Wenn die OGS einen Betreuungstermin Ihres Kindes absagt.",
		Group:       GroupAppointments,
		Portal:      PortalParent,
		TenantGate:  configModel.KeyNotificationsCareCancelledEnabled,
		SortOrder:   3,
	})

	RegisterType(TypeDefinition{
		Key:         TypeParentRequestDecided,
		Label:       "Entscheidung über Ihre Anfrage",
		Description: "Wenn die OGS über eine Ihrer Anfragen entschieden hat, etwa zu Betreuungszeiten, Stammdaten oder einer Abmeldung.",
		Group:       GroupChildren,
		Portal:      PortalParent,
		TenantGate:  configModel.KeyParentNotesEnabled,
		SortOrder:   1,
	})
}
