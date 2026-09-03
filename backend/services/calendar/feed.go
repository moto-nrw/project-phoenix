package calendar

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	calModels "github.com/moto-nrw/project-phoenix/models/calendar"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Feed window: how far back / forward the subscription includes appointments.
// Recurrence is emitted as RRULE, so a bounded window still yields an ongoing
// series in the subscriber's calendar.
const (
	feedPastDays   = 30
	feedFutureDays = 365
	// feedTombstoneDays is how long a deleted appointment keeps being re-exported
	// as a STATUS:CANCELLED tombstone. It is deliberately longer than feedPastDays
	// so a subscriber that was offline past the normal lookback still receives the
	// cancellation and purges the stale event, rather than retaining it forever.
	feedTombstoneDays = 90
)

// FeedAccountRepo is the slice of the account repository the calendar feed
// needs: resolve/rotate the per-parent subscription token.
type FeedAccountRepo interface {
	FindByID(ctx context.Context, id any) (*authModels.Account, error)
	FindByCalendarFeedToken(ctx context.Context, token string) (*authModels.Account, error)
	SetCalendarFeedToken(ctx context.Context, accountID int64, token string) error
	EnsureCalendarFeedToken(ctx context.Context, accountID int64, newToken string) (string, error)
}

// ParentCalendarFeedURL returns the parent's iCalendar subscription URLs. The
// raw token only ever exists in memory at generation time (only its hash is
// stored), so the URLs are show-once: this returns them ONLY when it just
// generated the token (no feed existed yet). If a feed already exists, it
// returns empty URLs — the raw token is unrecoverable, and regenerating on every
// view would silently break the parent's existing subscription. The caller shows
// a "regenerate to reveal" state for empty URLs. Rotate is the way to see a link
// again.
func (s *service) ParentCalendarFeedURL(ctx context.Context, accountID int64) (string, string, error) {
	if s.cfg.AccountRepo == nil {
		return "", "", fmt.Errorf("%w: calendar feed not configured", ErrInvalidRequest)
	}
	if accountID <= 0 {
		return "", "", fmt.Errorf("%w: account id is required", ErrForbidden)
	}
	return s.coalesceFeedCreation(fmt.Sprintf("parent:%d", accountID), func() (string, string, error) {
		account, err := s.cfg.AccountRepo.FindByID(ctx, accountID)
		if err != nil {
			return "", "", err
		}
		if account == nil {
			return "", "", ErrNotFound
		}
		if account.CalendarFeedToken != nil && *account.CalendarFeedToken != "" {
			// A feed already exists (only its hash is stored) — not re-displayable.
			return "", "", nil
		}
		token, err := newFeedToken()
		if err != nil {
			return "", "", err
		}
		persisted, err := s.cfg.AccountRepo.EnsureCalendarFeedToken(ctx, accountID, feedTokenHash(token))
		if err != nil {
			return "", "", err
		}
		if persisted != feedTokenHash(token) {
			return "", "", fmt.Errorf("%w: calendar feed was created concurrently", ErrConflict)
		}
		httpsURL, webcalURL := feedURLs(s.cfg.ParentsURL, token)
		return httpsURL, webcalURL, nil
	})
}

// RotateParentCalendarFeed issues a fresh token, invalidating the previous
// subscription URL. Only the new token's hash is persisted; the raw URL is
// returned once.
func (s *service) RotateParentCalendarFeed(ctx context.Context, accountID int64) (string, string, error) {
	if s.cfg.AccountRepo == nil {
		return "", "", fmt.Errorf("%w: calendar feed not configured", ErrInvalidRequest)
	}
	if accountID <= 0 {
		return "", "", fmt.Errorf("%w: account id is required", ErrForbidden)
	}
	token, err := newFeedToken()
	if err != nil {
		return "", "", err
	}
	if err := s.cfg.AccountRepo.SetCalendarFeedToken(ctx, accountID, feedTokenHash(token)); err != nil {
		return "", "", err
	}
	httpsURL, webcalURL := feedURLs(s.cfg.ParentsURL, token)
	return httpsURL, webcalURL, nil
}

func (s *service) StaffCalendarFeedURL(ctx context.Context) (string, string, error) {
	accountID, tenantID, err := s.currentStaffFeedOwner(ctx)
	if err != nil {
		return "", "", err
	}
	return s.coalesceFeedCreation(fmt.Sprintf("staff:%d:%d", accountID, tenantID), func() (string, string, error) {
		token, err := newFeedToken()
		if err != nil {
			return "", "", err
		}
		persisted, err := s.cfg.StaffFeedRepo.EnsureToken(ctx, accountID, tenantID, feedTokenHash(token))
		if err != nil {
			return "", "", err
		}
		if persisted == "" {
			return "", "", ErrNotFound
		}
		if persisted != feedTokenHash(token) {
			return "", "", nil
		}
		httpsURL, webcalURL := feedURLs(s.cfg.FrontendURL, token)
		return httpsURL, webcalURL, nil
	})
}

type feedCreationResult struct {
	httpsURL  string
	webcalURL string
}

// coalesceFeedCreation lets concurrent first-time requests share the one raw
// token before only its irreversible hash remains. Later requests still run
// normally and receive the intentional already-active response with empty URLs.
func (s *service) coalesceFeedCreation(key string, create func() (string, string, error)) (string, string, error) {
	value, err, _ := s.feedCreation.Do(key, func() (any, error) {
		httpsURL, webcalURL, err := create()
		return feedCreationResult{httpsURL: httpsURL, webcalURL: webcalURL}, err
	})
	if err != nil {
		return "", "", err
	}
	result := value.(feedCreationResult)
	return result.httpsURL, result.webcalURL, nil
}

func (s *service) RotateStaffCalendarFeed(ctx context.Context) (string, string, error) {
	accountID, tenantID, err := s.currentStaffFeedOwner(ctx)
	if err != nil {
		return "", "", err
	}
	token, err := newFeedToken()
	if err != nil {
		return "", "", err
	}
	updated, err := s.cfg.StaffFeedRepo.RotateToken(ctx, accountID, tenantID, feedTokenHash(token))
	if err != nil {
		return "", "", err
	}
	if !updated {
		return "", "", ErrNotFound
	}
	httpsURL, webcalURL := feedURLs(s.cfg.FrontendURL, token)
	return httpsURL, webcalURL, nil
}

func (s *service) currentStaffFeedOwner(ctx context.Context) (int64, int64, error) {
	if s.cfg.StaffFeedRepo == nil || s.cfg.UserContext == nil || strings.TrimSpace(s.cfg.FrontendURL) == "" {
		return 0, 0, fmt.Errorf("%w: staff calendar feed not configured", ErrInvalidRequest)
	}
	account, err := s.cfg.UserContext.GetCurrentUser(ctx)
	if err != nil {
		if errors.Is(err, usercontext.ErrUserNotAuthenticated) || errors.Is(err, usercontext.ErrUserNotFound) {
			return 0, 0, fmt.Errorf("%w: current account required", ErrForbidden)
		}
		return 0, 0, err
	}
	if account == nil || !account.IsActive() {
		return 0, 0, fmt.Errorf("%w: current account required", ErrForbidden)
	}
	staff, err := s.cfg.UserContext.GetCurrentStaff(ctx)
	if err != nil {
		if errors.Is(err, usercontext.ErrUserNotAuthenticated) ||
			errors.Is(err, usercontext.ErrUserNotFound) ||
			errors.Is(err, usercontext.ErrUserNotLinkedToPerson) ||
			errors.Is(err, usercontext.ErrUserNotLinkedToStaff) {
			return 0, 0, fmt.Errorf("%w: current staff required", ErrForbidden)
		}
		return 0, 0, err
	}
	if staff == nil {
		return 0, 0, fmt.Errorf("%w: current staff required", ErrForbidden)
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return 0, 0, fmt.Errorf("%w: tenant is required", ErrForbidden)
	}
	return account.ID, tenantID, nil
}

func (s *service) StaffCalendarFeedByToken(ctx context.Context, token string) (string, string, error) {
	if s.cfg.StaffFeedRepo == nil || s.cfg.StaffFeedTombstoneRepo == nil || s.cfg.AccountRepo == nil || s.cfg.PersonRepo == nil || s.cfg.StaffRepo == nil {
		return "", "", fmt.Errorf("%w: staff calendar feed not configured", ErrInvalidRequest)
	}
	if strings.TrimSpace(token) == "" {
		return "", "", ErrNotFound
	}
	owner, err := s.cfg.StaffFeedRepo.FindOwnerByTokenHash(ctx, feedTokenHash(token))
	if err != nil {
		return "", "", err
	}
	if owner == nil {
		return "", "", ErrNotFound
	}
	account, err := s.cfg.AccountRepo.FindByID(ctx, owner.AccountID)
	if err != nil {
		return "", "", err
	}
	if account == nil || !account.IsActive() {
		return "", "", ErrNotFound
	}

	var events []CalendarEvent
	err = tenant.WithTenantTx(ctx, s.cfg.DB, owner.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		person, err := s.cfg.PersonRepo.FindByAccountID(txCtx, owner.AccountID)
		if err != nil {
			return err
		}
		if person == nil {
			return ErrNotFound
		}
		staff, err := s.cfg.StaffRepo.FindByPersonID(txCtx, person.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if staff == nil {
			return ErrNotFound
		}
		reachable, err := s.cfg.StaffRepo.FindReachableCalendarStaffIDs(txCtx, []int64{staff.ID})
		if err != nil {
			return err
		}
		if !reachable[staff.ID] {
			return ErrNotFound
		}

		from := timezone.TodayDate().AddDays(-feedPastDays)
		to := timezone.TodayDate().AddDays(feedFutureDays)
		appointments, err := s.cfg.AppointmentRepo.ListVisibleForStaff(txCtx, staff.ID, toCalendarDate(from), toCalendarDate(to))
		if err != nil {
			return err
		}
		ids := make([]int64, 0, len(appointments))
		for _, appointment := range appointments {
			ids = append(ids, appointment.ID)
		}
		recurrences, err := s.cfg.RecurrenceRepo.FindByAppointmentIDs(txCtx, ids)
		if err != nil {
			return err
		}
		recurrenceByID := make(map[int64]*calModels.RecurrenceRule, len(recurrences))
		for _, recurrence := range recurrences {
			recurrenceByID[recurrence.AppointmentID] = recurrence
		}
		cancelledOverrides, err := s.cfg.OverrideRepo.FindCancelledByAppointmentIDs(txCtx, ids)
		if err != nil {
			return err
		}
		overridesByID := make(map[int64][]*calModels.AppointmentOccurrenceOverride, len(cancelledOverrides))
		for _, override := range cancelledOverrides {
			overridesByID[override.AppointmentID] = append(overridesByID[override.AppointmentID], override)
		}
		emittedIDs := make(map[int64]struct{}, len(appointments))
		for _, appointment := range appointments {
			recurrence := recurrenceByID[appointment.ID]
			if recurrence != nil && !hasOccurrenceInWindow(appointment, recurrence, from, to) {
				continue
			}
			event := appointmentICSEvent(appointment, recurrence, overridesByID[appointment.ID])
			event.Description = ""
			events = append(events, event)
			emittedIDs[appointment.ID] = struct{}{}
		}

		tombstoneCutoff := timezone.TodayDate().AddDays(-feedTombstoneDays).BerlinMidnight()
		tombstones, err := s.cfg.AppointmentRepo.ListCancellationTombstonesForStaff(txCtx, staff.ID, tombstoneCutoff)
		if err != nil {
			return err
		}
		tombstoneIDs := make([]int64, 0, len(tombstones))
		for _, appointment := range tombstones {
			if _, emitted := emittedIDs[appointment.ID]; !emitted {
				tombstoneIDs = append(tombstoneIDs, appointment.ID)
			}
		}
		tombstoneRecurrences, err := s.cfg.RecurrenceRepo.FindByAppointmentIDs(txCtx, tombstoneIDs)
		if err != nil {
			return err
		}
		tombstoneRecurrenceByID := make(map[int64]*calModels.RecurrenceRule, len(tombstoneRecurrences))
		for _, recurrence := range tombstoneRecurrences {
			tombstoneRecurrenceByID[recurrence.AppointmentID] = recurrence
		}
		for _, appointment := range tombstones {
			if _, emitted := emittedIDs[appointment.ID]; emitted {
				continue
			}
			event := appointmentICSEvent(appointment, tombstoneRecurrenceByID[appointment.ID], nil)
			event.Description = ""
			events = append(events, event)
		}

		timetableEvents, err := s.staffTimetableFeedEvents(txCtx, staff.ID, from, to)
		if err != nil {
			return err
		}
		shiftEvents, err := s.staffShiftFeedEvents(txCtx, staff.ID, from, to)
		if err != nil {
			return err
		}
		staffEvents := append(timetableEvents, shiftEvents...)
		currentStaffEventIDs := make(map[string]struct{}, len(staffEvents))
		for _, staffEvent := range staffEvents {
			currentStaffEventIDs[staffEvent.ID] = struct{}{}
			event, err := staffCalendarICSEvent(owner.TenantID, staffEvent)
			if err != nil {
				return err
			}
			events = append(events, event)
		}
		staffTombstones, err := s.cfg.StaffFeedTombstoneRepo.ListForStaffSince(txCtx, staff.ID, tombstoneCutoff)
		if err != nil {
			return err
		}
		for _, tombstone := range staffTombstones {
			staffEvent := staffFeedTombstoneEvent(tombstone)
			if _, current := currentStaffEventIDs[staffEvent.ID]; current {
				continue
			}
			event, err := staffCalendarICSEvent(owner.TenantID, staffEvent)
			if err != nil {
				return err
			}
			events = append(events, event)
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return "", "", ErrNotFound
		}
		return "", "", err
	}
	content, err := s.renderCalendar(ctx, "moto Termine", events)
	return "moto-kalender.ics", content, err
}

func staffFeedTombstoneEvent(tombstone *calModels.StaffFeedTombstone) Event {
	return Event{
		ID:         fmt.Sprintf("%s:%d", tombstone.Source, tombstone.SourceID),
		Source:     tombstone.Source,
		Title:      tombstone.Title,
		StartDate:  tombstone.EventDate.String(),
		EndDate:    tombstone.EventDate.String(),
		StartTime:  formatClock(tombstone.StartTime),
		EndTime:    formatClock(tombstone.EndTime),
		Cancelled:  true,
		ModifiedAt: tombstone.CancelledAt,
	}
}

func staffCalendarICSEvent(tenantID int64, event Event) (CalendarEvent, error) {
	startDate, err := timezone.ParseDate(event.StartDate)
	if err != nil {
		return CalendarEvent{}, fmt.Errorf("parse staff calendar start date: %w", err)
	}
	endDate, err := timezone.ParseDate(event.EndDate)
	if err != nil {
		return CalendarEvent{}, fmt.Errorf("parse staff calendar end date: %w", err)
	}
	startClock, err := staffCalendarClock(event.StartTime)
	if err != nil {
		return CalendarEvent{}, err
	}
	endClock, err := staffCalendarClock(event.EndTime)
	if err != nil {
		return CalendarEvent{}, err
	}
	location := ""
	if event.Location != nil {
		location = *event.Location
	}
	return CalendarEvent{
		UID:          fmt.Sprintf("%s-%d@moto-app.de", strings.ReplaceAll(event.ID, ":", "-"), tenantID),
		Summary:      event.Title,
		Location:     location,
		StartDate:    startDate.String(),
		EndDate:      endDate.String(),
		StartClock:   startClock,
		EndClock:     endClock,
		AllDay:       event.AllDay,
		Cancelled:    event.Cancelled,
		Stamp:        event.ModifiedAt,
		LastModified: event.ModifiedAt,
	}, nil
}

func staffCalendarClock(value string) (time.Time, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return time.Time{}, fmt.Errorf("invalid staff calendar clock %q", value)
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil {
		return time.Time{}, fmt.Errorf("parse staff calendar hour: %w", err)
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil {
		return time.Time{}, fmt.Errorf("parse staff calendar minute: %w", err)
	}
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return time.Time{}, fmt.Errorf("invalid staff calendar clock %q", value)
	}
	return timezone.NormalizeWallClock(time.Date(2000, time.January, 1, hour, minute, 0, 0, time.UTC)), nil
}

// ParentCalendarFeedByToken renders the subscription feed for the account that
// owns the token. Unauthenticated: the token is the only credential, so an
// unknown token is a plain 404.
func (s *service) ParentCalendarFeedByToken(ctx context.Context, token string) (string, string, error) {
	if s.cfg.AccountRepo == nil {
		return "", "", fmt.Errorf("%w: calendar feed not configured", ErrInvalidRequest)
	}
	if strings.TrimSpace(token) == "" {
		return "", "", ErrNotFound
	}
	// Only the token's hash is stored, so hash the presented raw token before the
	// lookup (matching how it was persisted at generation/rotation).
	account, err := s.cfg.AccountRepo.FindByCalendarFeedToken(ctx, feedTokenHash(token))
	if err != nil {
		return "", "", err
	}
	if account == nil {
		return "", "", ErrNotFound
	}
	// The public feed bypasses normal parent auth — the token is the only
	// credential. A deactivated account must lose feed access immediately, so
	// treat an inactive account like an unknown token (plain 404, no leak).
	if !account.IsActive() {
		return "", "", ErrNotFound
	}

	children, err := s.parentChildren(ctx, account.ID)
	if err != nil {
		return "", "", err
	}
	from := timezone.TodayDate().AddDays(-feedPastDays)
	to := timezone.TodayDate().AddDays(feedFutureDays)

	var events []CalendarEvent
	for tenantID, tenantChildren := range groupChildrenByTenant(children) {
		tenantID := tenantID
		tenantChildren := tenantChildren
		if err := tenant.WithTenantTx(ctx, s.cfg.DB, tenantID, func(txCtx context.Context, _ bun.Tx) error {
			guardianProfileIDs := distinctGuardianProfileIDs(tenantChildren)
			studentIDs := distinctChildStudentIDs(tenantChildren)
			appointments, err := s.cfg.AppointmentRepo.ListVisibleForGuardianProfiles(txCtx, guardianProfileIDs, studentIDs, toCalendarDate(from), toCalendarDate(to))
			if err != nil {
				return err
			}
			ids := make([]int64, 0, len(appointments))
			for _, appointment := range appointments {
				ids = append(ids, appointment.ID)
			}
			recurrences, err := s.cfg.RecurrenceRepo.FindByAppointmentIDs(txCtx, ids)
			if err != nil {
				return err
			}
			recurrenceByID := make(map[int64]*calModels.RecurrenceRule, len(recurrences))
			for _, recurrence := range recurrences {
				recurrenceByID[recurrence.AppointmentID] = recurrence
			}
			// Cancelled single occurrences become EXDATEs so subscribers drop
			// them from the RRULE expansion.
			cancelledOverrides, err := s.cfg.OverrideRepo.FindCancelledByAppointmentIDs(txCtx, ids)
			if err != nil {
				return err
			}
			overridesByID := make(map[int64][]*calModels.AppointmentOccurrenceOverride, len(cancelledOverrides))
			for _, override := range cancelledOverrides {
				overridesByID[override.AppointmentID] = append(overridesByID[override.AppointmentID], override)
			}
			// Emit each live appointment with its REAL (unclamped) recurrence — the
			// window only decides inclusion, never the RRULE horizon (see
			// appointmentICSEvent for why a moving UNTIL would freeze subscribers).
			emittedIDs := make(map[int64]struct{}, len(appointments))
			for _, appointment := range appointments {
				recurrence := recurrenceByID[appointment.ID]
				// A count-bounded recurring series (ends_on IS NULL, occurrence_count
				// set) can slip through applyAppointmentWindow, which treats a NULL
				// ends_on as open-ended, even when all its occurrences are already
				// before the feed's lookback. Skip any recurring appointment that
				// produces no occurrence overlapping the feed window. hasOccurrenceInWindow
				// checks overlap within a bounded window instead of expanding every
				// day since the (possibly years-old) series start on each poll.
				if recurrence != nil && !hasOccurrenceInWindow(appointment, recurrence, from, to) {
					continue
				}
				events = append(events, appointmentICSEvent(appointment, recurrence, overridesByID[appointment.ID]))
				emittedIDs[appointment.ID] = struct{}{}
			}

			// Cancelled or deleted appointments are re-exported as durable
			// STATUS:CANCELLED tombstones (retained by cancellation/deletion time,
			// independent of the date lookback) so even a long-offline subscriber
			// still receives the cancellation and purges the stale event. Skip any
			// already emitted above (a recently-cancelled appointment still inside
			// the date window) to avoid a duplicate UID in the feed.
			tombstoneCutoff := timezone.TodayDate().AddDays(-feedTombstoneDays).BerlinMidnight()
			tombstones, err := s.cfg.AppointmentRepo.ListCancellationTombstonesForGuardianProfiles(txCtx, guardianProfileIDs, studentIDs, tombstoneCutoff)
			if err != nil {
				return err
			}
			tombstoneIDs := make([]int64, 0, len(tombstones))
			for _, appointment := range tombstones {
				if _, done := emittedIDs[appointment.ID]; done {
					continue
				}
				tombstoneIDs = append(tombstoneIDs, appointment.ID)
			}
			tombstoneRecurrences, err := s.cfg.RecurrenceRepo.FindByAppointmentIDs(txCtx, tombstoneIDs)
			if err != nil {
				return err
			}
			tombstoneRecurrenceByID := make(map[int64]*calModels.RecurrenceRule, len(tombstoneRecurrences))
			for _, recurrence := range tombstoneRecurrences {
				tombstoneRecurrenceByID[recurrence.AppointmentID] = recurrence
			}
			for _, appointment := range tombstones {
				if _, done := emittedIDs[appointment.ID]; done {
					continue
				}
				events = append(events, appointmentICSEvent(appointment, tombstoneRecurrenceByID[appointment.ID], nil))
			}
			return nil
		}); err != nil {
			return "", "", err
		}
	}

	content, err := s.renderCalendar(ctx, "moto Termine", events)
	return "moto-kalender.ics", content, err
}

func feedURLs(portalURL, token string) (string, string) {
	base := strings.TrimRight(portalURL, "/") + "/api/calendar-feed/" + token
	webcal := base
	switch {
	case strings.HasPrefix(base, "https://"):
		webcal = "webcal://" + strings.TrimPrefix(base, "https://")
	case strings.HasPrefix(base, "http://"):
		webcal = "webcal://" + strings.TrimPrefix(base, "http://")
	}
	return base, webcal
}

// newFeedToken returns a 256-bit URL-safe secret capability token.
func newFeedToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate calendar feed token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// feedTokenHash returns the hex-encoded SHA-256 of the raw feed token. Only the
// hash is persisted (in auth.accounts.calendar_feed_token or the tenant-bound
// auth.account_tenants.staff_calendar_feed_token), so a database read, backup,
// or dump exposes no replayable /api/calendar-feed/{token} URL. SHA-256
// (not Argon2) is sufficient because the token is 256 bits of randomness — there
// is no offline-guessing surface. Mirrors the trusted-device / display token
// handling (services/auth.HashTrustedDeviceToken, services/display.displayTokenHash).
func feedTokenHash(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}
