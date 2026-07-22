package calendar

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/ical"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	calModels "github.com/moto-nrw/project-phoenix/models/calendar"
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

// ParentCalendarFeedURL returns the parent's iCalendar subscription URLs,
// generating the secret token on first request.
func (s *service) ParentCalendarFeedURL(ctx context.Context, accountID int64) (string, string, error) {
	token, err := s.ensureFeedToken(ctx, accountID)
	if err != nil {
		return "", "", err
	}
	httpsURL, webcalURL := feedURLs(s.cfg.ParentsURL, token)
	return httpsURL, webcalURL, nil
}

// RotateParentCalendarFeed issues a fresh token, invalidating the previous
// subscription URL.
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
	if err := s.cfg.AccountRepo.SetCalendarFeedToken(ctx, accountID, token); err != nil {
		return "", "", err
	}
	httpsURL, webcalURL := feedURLs(s.cfg.ParentsURL, token)
	return httpsURL, webcalURL, nil
}

func (s *service) ensureFeedToken(ctx context.Context, accountID int64) (string, error) {
	if s.cfg.AccountRepo == nil {
		return "", fmt.Errorf("%w: calendar feed not configured", ErrInvalidRequest)
	}
	if accountID <= 0 {
		return "", fmt.Errorf("%w: account id is required", ErrForbidden)
	}
	account, err := s.cfg.AccountRepo.FindByID(ctx, accountID)
	if err != nil {
		return "", err
	}
	if account == nil {
		return "", ErrNotFound
	}
	if account.CalendarFeedToken != nil && *account.CalendarFeedToken != "" {
		return *account.CalendarFeedToken, nil
	}
	token, err := newFeedToken()
	if err != nil {
		return "", err
	}
	// Atomic first-writer-wins: if two initial requests race, both get whichever
	// token actually persisted, never a URL a later write silently overwrote.
	return s.cfg.AccountRepo.EnsureCalendarFeedToken(ctx, accountID, token)
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
	account, err := s.cfg.AccountRepo.FindByCalendarFeedToken(ctx, token)
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

	var events []ical.Event
	for tenantID, tenantChildren := range groupChildrenByTenant(children) {
		tenantID := tenantID
		tenantChildren := tenantChildren
		if err := tenant.WithTenantTx(ctx, s.cfg.DB, tenantID, func(txCtx context.Context, _ bun.Tx) error {
			guardianProfileIDs := distinctGuardianProfileIDs(tenantChildren)
			studentIDs := distinctChildStudentIDs(tenantChildren)
			appointments, err := s.cfg.AppointmentRepo.ListVisibleForGuardianProfiles(txCtx, guardianProfileIDs, studentIDs, from, to)
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

	return "moto-kalender.ics", ical.Render("moto Termine", events), nil
}

func feedURLs(parentsURL, token string) (string, string) {
	base := strings.TrimRight(parentsURL, "/") + "/api/calendar-feed/" + token
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
