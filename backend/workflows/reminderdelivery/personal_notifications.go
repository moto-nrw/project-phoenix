// Reminder → notification bridge (epic "persönliche Benachrichtigungen"): the
// producer that turns the #1457 reminder list into notifications for the people
// it actually concerns.
//
// Every minute, per tenant, the Worker's tick asks this producer to find who
// agreed to hear about which kind of reminder, compute those people's personal
// reminder lists in one batch, and dispatch one notice per person and kind.
// Nobody is addressed who did not switch that kind on in their own profile.
//
// The producer belongs to this workflow because it joins the reminder list
// with the notification audience; the Worker only runs the tick and the
// Delivery Platform only delivers. It knows nothing about channels either — it
// builds notices and hands them to Notify, so every channel the notification
// service grows applies here automatically.
//
// GDPR: reminder rows carry student names, the dispatched notices never do.
// Title and body only say how many reminders of which kind are due; the deep
// link leads to /reminders, where the names are shown to the authenticated,
// permission-filtered viewer.
//
// Re-fire guard: an in-memory sync.Map keyed by (tenant, account, reminder
// identity), rotated on civil-date rollover — the same shape as the Worker's
// instance-overdue tick. This is deliberately a single-instance guarantee: two
// active Worker processes have independent maps and may both emit the same
// reminder. Deployments run one Worker under the lease. A mid-day restart, or
// a crash between commit and the after-commit hook that fills the map,
// likewise re-fires currently-due reminders once. Toasts are ephemeral, so
// that cost is acceptable for zero disk state. Push is durable: each notice's
// idempotency key names the occurrences it reports, so the push outbox drops
// the repeat of an identical set (#4605).
package reminderdelivery

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"log/slog"
	"slices"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// TypeMyActivityStarting is the notification kind of an activity the person
// is personally planned on. It is not a reminder type of its own: the
// producer reclassifies an activity_start reminder into it.
const TypeMyActivityStarting = "my_activity_starting"

// Priorities and audience scope of a personal reminder notice, in the
// vocabulary of the Delivery Platform's notification events.
const (
	NoticePriorityNormal = "normal"
	NoticePriorityHigh   = "high"
	NoticeScopeStaff     = "staff"
)

// personalReminderTypes are the notification kinds the producer can send, in
// the order the consent lookup runs.
var personalReminderTypes = []string{
	TypePickupUpcoming,
	TypePickupOverdue,
	TypeActivityStart,
	TypeActivityOverdue,
	TypeActivityRunningOver,
	TypeMyActivityStarting,
}

// PersonalReminderNotice is one person's due count of one kind, rendered for
// the Delivery Platform's notification service.
type PersonalReminderNotice struct {
	Type            string
	IdempotencyKey  string
	RelatedType     string
	RelatedID       int64
	TenantID        int64
	Scope           string
	StaffAccountIDs []int64
	Priority        string
	Title           string
	Body            string
	DeepLink        string
	Data            map[string]string
}

// PersonalReminderConsent is the consent lookup of the notification
// preferences. HasAnyOptedIn is only a cheap gate; FilterOptedInByType still
// narrows the relation-derived audience and applies each kind's school-wide
// gate.
type PersonalReminderConsent interface {
	HasAnyOptedIn(ctx context.Context, notificationTypes []string) (bool, error)
	FilterOptedInByType(ctx context.Context, notificationTypes []string, accountIDs []int64) (map[string][]int64, error)
}

// PersonalReminderStaffAccounts lists the login account of every staff member.
type PersonalReminderStaffAccounts interface {
	ListAllStaffAccountIDs(ctx context.Context) (map[int64]int64, error)
}

// PersonalReminderAdminAccounts lists the effective admin accounts.
type PersonalReminderAdminAccounts interface {
	ListEffectiveAdminAccountIDs(ctx context.Context) ([]int64, error)
}

// PersonalReminderDependencies is what the producer needs. All of it or none:
// a partially wired producer would resolve a narrower audience for a reason
// nobody intended, so construction refuses an incomplete set.
type PersonalReminderDependencies struct {
	Computer BatchComputer
	Consent  PersonalReminderConsent
	Staff    PersonalReminderStaffAccounts
	Admins   PersonalReminderAdminAccounts
	// OnDutyStaff answers which staff members are working right now.
	OnDutyStaff func(context.Context) (map[int64]struct{}, error)
	// Notify dispatches the notices. Paused reports the ordinary answers of a
	// closed delivery window and a switched-off feature flag.
	Notify func(context.Context, []PersonalReminderNotice) error
	Paused func(error) bool
	// AfterCommit runs fn once the surrounding tenant transaction committed.
	AfterCommit func(context.Context, func())
	Logger      *slog.Logger
}

// reminderNotificationKey identifies one dispatched reminder occurrence for one
// person. The account is part of the key because the same pickup legitimately
// reaches several people, and each of them must hear about it once.
type reminderNotificationKey struct {
	tenantID  int64
	accountID int64
	reminder  string // type + entity id + due time
}

// PersonalReminderNotifications sends the personal reminder notices of one
// school per tick.
type PersonalReminderNotifications struct {
	deps PersonalReminderDependencies

	notified   sync.Map // reminderNotificationKey → time.Time
	notifiedMu sync.Mutex
	notifiedOn calendar.Date
}

// NewPersonalReminderNotifications builds the producer over a complete set of
// dependencies.
func NewPersonalReminderNotifications(deps PersonalReminderDependencies) (*PersonalReminderNotifications, error) {
	if deps.Computer == nil || deps.Consent == nil || deps.Staff == nil || deps.Admins == nil ||
		deps.OnDutyStaff == nil || deps.Notify == nil || deps.Paused == nil || deps.AfterCommit == nil || deps.Logger == nil {
		return nil, errors.New("personal reminder notifications: all dependencies are required")
	}
	return &PersonalReminderNotifications{deps: deps}, nil
}

// reminderRecipient is one person the tick may address, with the kinds of
// reminder they agreed to hear about.
type reminderRecipient struct {
	staffID   int64
	accountID int64
	isAdmin   bool
	consent   map[string]struct{}
}

func (r reminderRecipient) resultKey() int64 {
	if r.staffID > 0 {
		return r.staffID
	}
	// Account and staff IDs come from different sequences. Keeping staff-less
	// admins in the negative half makes their opaque batch keys collision-free.
	return -r.accountID
}

// NotifyDue resolves the audience of one school, computes everybody's personal
// list in one batch and dispatches what is new. onDutyOnly restricts the
// audience to staff members who are working right now. Failures are logged:
// the next tick tries again.
func (p *PersonalReminderNotifications) NotifyDue(ctx context.Context, tenantID int64, now time.Time, onDutyOnly bool) {
	p.rotateIfNewDay(now)

	// This is a gate, never an audience source. Stale consent rows can survive
	// offboarding, so a positive answer only justifies paying for the canonical
	// staff/admin relationship lookup below.
	hasAnyConsent, err := p.deps.Consent.HasAnyOptedIn(ctx, personalReminderTypes)
	if err != nil {
		p.warn("reminder notification tick: consent gate failed", tenantID, err)
		return
	}
	if !hasAnyConsent {
		return
	}

	recipients, err := p.resolveRecipients(ctx, onDutyOnly)
	if err != nil {
		p.warn("reminder notification tick: audience resolution failed", tenantID, err)
		return
	}
	if len(recipients) == 0 {
		return
	}

	results, err := p.deps.Computer.ComputeBatch(ctx, batchScopes(recipients))
	if err != nil {
		p.warn("reminder notification tick: compute failed", tenantID, err)
		return
	}

	notices, freshKeys := buildPersonalReminderNotices(tenantID, calendar.DateFromTime(now), recipients, results, func(key reminderNotificationKey) bool {
		_, seen := p.notified.Load(key)
		return seen
	})
	if len(notices) == 0 {
		return
	}
	p.dispatch(ctx, tenantID, now, len(recipients), notices, freshKeys)
}

func batchScopes(recipients []reminderRecipient) []BatchScope {
	scopes := make([]BatchScope, 0, len(recipients))
	for _, r := range recipients {
		_, includeAssignedStart := r.consent[TypeMyActivityStarting]
		scopes = append(scopes, BatchScope{
			Scope:                        Scope{IsAdmin: r.isAdmin, StaffID: r.staffID},
			ResultKey:                    r.resultKey(),
			IncludeAssignedActivityStart: includeAssignedStart,
		})
	}
	return scopes
}

func (p *PersonalReminderNotifications) dispatch(
	ctx context.Context,
	tenantID int64,
	now time.Time,
	recipientCount int,
	notices []PersonalReminderNotice,
	freshKeys []reminderNotificationKey,
) {
	if err := p.deps.Notify(ctx, notices); err != nil {
		// A closed delivery window and a flipped feature flag are ordinary
		// answers, not failures: quiet hours would otherwise produce one warning
		// per tenant per minute all evening. Either way the occurrences stay
		// unmarked and fire on the next tick once dispatch is possible again.
		if !p.deps.Paused(err) {
			p.warn("reminder notification tick: notify failed", tenantID, err)
		}
		return
	}

	// Notify registered delivery first, so this hook runs after delivery and
	// only after the surrounding tenant transaction commits. A rollback or
	// commit failure drops both hooks and leaves the occurrences eligible for
	// the next tick.
	p.deps.AfterCommit(ctx, func() {
		for _, key := range freshKeys {
			p.notified.Store(key, now)
		}
		p.deps.Logger.Info("personal reminder notifications dispatched",
			slog.Int64("tenant_id", tenantID),
			slog.Int("recipient_count", recipientCount),
			slog.Int("event_count", len(notices)),
		)
	})
}

func (p *PersonalReminderNotifications) warn(message string, tenantID int64, err error) {
	p.deps.Logger.Warn(message,
		slog.Int64("tenant_id", tenantID),
		slog.String("error", err.Error()),
	)
}

// resolveRecipients builds the audience: every staff member with a login plus
// every effective admin account, narrowed by their own consent and, when the
// school asks for it, by whether staff-backed recipients are on duty.
//
// The order is the one the consent service prescribes — a relation first, then
// consent narrowing it. Consent is never the source of the audience, it is the
// filter on top; that is also why an empty consent result ends the tick before
// any reminder is computed.
func (p *PersonalReminderNotifications) resolveRecipients(ctx context.Context, onDutyOnly bool) ([]reminderRecipient, error) {
	audience, err := p.loadAudience(ctx)
	if err != nil {
		return nil, err
	}
	if len(audience.accountIDs) == 0 {
		return nil, nil
	}

	consentByAccount, err := p.consentByAccount(ctx, audience.accountIDs)
	if err != nil {
		return nil, err
	}
	if len(consentByAccount) == 0 {
		return nil, nil
	}

	var onDuty map[int64]struct{}
	if onDutyOnly {
		// A nil map means "no restriction"; an empty, non-nil map means nobody
		// is currently on duty.
		if onDuty, err = p.deps.OnDutyStaff(ctx); err != nil {
			return nil, fmt.Errorf("load staff presence: %w", err)
		}
		if onDuty == nil {
			onDuty = map[int64]struct{}{}
		}
	}
	return audience.recipients(consentByAccount, onDuty), nil
}

// reminderAudience is the relation-derived audience before consent.
type reminderAudience struct {
	accountIDs     []int64
	staffByAccount map[int64]int64
	admins         map[int64]struct{}
}

func (p *PersonalReminderNotifications) loadAudience(ctx context.Context) (reminderAudience, error) {
	accountsByStaff, err := p.deps.Staff.ListAllStaffAccountIDs(ctx)
	if err != nil {
		return reminderAudience{}, fmt.Errorf("list staff accounts: %w", err)
	}
	adminIDs, err := p.deps.Admins.ListEffectiveAdminAccountIDs(ctx)
	if err != nil {
		return reminderAudience{}, fmt.Errorf("list admin accounts: %w", err)
	}

	audience := reminderAudience{
		staffByAccount: make(map[int64]int64, len(accountsByStaff)),
		admins:         make(map[int64]struct{}, len(adminIDs)),
	}
	accountSet := make(map[int64]struct{}, len(accountsByStaff)+len(adminIDs))
	for staffID, accountID := range accountsByStaff {
		if accountID <= 0 {
			continue
		}
		// An account should map to one staff row. Choosing the smaller ID keeps
		// resolution deterministic if inconsistent legacy data says otherwise.
		current, exists := audience.staffByAccount[accountID]
		if !exists || staffID < current {
			audience.staffByAccount[accountID] = staffID
		}
		accountSet[accountID] = struct{}{}
	}
	for _, accountID := range adminIDs {
		if accountID <= 0 {
			continue
		}
		audience.admins[accountID] = struct{}{}
		accountSet[accountID] = struct{}{}
	}

	audience.accountIDs = make([]int64, 0, len(accountSet))
	for accountID := range accountSet {
		audience.accountIDs = append(audience.accountIDs, accountID)
	}
	sort.Slice(audience.accountIDs, func(i, j int) bool { return audience.accountIDs[i] < audience.accountIDs[j] })
	return audience, nil
}

// consentByAccount narrows the accounts to the kinds each agreed to.
// FilterOptedInByType also applies every type's school-wide gate (the matching
// reminders.* setting). The consent service resolves the gates in one settings
// query and the preferences in one IN query.
func (p *PersonalReminderNotifications) consentByAccount(ctx context.Context, accountIDs []int64) (map[int64]map[string]struct{}, error) {
	optedInByType, err := p.deps.Consent.FilterOptedInByType(ctx, personalReminderTypes, accountIDs)
	if err != nil {
		return nil, fmt.Errorf("filter opted-in accounts: %w", err)
	}
	consentByAccount := make(map[int64]map[string]struct{}, len(accountIDs))
	for notificationType, optedIn := range optedInByType {
		for _, accountID := range optedIn {
			if consentByAccount[accountID] == nil {
				consentByAccount[accountID] = make(map[string]struct{}, len(personalReminderTypes))
			}
			consentByAccount[accountID][notificationType] = struct{}{}
		}
	}
	return consentByAccount, nil
}

// recipients combines relation, consent and duty. A nil onDuty means no duty
// restriction.
func (a reminderAudience) recipients(consentByAccount map[int64]map[string]struct{}, onDuty map[int64]struct{}) []reminderRecipient {
	recipients := make([]reminderRecipient, 0, len(consentByAccount))
	for _, accountID := range a.accountIDs {
		consent := consentByAccount[accountID]
		if len(consent) == 0 {
			continue
		}
		staffID := a.staffByAccount[accountID]
		_, isAdmin := a.admins[accountID]
		if !a.addressable(staffID, isAdmin, onDuty) {
			continue
		}
		recipients = append(recipients, reminderRecipient{
			staffID:   staffID,
			accountID: accountID,
			isAdmin:   isAdmin,
			consent:   consent,
		})
	}
	// accountIDs is sorted, so the dispatched batch (and therefore the logs and
	// the tests) stays reproducible.
	return recipients
}

func (a reminderAudience) addressable(staffID int64, isAdmin bool, onDuty map[int64]struct{}) bool {
	if staffID <= 0 && !isAdmin {
		return false
	}
	if onDuty == nil {
		return true
	}
	if staffID <= 0 {
		return false
	}
	_, working := onDuty[staffID]
	return working
}

// buildPersonalReminderNotices turns the batch results into one notice per
// person and notification type, skipping everything the person did not agree
// to and everything they already heard about today.
//
// Pure apart from the injected seen-check, so the whole mapping — consent,
// classification, aggregation — is testable without the producer.
func buildPersonalReminderNotices(
	tenantID int64,
	day calendar.Date,
	recipients []reminderRecipient,
	results map[int64]*Result,
	seen func(reminderNotificationKey) bool,
) ([]PersonalReminderNotice, []reminderNotificationKey) {
	var (
		notices   []PersonalReminderNotice
		freshKeys []reminderNotificationKey
	)

	for _, r := range recipients {
		result := results[r.resultKey()]
		if result == nil || !result.Enabled || len(result.Reminders) == 0 {
			continue
		}
		occurrences, order, keys := collectFreshReminders(tenantID, r, result, seen)
		freshKeys = append(freshKeys, keys...)
		for _, notificationType := range order {
			notices = append(notices, buildPersonalReminderNotice(
				tenantID, day, r.accountID, notificationType, occurrences[notificationType]))
		}
	}

	return notices, freshKeys
}

// collectFreshReminders gathers one person's wanted, not yet notified reminder
// identities per notification type, in first-seen order.
func collectFreshReminders(
	tenantID int64,
	r reminderRecipient,
	result *Result,
	seen func(reminderNotificationKey) bool,
) (map[string][]string, []string, []reminderNotificationKey) {
	occurrences := make(map[string][]string, len(personalReminderTypes))
	order := make([]string, 0, len(personalReminderTypes))
	var freshKeys []reminderNotificationKey

	for _, reminder := range result.Reminders {
		notificationType, wanted := notificationTypeFor(reminder, result.AssignedActivityInstanceIDs, r.consent)
		if !wanted {
			continue
		}
		key := reminderNotificationKey{
			tenantID:  tenantID,
			accountID: r.accountID,
			reminder:  reminderIdentity(reminder),
		}
		if seen(key) {
			continue
		}
		if len(occurrences[notificationType]) == 0 {
			order = append(order, notificationType)
		}
		occurrences[notificationType] = append(occurrences[notificationType], key.reminder)
		freshKeys = append(freshKeys, key)
	}
	return occurrences, order, freshKeys
}

// notificationTypeFor decides which notification type a reminder belongs to for
// one person, and whether that person wants it.
//
// The only non-obvious case is an activity the person is personally planned on.
// It is reported as "Ihr nächster Einsatz" even when they also happen to
// supervise the room, because that is what the two catalogue descriptions
// promise: the room type is about "a room you are watching", the assignment type
// about your own slot "auch wenn Sie gerade keine Aufsicht führen". Each switch
// therefore means exactly what it says, and the same activity never arrives
// twice.
func notificationTypeFor(
	reminder Reminder,
	assigned map[string]struct{},
	consent map[string]struct{},
) (string, bool) {
	notificationType := reminder.Type
	if reminder.Type == TypeActivityStart && reminder.ActivityInstanceID != nil {
		if _, mine := assigned[*reminder.ActivityInstanceID]; mine {
			notificationType = TypeMyActivityStarting
		}
	}
	if _, agreed := consent[notificationType]; !agreed {
		return "", false
	}
	return notificationType, true
}

// reminderIdentity builds the dedup identity of one reminder occurrence:
// type + entity + due time. A rescheduled pickup (new DueTime) is a new
// occurrence on purpose — the plan changed, staff should hear about it again.
func reminderIdentity(r Reminder) string {
	entity := ""
	switch {
	case r.StudentID != nil:
		entity = "s" + *r.StudentID
	case r.ActivityInstanceID != nil:
		entity = "a" + *r.ActivityInstanceID
	}
	return r.Type + "|" + entity + "|" + r.DueTime
}

// personalReminderCopy is the display-safe German wording per notification
// type. Counts only: these strings end up on a lock screen, and the reminder
// rows they summarise carry child names.
type personalReminderCopy struct {
	title    string
	singular string
	plural   string // one %d
	urgent   bool
}

var reminderCopyByType = map[string]personalReminderCopy{
	TypePickupUpcoming: {
		title:    "Abholung steht an",
		singular: "Ein Kind aus Ihrer Aufsicht wird bald abgeholt.",
		plural:   "%d Kinder aus Ihrer Aufsicht werden bald abgeholt.",
	},
	TypePickupOverdue: {
		title:    "Abholung überfällig",
		singular: "Ein Kind ist noch da, obwohl die Abholzeit vorbei ist.",
		plural:   "%d Kinder sind noch da, obwohl ihre Abholzeit vorbei ist.",
		urgent:   true,
	},
	TypeActivityStart: {
		title:    "Aktivität beginnt",
		singular: "In einem Raum Ihrer Aufsicht beginnt gleich eine Aktivität.",
		plural:   "In Ihren Räumen beginnen gleich %d Aktivitäten.",
	},
	TypeActivityOverdue: {
		title:    "Aktivität nicht gestartet",
		singular: "Eine geplante Aktivität ist überfällig und wurde noch nicht gestartet.",
		plural:   "%d geplante Aktivitäten sind überfällig und wurden noch nicht gestartet.",
		urgent:   true,
	},
	TypeActivityRunningOver: {
		title:    "AG dauert länger",
		singular: "Eine Aktivität läuft länger als geplant.",
		plural:   "%d Aktivitäten laufen länger als geplant.",
		urgent:   true,
	},
	TypeMyActivityStarting: {
		title:    "Ihr Einsatz beginnt",
		singular: "Einer Ihrer geplanten Einsätze beginnt gleich.",
		plural:   "%d Ihrer geplanten Einsätze beginnen gleich.",
	},
}

// buildPersonalReminderNotice renders one person's due occurrences of one
// kind as their count.
func buildPersonalReminderNotice(tenantID int64, day calendar.Date, accountID int64, notificationType string, occurrences []string) PersonalReminderNotice {
	count := len(occurrences)
	copyFor, known := reminderCopyByType[notificationType]
	if !known {
		// Unreachable while personalReminderTypes and the copy table agree,
		// which a test pins. A generic wording still beats an empty title,
		// which validation would reject and which would drop the whole batch.
		copyFor = personalReminderCopy{
			title:    "Erinnerung",
			singular: "Eine Erinnerung ist fällig.",
			plural:   "%d Erinnerungen sind fällig.",
		}
	}

	body := copyFor.singular
	if count > 1 {
		body = fmt.Sprintf(copyFor.plural, count)
	}

	priority := NoticePriorityNormal
	if copyFor.urgent {
		priority = NoticePriorityHigh
	}

	return PersonalReminderNotice{
		Type:            notificationType,
		IdempotencyKey:  personalReminderIdempotencyKey(tenantID, day, accountID, notificationType, occurrences),
		RelatedType:     "personal_reminder",
		RelatedID:       accountID,
		TenantID:        tenantID,
		Scope:           NoticeScopeStaff,
		StaffAccountIDs: []int64{accountID},
		Priority:        priority,
		Title:           copyFor.title,
		Body:            body,
		DeepLink:        "/reminders",
		Data:            map[string]string{"count": strconv.Itoa(count)},
	}
}

// personalReminderIdempotencyKey names the tick's day and the exact
// occurrences a notice reports. The count alone would collide with a later
// notice of the same kind and count that day, which the push outbox would
// then drop; the sorted identities keep the key independent of the order the
// batch listed them in.
func personalReminderIdempotencyKey(tenantID int64, day calendar.Date, accountID int64, notificationType string, occurrences []string) string {
	digest := fnv.New64a()
	for _, occurrence := range slices.Sorted(slices.Values(occurrences)) {
		_, _ = digest.Write([]byte(occurrence + "\n"))
	}
	return fmt.Sprintf("personal-reminder:%d:%d:%s:%s:%x",
		tenantID, accountID, notificationType, day.String(), digest.Sum64())
}

// rotateIfNewDay clears the re-fire guard on civil-date rollover, mirroring
// the Worker's overdue cache. Reminder identities embed only wall-clock due
// times, so yesterday's keys would otherwise suppress today's identical
// schedule.
func (p *PersonalReminderNotifications) rotateIfNewDay(now time.Time) {
	today := calendar.DateFromTime(now)
	p.notifiedMu.Lock()
	defer p.notifiedMu.Unlock()
	if p.notifiedOn != today {
		p.notified.Range(func(k, _ any) bool {
			p.notified.Delete(k)
			return true
		})
		p.notifiedOn = today
	}
}
