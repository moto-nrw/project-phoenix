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

	"github.com/moto-nrw/project-phoenix/modules/delivery/contract"
)

// Portals a notification type can belong to.
const (
	PortalStaff  = "staff"
	PortalParent = "parent"
	// PortalSchool is the Lehrkraft portal "moto schule" (#2208). It borrows
	// from the staff catalogue: a staff type opts into it with SchoolPortal,
	// and the decision is stored on the same (account, type) row as in the
	// OGS portal — the same person hears about the same conversation once.
	// A type registered with PortalSchool itself exists only there (#4792):
	// the OGS portal neither offers it nor delivers it.
	PortalSchool = "school"
)

// Channels a notification type is delivered over.
const (
	// ChannelPush is the default: push and in-app hints, decided on the
	// profile page. An empty Channel means push.
	ChannelPush = "push"
	// ChannelEmail is an opt-in e-mail to staff (#3780). It never appears in
	// a portal catalogue: the person decides it where its event happens, and
	// only while holding the type's Permission.
	ChannelEmail = "email"
)

// permissionUsersDelete restates the permission registry's name, which this
// package may not import: the exit notices go to whoever may plan an exit
// (#4809). TestStaffPlannedExitNoticeFollowsTheRight pins it to the registry.
const permissionUsersDelete = "users:delete"

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
// The four reminder keys are the same strings the reminder query emits as
// Reminder.Type and the frontend switches on. They are declared separately
// rather than imported to keep this package free of a dependency on reminders;
// TestNotificationTypeKeysMatchReminders pins them against each other.
const (
	TypePickupUpcoming         = "pickup_upcoming"
	TypePickupOverdue          = "pickup_overdue"
	TypeActivityStart          = "activity_start"
	TypeActivityOverdue        = "activity_overdue"
	TypeActivityRunningOver    = "activity_running_over"
	TypeMyActivityStarting     = "my_activity_starting"
	TypeStudentAbsenceReported = "student_absence_reported"
	TypeStaffParentMessage     = "staff_parent_message"
	// TypeStaffMessage is a message from a colleague in the OGS-internal chat
	// (#2598) — staff to staff, no parent involved.
	TypeStaffMessage = "staff_message"
	// TypeStaffSurvey announces a new or still open staff survey (#4781).
	TypeStaffSurvey = "staff_survey"
	// TypeStaffPlannedExit reminds the Leitung of a planned staff exit and
	// tells it when the exit is blocked (#4809).
	TypeStaffPlannedExit = contract.NotificationStaffPlannedExit
	// TypeSchoolClassAbsenceReported tells a Lehrkraft or the
	// Schulverwaltung in moto schule that a child of one of their classes was
	// reported sick or excused for today (#4792).
	TypeSchoolClassAbsenceReported = "school_class_absence_reported"
	// TypeSchoolParentConferenceBooking tells a Lehrkraft in moto schule that
	// a family booked, moved or cancelled a slot at her Elternsprechtag
	// (#4801). The calendar produces it under the same key.
	TypeSchoolParentConferenceBooking = "school_parent_conference_booking"
	// TypeSchoolUnreportedAbsence tells the Schulverwaltung in moto schule
	// how many children the Lehrkräfte marked absent this morning without a
	// report (#4791). It names no child.
	TypeSchoolUnreportedAbsence = "school_unreported_absence"

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
	// TypeParentBillingDocument announces a document of the Träger in the
	// inbox of the parents portal (#4671), such as a confirmation letter.
	TypeParentBillingDocument = "parent_billing_document"

	// TypeEnrollmentSubmitted is the e-mail "Neue Anmeldung" (#3780) to the
	// enrollment managers who switched it on in the Anmeldungen overview.
	TypeEnrollmentSubmitted = "enrollment_submitted"
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
	// SchoolPortal offers a PortalStaff type in the school portal as well
	// (#2208). Only types a Lehrkraft can actually receive belong here; the
	// OGS supervision reminders never address a school-only account.
	SchoolPortal bool
	// Channel is ChannelPush (or empty) or ChannelEmail.
	Channel string
	// Permission is the right an e-mail type's recipient must hold when the
	// mail is sent. Losing it stops the mail although the decision stays on.
	// A push type with a Permission is offered only to its holders (#4791).
	Permission string
	// DefaultOn makes a decision nobody stored read as on: the type is on
	// until the person switches it off, and delivery drops only explicit
	// declines (FilterNotOptedOut). It needs a Permission, so it is on only
	// for the people whose task the notice is (#4791).
	DefaultOn bool
}

var (
	typeMu       sync.RWMutex
	typeRegistry = make(map[string]TypeDefinition)
)

// RegisterType adds a definition to the catalogue. It panics on a duplicate
// key or an incomplete definition: both are programming errors that must not
// survive process start.
func RegisterType(def TypeDefinition) {
	if def.Key == "" || def.Label == "" || def.Portal == "" {
		panic("notifications: incomplete type definition for key " + def.Key)
	}
	switch def.Portal {
	case PortalStaff, PortalParent:
	case PortalSchool:
		// A school-only type is already offered there; SchoolPortal only
		// widens a staff type.
		if def.SchoolPortal {
			panic("notifications: school type " + def.Key + " must not set SchoolPortal")
		}
	default:
		panic("notifications: unknown portal " + def.Portal + " for key " + def.Key)
	}
	switch def.Channel {
	case "", ChannelPush:
		if _, ok := groupOrder[def.Group]; !ok {
			panic("notifications: unknown group " + def.Group + " for key " + def.Key)
		}
		if def.DefaultOn && def.Permission == "" {
			panic("notifications: default-on type " + def.Key + " needs a permission")
		}
	case ChannelEmail:
		// An e-mail type is addressed by permission, not by a relation to a
		// child or a room, so it cannot exist without one. It has no profile
		// heading and no school-wide gate: EmailSubscriptions applies neither.
		if def.Portal != PortalStaff || def.Permission == "" || def.TenantGate != "" || def.SchoolPortal {
			panic("notifications: e-mail type " + def.Key + " needs the staff portal, a permission and no gate")
		}
	default:
		panic("notifications: unknown channel " + def.Channel + " for key " + def.Key)
	}

	typeMu.Lock()
	defer typeMu.Unlock()
	if _, exists := typeRegistry[def.Key]; exists {
		panic("notifications: duplicate type key " + def.Key)
	}
	typeRegistry[def.Key] = def
}

// OfferedInPortal reports whether a person using the given portal may see
// and decide this type. The school portal borrows from the staff catalogue
// and adds its own school-only types.
func OfferedInPortal(def TypeDefinition, portal string) bool {
	if def.Channel == ChannelEmail {
		return false
	}
	if portal == PortalSchool {
		return def.Portal == PortalSchool || (def.Portal == PortalStaff && def.SchoolPortal)
	}
	return def.Portal == portal
}

// GetType returns one definition.
func GetType(key string) (TypeDefinition, bool) {
	typeMu.RLock()
	defer typeMu.RUnlock()
	def, ok := typeRegistry[key]
	return def, ok
}

// EmailType returns the definition of an e-mail type. It reports false for
// unknown keys and for push types.
func EmailType(key string) (TypeDefinition, bool) {
	def, ok := GetType(key)
	if !ok || def.Channel != ChannelEmail {
		return TypeDefinition{}, false
	}
	return def, true
}

// TypesForPortal returns the catalogue of one portal, ordered by group and
// then by SortOrder.
func TypesForPortal(portal string) []TypeDefinition {
	typeMu.RLock()
	defs := make([]TypeDefinition, 0, len(typeRegistry))
	for _, def := range typeRegistry {
		if OfferedInPortal(def, portal) {
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
	registerPickupTypes()
	registerActivityTypes()
	registerStaffMessageTypes()
	registerParentMessageTypes()
	registerParentStatusTypes()
	registerSchoolTypes()
}

func registerPickupTypes() {
	RegisterType(TypeDefinition{
		Key:         TypePickupUpcoming,
		Label:       "Anstehende Abholung",
		Description: "Kurz bevor ein Kind abgeholt wird, das Sie gerade betreuen.",
		Group:       GroupPickup,
		Portal:      PortalStaff,
		TenantGate:  settingRemindersPickupUpcomingEnabled,
		SortOrder:   1,
	})
	RegisterType(TypeDefinition{
		Key:         TypePickupOverdue,
		Label:       "Überfällige Abholung",
		Description: "Wenn ein Kind nach seiner Abholzeit noch da ist.",
		Group:       GroupPickup,
		Portal:      PortalStaff,
		TenantGate:  settingRemindersPickupOverdueEnabled,
		SortOrder:   2,
	})
}

func registerActivityTypes() {

	// The two activity types read almost alike, so their labels name the
	// relation rather than the event: one is about the room you are watching,
	// the other about the slot you are assigned to.
	RegisterType(TypeDefinition{
		Key:         TypeActivityStart,
		Label:       "Aktivität in Ihrem Raum",
		Description: "Kurz bevor eine geplante Aktivität in einem Raum startet, den Sie gerade beaufsichtigen.",
		Group:       GroupActivities,
		Portal:      PortalStaff,
		TenantGate:  settingRemindersActivityStartEnabled,
		SortOrder:   1,
	})
	RegisterType(TypeDefinition{
		Key:         TypeActivityOverdue,
		Label:       "Aktivität nicht gestartet",
		Description: "Wenn eine geplante Aktivität überfällig ist und noch niemand sie gestartet hat.",
		Group:       GroupActivities,
		Portal:      PortalStaff,
		TenantGate:  settingRemindersActivityOverdueEnabled,
		SortOrder:   2,
	})
	RegisterType(TypeDefinition{
		Key:         TypeActivityRunningOver,
		Label:       "AG dauert länger",
		Description: "Wenn eine gestartete Aktivität in einem Raum Ihrer Aufsicht nach dem geplanten Ende noch läuft.",
		Group:       GroupActivities,
		Portal:      PortalStaff,
		TenantGate:  settingRemindersActivityRunningOverEnabled,
		SortOrder:   3,
	})
	RegisterType(TypeDefinition{
		Key:         TypeMyActivityStarting,
		Label:       "Ihr nächster Einsatz",
		Description: "Kurz bevor ein Einsatz aus Ihrem Betreuungsplan beginnt, auch wenn Sie gerade keine Aufsicht führen.",
		Group:       GroupActivities,
		Portal:      PortalStaff,
		SortOrder:   4,
	})
}

func registerStaffMessageTypes() {

	RegisterType(TypeDefinition{
		Key:         TypeStudentAbsenceReported,
		Label:       "Krankmeldung eines Kindes",
		Description: "Wenn für ein Kind aus Ihren Gruppen eine Krankmeldung oder Entschuldigung für heute eingetragen wird.",
		Group:       GroupChildren,
		Portal:      PortalStaff,
		TenantGate:  settingAbsenceReportedEnabled,
		SortOrder:   1,
	})
	RegisterType(TypeDefinition{
		Key:         TypeStaffParentMessage,
		Label:       "Nachrichten von Eltern",
		Description: "Wenn Eltern der OGS eine neue Nachricht schreiben.",
		Group:       GroupMessages,
		Portal:      PortalStaff,
		TenantGate:  settingParentNotesEnabled,
		SortOrder:   1,
	})

	// OGS-internal colleague chat (#2598). Gated on the school's own feature
	// switch: a school that has the internal messenger off has no conversation
	// to announce, so the type disappears from the preference catalogue too.
	RegisterType(TypeDefinition{
		Key:          TypeStaffMessage,
		Label:        "Nachrichten von Kolleginnen und Kollegen",
		Description:  "Wenn Ihnen jemand aus dem Team eine Nachricht im Team-Chat schreibt.",
		Group:        GroupMessages,
		Portal:       PortalStaff,
		TenantGate:   settingStaffMessagingEnabled,
		SortOrder:    2,
		SchoolPortal: true,
	})
	// Staff surveys (#4781) are answered in the OGS portal only; the school
	// portal never offers them.
	RegisterType(TypeDefinition{
		Key:         TypeStaffSurvey,
		Label:       "Umfragen der Einrichtung",
		Description: "Wenn die Einrichtung eine Umfrage an das Team veröffentlicht oder an eine offene Umfrage erinnert.",
		Group:       GroupMessages,
		Portal:      PortalStaff,
		SortOrder:   3,
	})

	// The Leitung's task (#4809): on until the person switches it off, and
	// only offered to the holders of users:delete, who plan the exits.
	RegisterType(TypeDefinition{
		Key:         TypeStaffPlannedExit,
		Label:       "Austritte aus dem Team",
		Description: "Eine Woche vor dem letzten Arbeitstag einer Person, am Tag selbst und wenn ihr Austritt warten muss.",
		Group:       GroupMessages,
		Portal:      PortalStaff,
		Permission:  permissionUsersDelete,
		DefaultOn:   true,
		SortOrder:   4,
	})
}

func registerParentMessageTypes() {

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
	// No school gate: the document comes from the Träger, not from a school
	// feature; the family decides here whether the phone rings for it.
	RegisterType(TypeDefinition{
		Key:         TypeParentBillingDocument,
		Label:       "Neues Dokument vom Träger",
		Description: "Wenn der Träger Ihnen ein Dokument schickt, zum Beispiel ein Bestätigungsschreiben.",
		Group:       GroupMessages,
		Portal:      PortalParent,
		SortOrder:   3,
	})

	RegisterType(TypeDefinition{
		Key:         TypeParentMessage,
		Label:       "Neue Nachricht der OGS",
		Description: "Wenn die OGS Ihnen zu einem Kind schreibt.",
		Group:       GroupMessages,
		Portal:      PortalParent,
		TenantGate:  settingParentNotesEnabled,
		SortOrder:   2,
	})
}

func registerParentStatusTypes() {

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
		TenantGate:  settingCalendarAppointmentReminder,
		SortOrder:   2,
	})

	// No school gate and never the news flag: the person cancelling decides
	// in the dialog whether a notice goes out (#3731); the family decides here
	// whether the phone rings for it.
	RegisterType(TypeDefinition{
		Key:         TypeParentCareCancelled,
		Label:       "Betreuung fällt aus",
		Description: "Wenn die OGS einen Betreuungstermin Ihres Kindes absagt.",
		Group:       GroupAppointments,
		Portal:      PortalParent,
		SortOrder:   3,
	})

	RegisterType(TypeDefinition{
		Key:         TypeParentRequestDecided,
		Label:       "Entscheidung über Ihre Anfrage",
		Description: "Wenn die OGS über eine Ihrer Anfragen entschieden hat, etwa zu Betreuungszeiten, Stammdaten oder einer Abmeldung.",
		Group:       GroupChildren,
		Portal:      PortalParent,
		TenantGate:  settingParentNotesEnabled,
		SortOrder:   1,
	})
}

// registerSchoolTypes adds the types only moto schule offers (#4792).
func registerSchoolTypes() {

	// Gated like the OGS sick note notice: a school that switched the
	// absence notification off has decided against announcing health
	// information at all, in either portal.
	RegisterType(TypeDefinition{
		Key:         TypeSchoolClassAbsenceReported,
		Label:       "Krankmeldungen meiner Klassen",
		Description: "Wenn ein Kind aus Ihren Klassen für heute krankgemeldet oder entschuldigt wird.",
		Group:       GroupChildren,
		Portal:      PortalSchool,
		TenantGate:  settingAbsenceReportedEnabled,
		SortOrder:   1,
	})

	// The Schulverwaltung's task (#4791): on until the person switches it
	// off, and only offered to the holders of class_day:absence_report_write,
	// who can act on it. The notice names only a number, never a child.
	RegisterType(TypeDefinition{
		Key:         TypeSchoolUnreportedAbsence,
		Label:       "Fehlt ohne Meldung",
		Description: "Wenn eine Lehrkraft ein Kind als fehlend einträgt und keine Meldung da ist.",
		Group:       GroupChildren,
		Portal:      PortalSchool,
		Permission:  permissionClassDayAbsenceReportWrite,
		DefaultOn:   true,
		SortOrder:   2,
	})

	// No school gate: the Lehrkraft offered the day herself and decides
	// here whether her phone rings for its bookings.
	RegisterType(TypeDefinition{
		Key:         TypeSchoolParentConferenceBooking,
		Label:       "Buchungen am Elternsprechtag",
		Description: "Wenn Eltern einen Termin an Ihrem Elternsprechtag buchen, verschieben oder absagen.",
		Group:       GroupAppointments,
		Portal:      PortalSchool,
		SortOrder:   1,
	})
}
