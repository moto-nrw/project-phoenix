package application

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/careplan"
	"github.com/moto-ogs/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-ogs/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-ogs/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// Device copy of a changed pickup time (#6855). It names no child: the text
// shows on a lock screen, possibly on a shared group tablet. The unread
// marker in moto and the child's page name the child.
const (
	pickupChangedTitle   = "Abholzeit geändert"
	pickupChangedBody    = "Neue Abholzeit für ein Kind: %s um %s Uhr."
	pickupWithdrawnTitle = "Abholzeit zurückgenommen"
	pickupWithdrawnBody  = "Für ein Kind gilt %s wieder die normale Abholzeit."
)

// MaxPickupChangeTeamNotices bounds the reader's unread list. It is a pointer
// to the children, not an archive.
const MaxPickupChangeTeamNotices = 50

var germanWeekdayShort = [...]string{"So.", "Mo.", "Di.", "Mi.", "Do.", "Fr.", "Sa."}

// ErrPickupChangeNoticeInvalid reports a read without reader or child.
var ErrPickupChangeNoticeInvalid = errors.New("pickup change notices: reader and child are required")

// PickupChangeTeamService tells the child's team about a changed one-day
// pickup time and serves the unread markers. It decides that the marker
// follows the request (a past day, a deleted request and a child without a
// directory row drop out) and that the person who decided needs no notice.
// Who the team is and how it hears about it on its devices is the reach the
// root binds.
type PickupChangeTeamService struct {
	store   ports.PickupChangeTeamStore
	reach   ports.PickupChangeTeamReach
	observe ports.Observer
	today   func() calendar.Date
}

var (
	_ careplan.PickupChangeNotifier    = (*PickupChangeTeamService)(nil)
	_ careplan.PickupChangeTeamNotices = (*PickupChangeTeamService)(nil)
)

// NewPickupChangeTeam binds the service. It opens no transaction: the store
// joins the caller's school unit (the decision, the student route) and
// refuses to run without one. observe nil records nothing; today nil is the
// school's today.
func NewPickupChangeTeam(
	store ports.PickupChangeTeamStore, reach ports.PickupChangeTeamReach, observe ports.Observer, today func() calendar.Date,
) *PickupChangeTeamService {
	if store == nil || reach == nil {
		panic("care plan pickup change team: store and reach are required")
	}
	if observe == nil {
		observe = func(ports.Observation) {}
	}
	if today == nil {
		today = calendar.TodayDate
	}
	return &PickupChangeTeamService{store: store, reach: reach, observe: observe, today: today}
}

// NotifyPickupChangeApproved stores the markers of the responsible staff and
// enqueues the device notification, both in the decision's transaction: a
// failure of either fails the decision, so no approval is reported that the
// team did not hear about. A day already past is history and stays silent.
func (s *PickupChangeTeamService) NotifyPickupChangeApproved(ctx context.Context, notice careplan.PickupChangeNotice) error {
	today := s.today()
	if notice.StudentID <= 0 || notice.RequestID <= 0 || notice.Date == "" || notice.Date.Before(today) {
		return nil
	}
	if !notice.Withdrawn && notice.PickupTime == "" {
		return nil
	}
	return s.run(ctx, "notify_pickup_change_team", func(txCtx context.Context, stats *domain.OperationStats) error {
		recipients, err := s.reach.ResponsibleAccountIDs(txCtx, notice.StudentID, notice.ReviewerAccountID)
		if err != nil {
			return fmt.Errorf("pickup change team: responsible staff: %w", err)
		}
		recipients = withoutAccount(recipients, notice.ReviewerAccountID)
		if len(recipients) == 0 {
			return nil
		}
		writeStats, err := s.store.UpsertPickupChangeNotices(txCtx, notice.RequestID, notice.Date, recipients)
		stats.Add(writeStats)
		if err != nil {
			return err
		}
		title, body := pickupChangeCopy(notice, today)
		return s.reach.NotifyPickupChanged(txCtx, careplan.PickupChangePush{
			StudentID: notice.StudentID, RequestID: notice.RequestID, Withdrawn: notice.Withdrawn,
			RecipientAccountIDs: recipients, Title: title, Body: body,
			DeepLink: "/students/" + strconv.FormatInt(notice.StudentID, 10),
		})
	})
}

// ListUnreadPickupChangeNotices returns the reader's unread markers.
func (s *PickupChangeTeamService) ListUnreadPickupChangeNotices(ctx context.Context, readerAccountID int64) (result []careplan.PickupChangeTeamNotice, err error) {
	if readerAccountID <= 0 {
		return nil, ErrPickupChangeNoticeInvalid
	}
	err = s.run(ctx, "list_pickup_change_team_notices", func(txCtx context.Context, stats *domain.OperationStats) error {
		var listErr error
		result, listErr = s.unread(txCtx, stats, readerAccountID)
		return listErr
	})
	return result, err
}

// MarkPickupChangeNoticesRead clears the reader's observed markers of one
// child and returns those still unread.
func (s *PickupChangeTeamService) MarkPickupChangeNoticesRead(
	ctx context.Context, readerAccountID, studentID int64, observed []careplan.PickupChangeNoticeRevision,
) (result []careplan.PickupChangeTeamNotice, err error) {
	if readerAccountID <= 0 || studentID <= 0 {
		return nil, ErrPickupChangeNoticeInvalid
	}
	err = s.run(ctx, "mark_pickup_change_team_notices_read", func(txCtx context.Context, stats *domain.OperationStats) error {
		markStats, markErr := s.store.MarkPickupChangeNoticesRead(txCtx, readerAccountID, studentID, observed)
		stats.Add(markStats)
		if markErr != nil {
			return markErr
		}
		var listErr error
		result, listErr = s.unread(txCtx, stats, readerAccountID)
		return listErr
	})
	return result, err
}

func (s *PickupChangeTeamService) unread(ctx context.Context, stats *domain.OperationStats, readerAccountID int64) ([]careplan.PickupChangeTeamNotice, error) {
	rows, listStats, err := s.store.ListUnreadPickupChangeNotices(ctx, readerAccountID, s.today(), MaxPickupChangeTeamNotices)
	stats.Add(listStats)
	if err != nil {
		return nil, err
	}
	out := make([]careplan.PickupChangeTeamNotice, 0, len(rows))
	for _, row := range rows {
		terms := carerequests.StoredPickupTerms(row.Payload)
		if terms == nil || terms.Date == "" {
			continue
		}
		notice := careplan.PickupChangeTeamNotice{
			RequestID: row.RequestID, StudentID: row.StudentID, StudentFirstName: row.StudentFirstName,
			Date: terms.Date, Withdrawn: row.Status != carerequests.StatusApproved, InformedAt: row.InformedAt,
		}
		if !notice.Withdrawn {
			notice.PickupTime = terms.PickupTime
		}
		out = append(out, notice)
	}
	return out, nil
}

// run reports the operation to the observer; the store joins the caller's
// school unit.
func (s *PickupChangeTeamService) run(ctx context.Context, operation string, command func(context.Context, *domain.OperationStats) error) (err error) {
	started := time.Now()
	stats := domain.OperationStats{}
	defer func() {
		s.observe(ports.Observation{Operation: operation, Duration: time.Since(started), Stats: stats, Err: err})
	}()
	return command(ctx, &stats)
}

func withoutAccount(accountIDs []int64, excluded int64) []int64 {
	seen := make(map[int64]bool, len(accountIDs))
	out := make([]int64, 0, len(accountIDs))
	for _, id := range accountIDs {
		if id <= 0 || id == excluded || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// pickupChangeCopy names the day as "heute", "morgen" or "am Fr. 13.11." and
// the new time, or that the regular time applies again.
func pickupChangeCopy(notice careplan.PickupChangeNotice, today calendar.Date) (string, string) {
	day := pickupChangeDay(notice.Date, today)
	if notice.Withdrawn {
		return pickupWithdrawnTitle, fmt.Sprintf(pickupWithdrawnBody, day)
	}
	return pickupChangedTitle, fmt.Sprintf(pickupChangedBody, day, notice.PickupTime)
}

func pickupChangeDay(date, today calendar.Date) string {
	switch date {
	case today:
		return "heute"
	case today.AddDays(1):
		return "morgen"
	}
	return "am " + germanWeekdayShort[date.Weekday()] + " " + date.Format("02.01.")
}
