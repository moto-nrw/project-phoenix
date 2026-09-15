package students_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The request-review golden (#2705) pins the complete wire output of the
// staff request-review reads, the open list, the decision history and the
// pending-count badge, for one fixed fixture: the merged order and its type
// tie-break, keyset paging, every filter, empty pages, malformed queries, the
// permission gates and the narrowing of an absence-only reviewer.
//
// testdata/request_review.golden.json was written by the pre-cutover
// four-service fan-out reader (939126a901) running this same file with
// -update-request-review-golden; without the flag the test holds the current
// reader to that output byte for byte. Synthetic IDs and suffixed fixture
// names are replaced by stable tokens. Every calendar day lies long before or
// long after any plausible run date and the fixture has no weekly-plan
// request, because the pre-cutover reader judged urgency against the wall
// clock; the urgency phases are pinned by the projection's own tests.
var updateRequestReviewGolden = flag.Bool("update-request-review-golden", false,
	"rewrite testdata/request_review.golden.json from the current reader")

const requestReviewGoldenFile = "testdata/request_review.golden.json"

func TestRequestReviewGolden(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	ctx := testpkg.Ctx(t)
	scrub := newGoldenScrubber(t)

	_, adminAccount := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Golden", "Admin")
	leader, leaderAccount := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Golden", "Leader")
	parent := testpkg.CreateTestAccount(t, tc.db, "golden-review-parent@example.test")
	groupA := testpkg.CreateTestEducationGroup(t, tc.db, "GoldenGroupA")
	groupB := testpkg.CreateTestEducationGroup(t, tc.db, "GoldenGroupB")
	testpkg.CreateTestGroupTeacher(t, tc.db, groupA.ID, leader.ID)
	aster := testpkg.CreateTestStudent(t, tc.db, "Golda", "Aster", "2a")
	birke := testpkg.CreateTestStudent(t, tc.db, "Golda", "Birke", "3b")
	testpkg.AssignStudentToGroup(t, tc.db, aster.ID, groupA.ID)
	testpkg.AssignStudentToGroup(t, tc.db, birke.ID, groupB.ID)
	tenantID := aster.TenantID
	booking := setupCorrectionFixture(t, tc, birke.ID, tenantID, "Birke")

	scrub.name("tenant", tenantID, "school")
	scrub.name("account", adminAccount.ID, "admin")
	scrub.name("account", leaderAccount.ID, "leader")
	scrub.name("account", parent.ID, "parent")
	scrub.name("group", groupA.ID, "a")
	scrub.name("group", groupB.ID, "b")
	scrub.text(groupA.Name, "<group-name:a>")
	scrub.text(groupB.Name, "<group-name:b>")
	scrub.name("student", aster.ID, "aster")
	scrub.name("student", birke.ID, "birke")
	scrub.name("person", aster.PersonID, "aster")
	scrub.name("person", birke.PersonID, "birke")
	scrub.name("offering", booking.ganztag.ID, "ganztag")
	scrub.name("offering", booking.mittag.ID, "mittag")
	scrub.text(booking.ganztag.Name, "<offering-name:ganztag>")
	scrub.text(booking.mittag.Name, "<offering-name:mittag>")
	scrub.name("request-child", booking.child.ID, "birke")

	// base is Monday 2026-03-02, 10:00 Berlin. The open rows are submitted
	// after it, the decisions before it: two on Monday, two on Sunday
	// 2026-03-01 for the history date range to cut, one on Saturday.
	base := time.Date(2026, time.March, 2, 9, 0, 0, 0, time.UTC)
	farFuture := timezone.NewDate(2099, time.May, 4)
	longPast := timezone.NewDate(2026, time.January, 12)

	insert := func(model any) {
		t.Helper()
		_, err := tc.db.NewInsert().Model(model).Exec(ctx)
		require.NoError(t, err)
	}

	masterData := func(studentID int64, field, value string, created time.Time) int64 {
		request := &userModels.StudentDataChangeRequest{
			StudentID: studentID, SubmittedBy: parent.ID,
			Target: userModels.DataChangeTargetPerson, FieldKey: field,
			NewValue: json.RawMessage(strconv.Quote(value)), Status: userModels.DataChangeStatusPending,
		}
		request.TenantID = tenantID
		request.CreatedAt, request.UpdatedAt = created, created
		insert(request)
		return request.ID
	}
	care := func(studentID int64, date timezone.Date, status string, created time.Time, decided *time.Time) int64 {
		request := &scheduleModels.CareScheduleChangeRequest{
			StudentID: studentID, SubmittedBy: parent.ID,
			RequestKind: scheduleModels.CareRequestKindPickupChange,
			Payload:     map[string]any{"date": date.String(), "pickup_time": "14:30", "reason": "Arzttermin"},
			Status:      status, ReviewedAt: decided,
		}
		request.TenantID = tenantID
		request.CreatedAt, request.UpdatedAt = created, created
		if decided != nil {
			request.ReviewedBy = &adminAccount.ID
			request.UpdatedAt = *decided
		}
		insert(request)
		return request.ID
	}
	excused := func(studentID int64, dates []timezone.Date, absence, status string, created time.Time, decided *time.Time) int64 {
		request := &activeModels.ExcusedAbsenceRequest{
			StudentID: studentID, SubmittedBy: parent.ID, Dates: dates, Note: "Familienfeier",
			AbsenceStatus: absence, Status: status, ReviewedAt: decided,
		}
		request.TenantID = tenantID
		request.CreatedAt, request.UpdatedAt = created, created
		if decided != nil {
			request.ReviewedBy = &adminAccount.ID
			request.UpdatedAt = *decided
		}
		insert(request)
		return request.ID
	}
	offering := func(status string, effectiveFrom timezone.Date, created time.Time, decided *time.Time) int64 {
		var reviewedBy *int64
		updated := created
		if decided != nil {
			reviewedBy, updated = &adminAccount.ID, *decided
		}
		payload := fmt.Sprintf(`{"offerings":[{"offering_id":%d},{"offering_id":%d}]}`, booking.ganztag.ID, booking.mittag.ID)
		var id int64
		require.NoError(t, tc.db.NewRaw(`INSERT INTO enrollment.offering_change_requests
			(tenant_id, student_id, request_child_id, submitted_by, payload, effective_from, status,
			 reviewed_by, reviewed_at, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?::jsonb, ?, ?, ?, ?, ?, ?) RETURNING id`,
			tenantID, birke.ID, booking.child.ID, parent.ID, payload, effectiveFrom, status,
			reviewedBy, decided, created, updated).Scan(ctx, &id))
		return id
	}
	at := func(offset time.Duration) *time.Time {
		instant := base.Add(offset)
		return &instant
	}

	// Open: six requests over both children and all four queues. The
	// Stammdaten and Betreuungszeiten rows share an instant, so the type
	// tie-break decides their order; one absence lies wholly in the past.
	scrub.name("request:master_data", masterData(aster.ID, "first_name", "Goldi", base.Add(time.Hour)), "open-aster")
	scrub.name("request:master_data", masterData(birke.ID, "last_name", "Birkenfeld", base.Add(2*time.Hour)), "open-birke")
	scrub.name("request:care_schedule", care(aster.ID, farFuture, scheduleModels.CareRequestStatusPending, base.Add(2*time.Hour), nil), "open-aster")
	scrub.name("request:offering", offering("pending", timezone.NewDate(2099, time.August, 3), base.Add(3*time.Hour), nil), "open-birke")
	scrub.name("request:excused", excused(birke.ID, []timezone.Date{farFuture, farFuture.AddDays(1)}, "excused", "pending", base.Add(4*time.Hour), nil), "open-birke")
	scrub.name("request:excused", excused(aster.ID, []timezone.Date{longPast}, "excused", "pending", base.Add(5*time.Hour), nil), "open-aster-past")

	// History: five decisions.
	scrub.name("request:master_data", insertDecidedMasterDataRequest(t, tc, aster.ID, tenantID, adminAccount.ID,
		userModels.DataChangeStatusApproved, base.Add(-time.Hour)).ID, "approved-aster")
	scrub.name("request:master_data", insertDecidedMasterDataRequest(t, tc, birke.ID, tenantID, adminAccount.ID,
		userModels.DataChangeStatusRejected, base.Add(-25*time.Hour)).ID, "rejected-birke")
	scrub.name("request:care_schedule", care(birke.ID, timezone.NewDate(2026, time.January, 20), "rejected",
		base.Add(-3*time.Hour), at(-2*time.Hour)), "rejected-birke")
	scrub.name("request:excused", excused(aster.ID, []timezone.Date{timezone.NewDate(2026, time.January, 5)}, "sick", "approved",
		base.Add(-50*time.Hour), at(-49*time.Hour)), "approved-aster")
	scrub.name("request:offering", offering("rejected", timezone.NewDate(2026, time.February, 2),
		base.Add(-27*time.Hour), at(-26*time.Hour)), "rejected-birke")

	admin := goldenCaller{claims: testutil.AdminTestClaims(int(adminAccount.ID)), perms: []string{"users:read", "users:update"}}
	groupLeader := goldenCaller{claims: testutil.TeacherTestClaims(int(leaderAccount.ID)), perms: []string{"users:read", "users:update"}}
	absenceOnly := goldenCaller{claims: testutil.TeacherTestClaims(int(leaderAccount.ID)), perms: []string{"users:absence"}}
	absenceReviewer := goldenCaller{claims: testutil.AdminTestClaims(int(adminAccount.ID)), perms: []string{"users:read", "users:absence"}}
	readOnly := goldenCaller{claims: testutil.TeacherTestClaims(int(leaderAccount.ID)), perms: []string{"users:read"}}

	rec := &goldenRecorder{t: t, tc: tc, scrub: scrub}
	asterID, birkeID := strconv.FormatInt(aster.ID, 10), strconv.FormatInt(birke.ID, 10)

	rec.pages("open", "view=open&search=Golda&limit=2", admin)
	rec.pages("open-defaults", "search=Golda", admin)
	rec.list("open-excused", "view=open&search=Golda&types=excused", admin)
	rec.list("open-care-and-master-data", "view=open&search=Golda&types=care_schedule,master_data", admin)
	rec.listLabeled("open-student", "view=open&student_id="+asterID, "view=open&student_id=<student:aster>", admin)
	rec.list("open-no-match", "view=open&search=Niemand", admin)
	rec.list("open-corrections-only", "view=open&search=Golda&types=direct_correction", admin)

	rec.pages("history", "view=history&search=Golda&limit=2", admin)
	rec.list("history-rejected", "view=history&search=Golda&status=rejected", admin)
	rec.list("history-approved-or-withdrawn", "view=history&search=Golda&status=approved,withdrawn", admin)
	rec.list("history-sunday", "view=history&search=Golda&from=2026-03-01&to=2026-03-01", admin)
	rec.list("history-excused", "view=history&search=Golda&types=excused", admin)
	rec.listLabeled("history-student", "view=history&student_id="+birkeID, "view=history&student_id=<student:birke>", admin)

	rec.list("invalid-view", "view=everything", admin)
	rec.list("invalid-type", "types=unknown", admin)
	rec.list("invalid-open-status", "view=open&status=approved", admin)
	rec.list("invalid-range", "view=history&from=2026-03-02&to=2026-03-01", admin)
	rec.list("invalid-limit", "limit=0", admin)
	rec.list("invalid-cursor", "view=open&cursor=not-a-cursor", admin)

	rec.count("pending-count", admin)

	rec.list("group-leader/open", "view=open&search=Golda", groupLeader)
	rec.list("group-leader/history", "view=history&search=Golda", groupLeader)
	rec.count("group-leader/pending-count", groupLeader)

	// Without users:update only the excused queue is served (#2232).
	rec.list("absence-reviewer/open", "view=open&search=Golda", absenceReviewer)
	rec.list("absence-reviewer/open-master-data", "view=open&search=Golda&types=master_data", absenceReviewer)
	rec.list("absence-reviewer/history", "view=history&search=Golda", absenceReviewer)
	rec.count("absence-reviewer/pending-count", absenceReviewer)

	rec.list("absence-only/open", "view=open&search=Golda", absenceOnly)
	rec.list("absence-only/history", "view=history&search=Golda", absenceOnly)
	rec.count("absence-only/pending-count", absenceOnly)

	rec.list("read-only/open", "view=open&search=Golda", readOnly)
	rec.count("read-only/pending-count", readOnly)

	// Once the school lets group leaders decide, the leader reviews exactly
	// the children of the group they lead: Golda Aster, not Golda Birke.
	require.NoError(t, tc.resource.SettingsService.SetValue(ctx, configModels.KeyParentRequestGroupLeaderReviewEnabled, true, nil, nil))
	groupAbsenceReviewer := goldenCaller{claims: testutil.TeacherTestClaims(int(leaderAccount.ID)), perms: []string{"users:read", "users:absence"}}
	rec.list("group-leader-enabled/open", "view=open&search=Golda", groupLeader)
	rec.list("group-leader-enabled/history", "view=history&search=Golda", groupLeader)
	rec.count("group-leader-enabled/pending-count", groupLeader)
	rec.list("group-absence-reviewer/open", "view=open&search=Golda", groupAbsenceReviewer)
	rec.count("group-absence-reviewer/pending-count", groupAbsenceReviewer)

	var got bytes.Buffer
	encoder := json.NewEncoder(&got)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	require.NoError(t, encoder.Encode(rec.cases))

	if *updateRequestReviewGolden {
		require.NoError(t, os.MkdirAll("testdata", 0o750))
		require.NoError(t, os.WriteFile(requestReviewGoldenFile, got.Bytes(), 0o600))
		return
	}
	want, err := os.ReadFile(requestReviewGoldenFile)
	require.NoError(t, err)
	require.Equal(t, string(want), got.String(),
		"request-review output drifted from the pre-cutover golden; rerun with -update-request-review-golden only for an agreed contract change")
}

type goldenCaller struct {
	claims jwt.AppClaims
	perms  []string
}

type goldenCase struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Status int    `json:"status"`
	Body   any    `json:"body"`
}

// goldenRecorder runs requests through the production students router and
// collects their scrubbed responses in order.
type goldenRecorder struct {
	t     *testing.T
	tc    *testContext
	scrub *goldenScrubber
	cases []goldenCase
}

func (r *goldenRecorder) list(name, query string, caller goldenCaller) {
	r.listLabeled(name, query, query, caller)
}

func (r *goldenRecorder) listLabeled(name, query, label string, caller goldenCaller) {
	r.get(name, "/change-requests?"+query, "/change-requests?"+label, caller)
}

// count records the pending-count badge as one caller sees it.
func (r *goldenRecorder) count(name string, caller goldenCaller) {
	r.get(name, "/change-requests/pending-count", "/change-requests/pending-count", caller)
}

// pages follows the cursor chain of one query to its last page.
func (r *goldenRecorder) pages(name, query string, caller goldenCaller) {
	path, label := "/change-requests?"+query, "/change-requests?"+query
	for page := 1; ; page++ {
		next := r.get(fmt.Sprintf("%s/page-%d", name, page), path, label, caller)
		if next == "" {
			return
		}
		require.Less(r.t, page, 10, "the cursor chain of %s does not end", name)
		path = "/change-requests?" + query + "&cursor=" + url.QueryEscape(next)
		label = fmt.Sprintf("/change-requests?%s&cursor=<next_cursor of page %d>", query, page)
	}
}

// get records one response and returns its raw next_cursor, if any.
func (r *goldenRecorder) get(name, path, label string, caller goldenCaller) string {
	r.t.Helper()
	rr := authExec(r.t, r.tc, testutil.NewRequest(http.MethodGet, path, nil), caller.claims, caller.perms)
	decoder := json.NewDecoder(bytes.NewReader(rr.Body.Bytes()))
	decoder.UseNumber()
	var body any
	require.NoError(r.t, decoder.Decode(&body), "%s: %s", name, rr.Body.String())
	r.cases = append(r.cases, goldenCase{Name: name, Path: label, Status: rr.Code, Body: r.scrub.value("", "", body)})

	envelope, _ := body.(map[string]any)
	data, _ := envelope["data"].(map[string]any)
	next, _ := data["next_cursor"].(string)
	return next
}

// goldenScrubber replaces the values a fixture cannot fix, synthetic IDs and
// suffixed names, with stable tokens. IDs are read by the key they sit under;
// a request's own id takes the namespace of its request_type, because the
// four queues draw from separate sequences. An ID the fixture did not name
// gets an ordinal token in order of first appearance, so the output stays
// deterministic while the traversal order is.
type goldenScrubber struct {
	t     *testing.T
	named map[string]map[string]string
	seen  map[string]map[string]string
	texts [][2]string
}

func newGoldenScrubber(t *testing.T) *goldenScrubber {
	return &goldenScrubber{t: t, named: map[string]map[string]string{}, seen: map[string]map[string]string{}}
}

func (s *goldenScrubber) name(namespace string, id int64, token string) {
	if s.named[namespace] == nil {
		s.named[namespace] = map[string]string{}
	}
	s.named[namespace][strconv.FormatInt(id, 10)] = "<" + namespace + ":" + token + ">"
}

func (s *goldenScrubber) text(raw, token string) {
	s.texts = append(s.texts, [2]string{raw, token})
}

func (s *goldenScrubber) token(namespace, raw string) string {
	if token, ok := s.named[namespace][raw]; ok {
		return token
	}
	if s.seen[namespace] == nil {
		s.seen[namespace] = map[string]string{}
	}
	if token, ok := s.seen[namespace][raw]; ok {
		return token
	}
	token := fmt.Sprintf("<%s#%d>", namespace, len(s.seen[namespace])+1)
	s.seen[namespace][raw] = token
	return token
}

// value returns v with its synthetic values replaced. key is the JSON key v
// sits under and requestType the request_type of the enclosing item.
func (s *goldenScrubber) value(key, requestType string, v any) any {
	switch typed := v.(type) {
	case map[string]any:
		if kind, ok := typed["request_type"].(string); ok {
			requestType = kind
		}
		keys := make([]string, 0, len(typed))
		for k := range typed {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		out := make(map[string]any, len(typed))
		for _, k := range keys {
			if k == "next_cursor" {
				out[k] = s.cursor(typed[k])
				continue
			}
			out[k] = s.value(k, requestType, typed[k])
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = s.value(key, requestType, item)
		}
		return out
	case json.Number:
		if namespace := goldenIDNamespace(key, requestType); namespace != "" {
			return s.token(namespace, typed.String())
		}
		return typed
	case string:
		if namespace := goldenIDNamespace(key, requestType); namespace != "" && isDecimal(typed) {
			return s.token(namespace, typed)
		}
		// The care request's impact fingerprint is a 64-character digest the
		// retained service produces and the projection passes through.
		// Secret scanners flag such a literal in a committed file.
		if key == "impact_token" {
			return "<impact-token>"
		}
		// The offering conflict key names the offering by its ID.
		if id, ok := strings.CutPrefix(typed, "offer:"); ok && strings.HasPrefix(key, "conflict_key") && isDecimal(id) {
			return "offer:" + s.token("offering", id)
		}
		// The selectable effective-from window is the offering owner's rule
		// over the run date, not a property of the request.
		if key == "earliest_effective_from" || key == "latest_effective_from" {
			return "<bound of the run date>"
		}
		for _, text := range s.texts {
			typed = strings.ReplaceAll(typed, text[0], text[1])
		}
		return typed
	}
	return v
}

// cursor decodes the opaque keyset cursor so the golden shows the position
// per request type instead of a base64 string over synthetic IDs.
func (s *goldenScrubber) cursor(v any) any {
	raw, _ := v.(string)
	if raw == "" {
		return v
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		s.t.Errorf("next_cursor %q is not base64url: %v", raw, err)
		return "<undecodable cursor>"
	}
	var positions map[string]*struct {
		U string      `json:"u"`
		I json.Number `json:"i"`
	}
	decoder := json.NewDecoder(bytes.NewReader(decoded))
	decoder.UseNumber()
	if err := decoder.Decode(&positions); err != nil {
		s.t.Errorf("next_cursor %s is not a position map: %v", decoded, err)
		return "<undecodable cursor>"
	}
	types := make([]string, 0, len(positions))
	for kind := range positions {
		types = append(types, kind)
	}
	slices.Sort(types)
	out := make(map[string]any, len(positions))
	for _, kind := range types {
		position := positions[kind]
		if position == nil {
			out[kind] = nil
			continue
		}
		out[kind] = map[string]any{"u": position.U, "i": s.token("request:"+kind, position.I.String())}
	}
	return out
}

func goldenIDNamespace(key, requestType string) string {
	switch key {
	case "id":
		if requestType != "" {
			return "request:" + requestType
		}
		return "id"
	case "student_id", "student_ids":
		return "student"
	case "group_id":
		return "group"
	case "person_id":
		return "person"
	case "offering_id", "care_offering_id":
		return "offering"
	case "request_child_id":
		return "request-child"
	case "tenant_id":
		return "tenant"
	case "submitted_by", "reviewed_by", "decided_by", "changed_by", "account_id", "actor_account_id":
		return "account"
	}
	if strings.HasSuffix(key, "_id") || strings.HasSuffix(key, "_ids") {
		return key
	}
	return ""
}

func isDecimal(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
