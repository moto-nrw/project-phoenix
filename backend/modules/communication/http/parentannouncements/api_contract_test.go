package announcement_test

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	announcement "github.com/moto-nrw/project-phoenix/modules/communication/http/parentannouncements"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) { testutil.SeedTestJWTConfig(); testpkg.PerTestTenants(); testpkg.Run(m) }

// Only the communication owner is replaced. Requests use the registered
// router, authentication, permission gate and tenant transaction middleware.
type announcementContract struct {
	communication.ParentAnnouncementCapability
	t        *testing.T
	id       int64
	tenantID int64
	actorID  int64
	failure  error
	calls    []string
	row      communication.ParentAnnouncement
	input    communication.ParentAnnouncementInput
	reminder communication.ParentAnnouncementReminderInput
}

func (s *announcementContract) call(ctx context.Context, name string, id int64) error {
	s.t.Helper()
	require.Equal(s.t, s.id, id)
	require.Equal(s.t, s.tenantID, tenant.FromContext(ctx))
	s.calls = append(s.calls, name)
	return s.failure
}
func (s *announcementContract) ListParentAnnouncements(c context.Context, all bool) ([]communication.ParentAnnouncement, error) {
	require.True(s.t, all)
	err := s.call(c, "list", s.id)
	return []communication.ParentAnnouncement{s.row}, err
}
func (s *announcementContract) GetParentAnnouncement(c context.Context, id int64) (*communication.ParentAnnouncement, error) {
	return &s.row, s.call(c, "get", id)
}
func (s *announcementContract) CreateParentAnnouncement(c context.Context, id int64, v communication.ParentAnnouncementInput) (*communication.ParentAnnouncement, error) {
	require.Equal(s.t, s.actorID, id)
	s.input = v
	return &s.row, s.call(c, "create", s.id)
}
func (s *announcementContract) UpdateParentAnnouncement(c context.Context, id int64, v communication.ParentAnnouncementInput) (*communication.ParentAnnouncement, error) {
	s.input = v
	return &s.row, s.call(c, "update", id)
}
func (s *announcementContract) DeleteParentAnnouncement(c context.Context, id int64) error {
	return s.call(c, "delete", id)
}
func (s *announcementContract) PublishParentAnnouncement(c context.Context, id int64) (*communication.ParentAnnouncement, error) {
	return &s.row, s.call(c, "publish", id)
}
func (s *announcementContract) UnpublishParentAnnouncement(c context.Context, id int64) (*communication.ParentAnnouncement, error) {
	return &s.row, s.call(c, "unpublish", id)
}
func (s *announcementContract) ParentAnnouncementStats(c context.Context, id int64) (*communication.ParentAnnouncementStats, error) {
	return &communication.ParentAnnouncementStats{TargetCount: 3, ReadCount: 2, AcknowledgedCount: 1}, s.call(c, "stats", id)
}
func (s *announcementContract) ParentAnnouncementRecipients(c context.Context, id int64) ([]communication.ParentAnnouncementRecipient, error) {
	now := time.Date(2026, 9, 21, 9, 0, 0, 0, time.UTC)
	return []communication.ParentAnnouncementRecipient{{AccountID: id}, {AccountID: id, ReadAt: &now}, {AccountID: id, ReadAt: &now, AcknowledgedAt: &now}}, s.call(c, "recipients", id)
}
func (s *announcementContract) ParentAnnouncementPollResults(c context.Context, id int64) (*communication.ParentAnnouncementPollResults, error) {
	return &communication.ParentAnnouncementPollResults{TargetChildCount: 3, AnsweredCount: 1, Options: []communication.ParentAnnouncementPollOption{{OptionID: id, Label: "Ja", Count: 1}}}, s.call(c, "poll-results", id)
}
func (s *announcementContract) ParentAnnouncementPollChildren(c context.Context, id int64) ([]communication.ParentAnnouncementPollChild, error) {
	return []communication.ParentAnnouncementPollChild{{StudentID: id, CanAnswer: true}, {StudentID: id, AnswerLabels: []string{"Ja"}}}, s.call(c, "poll-children", id)
}
func (s *announcementContract) RemindParentAnnouncement(c context.Context, id int64) (int, error) {
	return 2, s.call(c, "remind", id)
}
func (s *announcementContract) ResendParentAnnouncementEmails(c context.Context, id int64) (int, error) {
	return 3, s.call(c, "resend-failed", id)
}
func (s *announcementContract) UpdateParentAnnouncementReminder(c context.Context, id int64, v communication.ParentAnnouncementReminderInput) (*communication.ParentAnnouncement, error) {
	s.reminder = v
	row := s.row
	row.ReminderAt = v.ReminderAt
	row.ReminderText = v.ReminderText
	return &row, s.call(c, "reminder", id)
}
func (s *announcementContract) ParentAnnouncementLetterStatus(c context.Context, id int64) (*communication.ParentAnnouncementLetterStatus, error) {
	now := time.Date(2026, 9, 21, 9, 0, 0, 0, time.UTC)
	return &communication.ParentAnnouncementLetterStatus{Recipients: []communication.ParentAnnouncementLetterRecipient{{EmailStatus: "failed", Reachability: "email"}}, Children: []communication.ParentAnnouncementLetterChild{{StudentID: id, CanConfirm: true}, {StudentID: id, AcknowledgedAt: &now, AckFirstName: "Ada", AckLastName: "Lovelace"}}, Summary: communication.ParentAnnouncementLetterSummary{ChildrenTotal: 2, ChildrenFulfilled: 1}}, s.call(c, "letter-status", id)
}

func announcementRoute(t *testing.T) (*announcementContract, *announcement.Resource, string) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	staff := testpkg.CreateTestStaff(t, db, "Announcement", "Contract")
	claims := testutil.DefaultTestClaims()
	claims.Permissions = []string{permissions.AdminWildcard}
	claims.IsAdmin = true
	s := &announcementContract{t: t, id: staff.ID, tenantID: testpkg.Tenant(t), actorID: int64(claims.ID), row: communication.ParentAnnouncement{ID: staff.ID, Title: "Information", Body: "Text", Active: true, Targets: []communication.ParentAnnouncementTarget{{TargetType: "student", RefID: &staff.ID}}, Options: []communication.ParentAnnouncementOption{{ID: staff.ID, Label: "Ja"}}}}
	return s, announcement.NewResource(s, db), testutil.MintTestJWT(t, claims)
}
func announcementRequest(t *testing.T, r *announcement.Resource, token, method, path string, body any) (int, string) {
	t.Helper()
	req := testutil.NewAuthenticatedRequest(t, method, path, body, testutil.WithJWTBearer(token))
	out := testutil.ExecuteRequest(r.Router(), req)
	return out.Code, out.Body.String()
}

func TestAnnouncementRoutesPreserveAuthoringAndResponseContracts(t *testing.T) {
	t.Parallel()
	s, r, token := announcementRoute(t)
	id := strconv.FormatInt(s.id, 10)
	input := map[string]any{"title": "Information", "body": "Text", "targets": []map[string]any{{"target_type": "student", "ref_id": id}}, "expires_at": "2026-10-01T12:00:00Z", "response_deadline": "2026-09-30T12:00:00Z", "options": []string{"Ja"}}
	for _, tc := range []struct {
		method, path, call, fragment string
		status                       int
		body                         any
	}{
		{"GET", "/?include_inactive=true", "list", `"title":"Information"`, 200, nil},
		{"GET", "/" + id, "get", `"id":"` + id + `"`, 200, nil},
		{"POST", "/", "create", `"ref_id":"` + id + `"`, 201, input},
		{"PUT", "/" + id, "update", `"label":"Ja"`, 200, input},
		{"DELETE", "/" + id, "delete", `"deleted":true`, 200, nil},
		{"POST", "/" + id + "/publish", "publish", `"title":"Information"`, 200, nil},
		{"POST", "/" + id + "/unpublish", "unpublish", `"title":"Information"`, 200, nil},
		{"GET", "/" + id + "/stats", "stats", `"acknowledged_count":1`, 200, nil},
		{"GET", "/" + id + "/recipients", "recipients", `"status":"acknowledged"`, 200, nil},
		{"GET", "/" + id + "/poll-results", "poll-results", `"option_id":"` + id + `"`, 200, nil},
		{"GET", "/" + id + "/poll-children", "poll-children", `"answer_labels":[]`, 200, nil},
		{"GET", "/" + id + "/letter-status", "letter-status", `"acknowledged_by":"Ada Lovelace"`, 200, nil},
		{"POST", "/" + id + "/remind", "remind", `"reminded_count":2`, 200, nil},
		{"POST", "/" + id + "/resend-failed", "resend-failed", `"resent_count":3`, 200, nil},
		{"PUT", "/" + id + "/reminder", "reminder", `"reminder_at":"2026-09-24T06:00:00Z"`, 200, map[string]any{"reminder_at": "2026-09-24T06:00:00Z", "reminder_text": "Morgen um 13:00 Uhr."}},
	} {
		t.Run(tc.call, func(t *testing.T) {
			before := len(s.calls)
			status, body := announcementRequest(t, r, token, tc.method, tc.path, tc.body)
			require.Equal(t, tc.status, status, body)
			require.Contains(t, body, tc.fragment)
			require.Equal(t, []string{tc.call}, s.calls[before:])
		})
	}
	require.Equal(t, s.id, *s.input.Targets[0].RefID)
	require.Equal(t, "2026-10-01T12:00:00Z", s.input.ExpiresAt.Format(time.RFC3339))
	require.Equal(t, []string{"Ja"}, s.input.Options)
}

func TestAnnouncementRoutesRejectUnauthorisedAndMalformedRequests(t *testing.T) {
	t.Parallel()
	s, r, token := announcementRoute(t)
	id := strconv.FormatInt(s.id, 10)
	claims := testutil.DefaultTestClaims()
	claims.IsAdmin = false
	claims.Permissions = []string{permissions.CommunicationsAnnounce}
	limited := testutil.MintTestJWT(t, claims)
	for _, tc := range []struct{ method, path string }{{"GET", "/" + id}, {"POST", "/"}, {"PUT", "/" + id}, {"DELETE", "/" + id}, {"POST", "/" + id + "/publish"}, {"POST", "/" + id + "/unpublish"}, {"GET", "/" + id + "/recipients"}, {"GET", "/" + id + "/stats"}, {"GET", "/" + id + "/poll-results"}, {"GET", "/" + id + "/poll-children"}, {"GET", "/" + id + "/letter-status"}, {"POST", "/" + id + "/remind"}, {"POST", "/" + id + "/resend-failed"}, {"PUT", "/" + id + "/reminder"}} {
		status, body := announcementRequest(t, r, limited, tc.method, tc.path, nil)
		require.Equal(t, 403, status, body)
		if tc.path != "/" {
			status, body = announcementRequest(t, r, token, tc.method, "/bad"+tc.path[len(id)+1:], nil)
			require.Equal(t, 400, status, body)
		}
	}
	for _, method := range []string{"POST", "PUT"} {
		path := "/"
		if method == "PUT" {
			path += id
		}
		for _, body := range []any{nil, map[string]any{"expires_at": "bad"}, map[string]any{"response_deadline": "bad"}, map[string]any{"targets": []map[string]any{{"ref_id": "bad"}}}} {
			status, out := announcementRequest(t, r, token, method, path, body)
			require.Equal(t, 400, status, out)
		}
	}
	require.Empty(t, s.calls)
}

func TestAnnouncementRoutesKeepServiceFailuresAndSystemImmutability(t *testing.T) {
	t.Parallel()
	s, r, token := announcementRoute(t)
	id := strconv.FormatInt(s.id, 10)
	for _, tc := range []struct {
		failure error
		status  int
		code    string
	}{
		{communication.ErrParentAnnouncementNotFound, 404, ""}, {communication.ErrParentNewsDisabled, 403, "parent_news_disabled"}, {communication.ErrPublishedParentAnnouncement, 409, "announcement_published_immutable"}, {communication.ErrSystemParentAnnouncementImmutable, 409, "system_announcement_immutable"}, {communication.ErrParentAnnouncementNotPoll, 400, ""}, {communication.ErrParentAnnouncementPollClosed, 409, "poll_not_open"}, {communication.ErrParentAnnouncementNotPublished, 409, "announcement_not_published"}, {communication.ErrParentAnnouncementNothingDue, 400, ""}, {communication.ErrParentAnnouncementValidation, 400, ""}, {communication.ErrParentAnnouncementReminderSent, 409, "announcement_reminder_sent"}, {errors.New("database unavailable"), 500, ""},
	} {
		s.failure = tc.failure
		status, body := announcementRequest(t, r, token, "DELETE", "/"+id, nil)
		require.Equal(t, tc.status, status, body)
		if tc.code != "" {
			require.Contains(t, body, tc.code)
		}
	}
	s.failure = communication.ErrParentAnnouncementNotFound
	for _, tc := range []struct{ method, path string }{{"GET", "/?include_inactive=true"}, {"GET", "/" + id}, {"POST", "/"}, {"PUT", "/" + id}, {"POST", "/" + id + "/publish"}, {"POST", "/" + id + "/unpublish"}, {"GET", "/" + id + "/stats"}, {"GET", "/" + id + "/recipients"}, {"GET", "/" + id + "/poll-results"}, {"GET", "/" + id + "/poll-children"}, {"GET", "/" + id + "/letter-status"}, {"POST", "/" + id + "/remind"}, {"POST", "/" + id + "/resend-failed"}, {"PUT", "/" + id + "/reminder"}} {
		status, body := announcementRequest(t, r, token, tc.method, tc.path, map[string]any{"title": "Info", "body": "Text"})
		require.Equal(t, 404, status, body)
	}
}
