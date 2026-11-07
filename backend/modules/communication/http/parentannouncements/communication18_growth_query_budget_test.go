package announcement_test

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	announcement "github.com/moto-nrw/project-phoenix/modules/communication/http/parentannouncements"
	"github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	"github.com/moto-nrw/project-phoenix/modules/delivery/objects"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/roots/communicationtest"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

func communication18JSON(t *testing.T, body []byte) any {
	t.Helper()
	var out struct {
		Data any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &out))
	return out.Data
}
func communication18Equal(t *testing.T, want any, got any) {
	t.Helper()
	b, err := json.Marshal(want)
	require.NoError(t, err)
	var normalized any
	require.NoError(t, json.Unmarshal(b, &normalized))
	communication18Diff(t, "data", normalized, got)
}

func communication18Diff(t *testing.T, path string, want, got any) {
	t.Helper()
	if reflect.DeepEqual(want, got) {
		return
	}
	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		require.True(t, ok, path)
		keys := []string{}
		for k := range w {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v, ok := g[k]
			require.True(t, ok, path+" missing "+k)
			communication18Diff(t, path+"."+k, w[k], v)
		}
		require.Len(t, g, len(w), path+" unexpected keys")
	case []any:
		g, ok := got.([]any)
		require.True(t, ok, path)
		require.Len(t, g, len(w), path)
		for i := range w {
			communication18Diff(t, fmt.Sprintf("%s[%d]", path, i), w[i], g[i])
		}
	default:
		// The API renders stored instants in the database session zone.
		if wantText, ok := want.(string); ok {
			if gotText, ok := got.(string); ok {
				wantTime, wantErr := time.Parse(time.RFC3339Nano, wantText)
				gotTime, gotErr := time.Parse(time.RFC3339Nano, gotText)
				if wantErr == nil && gotErr == nil {
					require.True(t, wantTime.Equal(gotTime), "%s: %s is not the instant %s", path, gotText, wantText)
					return
				}
			}
		}
		require.Equal(t, want, got, path)
	}
}

func communication18Announcement(a *communication.ParentAnnouncement) map[string]any {
	targets := []any{}
	for _, x := range a.Targets {
		r := map[string]any{"target_type": x.TargetType}
		if x.RefID != nil {
			r["ref_id"] = fmt.Sprint(*x.RefID)
		}
		if x.RefText != nil {
			r["ref_text"] = *x.RefText
		}
		targets = append(targets, r)
	}
	options := []any{}
	for _, x := range a.Options {
		options = append(options, map[string]any{"id": fmt.Sprint(x.ID), "label": x.Label})
	}
	status := "draft"
	if a.PublishedAt != nil {
		status = "published"
	}
	out := map[string]any{"id": fmt.Sprint(a.ID), "title": a.Title, "body": a.Body, "priority": a.Priority, "requires_acknowledgement": a.RequiresAcknowledgement, "send_email": a.SendEmail, "status": status, "active": a.Active, "created_at": a.CreatedAt, "updated_at": a.UpdatedAt, "targets": targets, "response_type": a.ResponseType, "options": options, "delivery_mode": a.DeliveryMode, "email_audience": a.EmailAudience}
	if a.PublishedAt != nil {
		out["published_at"] = a.PublishedAt
	}
	if a.Sender != nil {
		out["sender_name"] = a.Sender.Name
		out["sender_school_class"] = a.Sender.SchoolClass
	}
	if a.Declaration.Kind != "" {
		out["declaration_kind"] = a.Declaration.Kind
		out["declaration_signers"] = a.Declaration.Signers
		out["declaration_revocable"] = a.Declaration.Revocable
		out["declaration_requires_password"] = a.Declaration.RequiresPassword
		out["declaration_requires_signature"] = a.Declaration.RequiresSignature
		out["declaration_locked_attachments"] = a.DeclarationFrozen
	}
	return out
}

func TestCommunication18AnnouncementPositiveGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	// Each cardinality owns a tenant, including its active parent/account links.
	for _, n := range []int{3, 8} {
		t.Run(fmt.Sprintf("N%d", n), func(t *testing.T) {
			testpkg.OwnTenant(t)
			db := testpkg.SetupTestDB(t)
			database := testpkg.NewDatabase(t, db)
			ctx := testpkg.Ctx(t)
			organizations, err := owners.NewOrganizationTenancy(database)
			require.NoError(t, err)
			repos := api.NewParentMessagingSuiteRepositories(database).WithSchoolProjections(organizations)
			settings, err := owners.NewSettingsTestModule(database, testpkg.TenantRuntime(t, db))
			require.NoError(t, err)
			require.NoError(t, settings.Settings.SetValue(ctx, "operations.parent_news_enabled", true, nil, nil))
			objects, err := objects.PrivateUploadsBackend()
			require.NoError(t, err)
			files, _, err := api.NewFileStoreTestModule(database, testpkg.TenantRuntime(t, db), objects)
			require.NoError(t, err)
			delivery, err := api.NewDeliveryTestModule(database, testpkg.TenantRuntime(t, db))
			require.NoError(t, err)
			service, err := owners.BuildParentAnnouncements(owners.RootEnvironment{Logger: slog.Default(), Links: owners.ServiceLinks{ParentsURL: "http://localhost"}}, settings.Settings, delivery.EmailOutbox, delivery.Runtime.Module, delivery.Runtime.EmailOutcomes, owners.DeliveryNotifications{Notifier: delivery.Notifications, Preferences: delivery.NotificationPreferences}, repos.MessagingRepositories, files)
			require.NoError(t, err)
			_, viewer := testpkg.CreateTestStaffWithAccount(t, db, "Teacher", "Growth")
			token := testutil.MintTestJWT(t, sessiontoken.AppClaims{ID: int(viewer.ID), Sub: viewer.Email, TenantID: testpkg.Tenant(t), Roles: []string{"user"}, Permissions: []string{permissions.AdminWildcard}})
			schoolToken := testutil.MintTestJWT(t, sessiontoken.AppClaims{ID: int(viewer.ID), Sub: viewer.Email, TenantID: testpkg.Tenant(t), Scope: "school", Roles: []string{"user"}, Permissions: []string{permissions.ClassDayParentLetterWrite}})
			foreignTenant, _ := testpkg.CreateTestTenant(t, db)
			_, foreign := testpkg.CreateTestStaffWithAccountForTenant(t, db, foreignTenant, "Foreign", "Growth")
			foreignToken := testutil.MintTestJWT(t, sessiontoken.AppClaims{ID: int(foreign.ID), Sub: foreign.Email, TenantID: foreignTenant, Roles: []string{"user"}, Permissions: []string{permissions.AdminWildcard}})
			foreignSchool := testutil.MintTestJWT(t, sessiontoken.AppClaims{ID: int(foreign.ID), Sub: foreign.Email, TenantID: foreignTenant, Scope: "school", Roles: []string{"user"}, Permissions: []string{permissions.ClassDayParentLetterWrite}})
			chains := make([]testpkg.ParentChain, 0, n)
			for i := 0; i < n; i++ {
				c := testpkg.CreateTestParentGuardianChain(t, db)
				chains = append(chains, c)
				_, err = db.NewRaw(`UPDATE users.persons SET first_name = ?, last_name = 'Growth' WHERE id = ?`, fmt.Sprintf("Child%02d", i), c.PersonID).Exec(ctx)
				require.NoError(t, err)
				_, err = db.NewRaw(`UPDATE users.guardian_profiles SET first_name = ?, last_name = 'Growth' WHERE id = ?`, fmt.Sprintf("Guardian%02d", i), c.GuardianProfileID).Exec(ctx)
				require.NoError(t, err)
			}
			// A foreign child and announcement also exist: neither may enter local DTOs.
			foreignStudent := testpkg.CreateTestStudentForTenant(t, db, foreignTenant, "Foreign", "Child", "1a")
			var foreignAnnouncementID int64
			require.NoError(t, testpkg.WithinTenantContext(t, tenant.WithTenantID(ctx, foreignTenant), db, foreignTenant, func(c context.Context) error {
				a := &communication.PortalParentAnnouncement{TenantID: foreignTenant, CreatedBy: foreign.ID, Title: "Foreign announcement", Body: "Foreign body", Priority: "info", Active: true}
				if e := repos.ParentAnnouncement.Create(c, a); e != nil {
					return e
				}
				foreignAnnouncementID = a.ID
				id := foreignStudent.ID
				return repos.ParentAnnouncement.ReplaceTargets(c, foreignTenant, a.ID, []*communication.PortalParentAnnouncementTarget{{TargetType: "student", TargetRefID: &id}})
			}))
			var foreignAnnouncement *communication.ParentAnnouncement
			require.NoError(t, testpkg.WithinTenantContext(t, tenant.WithTenantID(ctx, foreignTenant), db, foreignTenant, func(c context.Context) error {
				var e error
				foreignAnnouncement, e = service.GetParentAnnouncement(c, foreignAnnouncementID)
				return e
			}))
			run := func(fn func(context.Context) error) {
				require.NoError(t, testpkg.WithinTenantContext(t, ctx, db, testpkg.Tenant(t), fn))
			}
			var poll, letter, declaration *communication.ParentAnnouncement
			var notices []*communication.ParentAnnouncement
			class := "1a"
			run(func(c context.Context) error {
				for i := 0; i < n-2; i++ {
					in := communication.ParentAnnouncementInput{Title: fmt.Sprintf("Notice%02d", i), Body: "Exact body", Targets: []communication.ParentAnnouncementTargetInput{}, ResponseType: "single_choice"}
					for _, chain := range chains {
						id := chain.StudentID
						in.Targets = append(in.Targets, communication.ParentAnnouncementTargetInput{TargetType: "student", RefID: &id})
					}
					for j := 0; j < n; j++ {
						in.Options = append(in.Options, fmt.Sprintf("Option%02d", j))
					}
					a, e := service.CreateParentAnnouncement(c, viewer.ID, in)
					if e != nil {
						return e
					}
					a.CreatedAt = time.Date(2026, 8, 1, 8, i, 0, 0, time.UTC)

					notices = append(notices, a)
				}
				poll = notices[0]
				var e error
				letter, e = service.CreateParentAnnouncement(communication.WithClassLetterAuthor(c, viewer.ID), viewer.ID, communication.ParentAnnouncementInput{Title: "Class letter", Body: "Letter body", DeliveryMode: "letter", Targets: []communication.ParentAnnouncementTargetInput{{TargetType: "class", RefText: &class}}, Sender: &communication.ParentLetterSender{Name: "Teacher Growth", SchoolClass: class}})
				if e != nil {
					return e
				}
				declaration, e = service.CreateParentAnnouncement(communication.WithClassLetterAuthor(c, viewer.ID), viewer.ID, communication.ParentAnnouncementInput{Title: "Declaration", Body: "Declaration body", DeliveryMode: "declaration", Declaration: communication.ParentDeclarationSettings{Kind: "consent", Signers: "any", Revocable: true}, Targets: []communication.ParentAnnouncementTargetInput{{TargetType: "class", RefText: &class}}, Sender: &communication.ParentLetterSender{Name: "Teacher Growth", SchoolClass: class}})
				return e
			})
			// Publication is fixture state, not the measured operation. Keep outbox
			// transport dormant; the read graph still binds the real delivery owner.
			for _, a := range notices {
				_, err = db.NewRaw(`UPDATE users.parent_announcements SET created_at = ? WHERE id = ?`, a.CreatedAt, a.ID).Exec(ctx)
				require.NoError(t, err)
			}
			for i, a := range []*communication.ParentAnnouncement{letter, declaration} {
				a.CreatedAt = time.Date(2026, 8, 1, 9, i, 0, 0, time.UTC)
				_, err = db.NewRaw(`UPDATE users.parent_announcements SET created_at = ? WHERE id = ?`, a.CreatedAt, a.ID).Exec(ctx)
				require.NoError(t, err)
			}
			testpkg.PublishTestParentAnnouncement(t, db, poll.ID)
			run(func(c context.Context) error {
				_, e := service.PublishParentAnnouncement(communication.WithClassLetterAuthor(c, viewer.ID), letter.ID)
				return e
			})
			testpkg.PublishTestParentAnnouncement(t, db, declaration.ID)
			run(func(c context.Context) error {
				var e error
				for i, a := range notices {
					notices[i], e = service.GetParentAnnouncement(c, a.ID)
					if e != nil {
						return e
					}
				}
				poll, e = service.GetParentAnnouncement(c, poll.ID)
				if e != nil {
					return e
				}
				notices[0] = poll
				letter, e = service.GetParentAnnouncement(c, letter.ID)
				return e
			})
			responseStamp := time.Date(2026, 8, 2, 11, 0, 0, 0, time.UTC).In(time.Local)
			run(func(c context.Context) error {
				for i, chain := range chains {
					ok, e := repos.ParentAnnouncement.SetResponse(c, testpkg.Tenant(t), poll.ID, chain.StudentID, chain.AccountID, []int64{poll.Options[i].ID}, *poll.PublishedAt)
					if e != nil {
						return e
					}
					require.True(t, ok)
				}
				return nil
			})
			_, err = db.NewRaw(`UPDATE users.parent_announcement_responses SET responded_at = ? WHERE announcement_id = ?`, responseStamp, poll.ID).Exec(ctx)
			require.NoError(t, err)
			var versions []*communication.PortalDeclarationVersion
			var submissions []*communication.PortalDeclarationSubmission
			policy := parentstore.NewPortalDeclarationPolicy()
			stamp := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC).In(time.Local)
			run(func(c context.Context) error {
				for i := 0; i < n; i++ {
					v := &communication.PortalDeclarationVersion{TenantID: testpkg.Tenant(t), AnnouncementID: declaration.ID, VersionNo: i + 1, Title: "Declaration", Body: fmt.Sprintf("Version%02d", i), Kind: "consent", Attachments: []communication.PortalDeclarationAttachmentDigest{}, PublishedAt: stamp.Add(time.Duration(i) * time.Hour)}
					v.ContentHash = communicationtest.Fingerprint(policy.PortalCanonicalContent(*v))
					if e := repos.ParentAnnouncement.InsertDeclarationVersion(c, v); e != nil {
						return e
					}
					versions = append(versions, v)
				}
				v := versions[n-1]
				for i, chain := range chains {
					account, guardian := chain.AccountID, chain.GuardianProfileID
					s := &communication.PortalDeclarationSubmission{TenantID: testpkg.Tenant(t), AnnouncementID: declaration.ID, VersionID: v.ID, VersionNo: n, StudentID: chain.StudentID, AccountID: &account, GuardianProfileID: &guardian, SignerName: fmt.Sprintf("Guardian%02d Growth", i), GuardianRole: "primary_guardian", Action: "agreed", Method: "simple_electronic", ContentHash: v.ContentHash, SubmittedAt: stamp.Add(24*time.Hour + time.Duration(i)*time.Minute)}
					s.RecordHash = communicationtest.Fingerprint(policy.PortalCanonicalRecord(*s))
					if e := repos.ParentAnnouncement.InsertDeclarationSubmission(c, s); e != nil {
						return e
					}
					submissions = append(submissions, s)
				}
				var e error
				declaration, e = service.GetParentAnnouncement(c, declaration.ID)
				return e
			})
			resource := announcement.NewResource(service, nil)
			router := chi.NewRouter()
			router.Use(testpkg.TenantRuntimeMiddleware(t, db))
			router.Mount("/api/parent-announcements", resource.Router(api.ProtectedTenantRoutes))
			router.Mount("/school/parent-letters", resource.SchoolRouter(api.ProtectedSchoolRoutes))
			counter := testpkg.CaptureQueriesForContext(t, db)
			counter.Stop()
			request := func(path, bearer string) *httptest.ResponseRecorder {
				req := httptest.NewRequest("GET", path, nil).WithContext(counter.Context(ctx))
				if bearer != "" {
					req.Header.Set("Authorization", "Bearer "+bearer)
				}
				return testutil.ExecuteRequestForTest(t, router, req)
			}
			type routeCase struct {
				name, path string
				school     bool
				row        *communication.ParentAnnouncement
			}
			cases := []routeCase{{"list", "/api/parent-announcements/", false, nil}, {"get", fmt.Sprintf("/api/parent-announcements/%d", poll.ID), false, poll}, {"recipients", fmt.Sprintf("/api/parent-announcements/%d/recipients", poll.ID), false, poll}, {"poll_children", fmt.Sprintf("/api/parent-announcements/%d/poll-children", poll.ID), false, poll}, {"poll_results", fmt.Sprintf("/api/parent-announcements/%d/poll-results", poll.ID), false, poll}, {"letter_status", fmt.Sprintf("/api/parent-announcements/%d/letter-status", letter.ID), false, letter}, {"declaration_status", fmt.Sprintf("/api/parent-announcements/%d/declaration-status", declaration.ID), false, declaration}, {"declaration_export", fmt.Sprintf("/api/parent-announcements/%d/declaration-export", declaration.ID), false, declaration}}
			for _, suffix := range []string{"get", "recipients", "letter_status", "declaration_status", "declaration_export"} {
				row := letter
				if strings.HasPrefix(suffix, "declaration") {
					row = declaration
				}
				tail := "/" + strings.ReplaceAll(suffix, "_", "-")
				if suffix == "get" {
					tail = ""
				}
				cases = append(cases, routeCase{"school_" + suffix, fmt.Sprintf("/school/parent-letters/%d%s", row.ID, tail), true, row})
			}
			for _, tc := range cases {
				bearer, other := token, foreignToken
				if tc.school {
					bearer, other = schoolToken, foreignSchool
				}
				counter.Reset()
				counter.Start()
				rec := request(tc.path, bearer)
				counter.Stop()
				require.Equal(t, http.StatusOK, rec.Code, "%s: %s", tc.name, rec.Body.String())
				switch tc.name {
				case "list":
					listDeclaration := *declaration
					listDeclaration.DeclarationFrozen = false
					want := []any{communication18Announcement(&listDeclaration), communication18Announcement(letter)}
					for i := len(notices) - 1; i >= 0; i-- {
						want = append(want, communication18Announcement(notices[i]))
					}
					communication18Equal(t, want, communication18JSON(t, rec.Body.Bytes()))
				case "get", "school_get":
					communication18Equal(t, communication18Announcement(tc.row), communication18JSON(t, rec.Body.Bytes()))
				case "recipients", "school_recipients":
					want := []any{}
					for i, c := range chains {
						want = append(want, map[string]any{"account_id": fmt.Sprint(c.AccountID), "first_name": fmt.Sprintf("Guardian%02d", i), "last_name": "Growth", "status": "pending"})
					}
					communication18Equal(t, want, communication18JSON(t, rec.Body.Bytes()))
				case "poll_children":
					want := []any{}
					for i, c := range chains {
						want = append(want, map[string]any{"student_id": fmt.Sprint(c.StudentID), "first_name": fmt.Sprintf("Child%02d", i), "last_name": "Growth", "school_class": "1a", "answer_labels": []string{fmt.Sprintf("Option%02d", i)}, "responded_at": responseStamp, "can_answer": true})
					}
					communication18Equal(t, want, communication18JSON(t, rec.Body.Bytes()))
				case "poll_results":
					options := []any{}
					for _, o := range poll.Options {
						options = append(options, map[string]any{"option_id": fmt.Sprint(o.ID), "label": o.Label, "count": 1})
					}
					communication18Equal(t, map[string]any{"child_count": n, "target_child_count": n, "answered_count": n, "options": options}, communication18JSON(t, rec.Body.Bytes()))
				case "letter_status", "school_letter_status":
					children := []any{}
					for i, c := range chains {
						children = append(children, map[string]any{"student_id": fmt.Sprint(c.StudentID), "first_name": fmt.Sprintf("Child%02d", i), "last_name": "Growth", "school_class": "1a", "fulfilled": false, "can_confirm": true})
					}
					recipients := []any{}
					for i, c := range chains {
						recipients = append(recipients, map[string]any{"first_name": fmt.Sprintf("Guardian%02d", i), "last_name": "Growth", "email": c.Email, "email_status": "pending", "mail_outcome": "queued", "mailed_to": c.Email, "reachability": "ok"})
					}
					communication18Equal(t, map[string]any{"recipients": recipients, "children": children, "summary": communication.ParentAnnouncementLetterSummary{ChildrenTotal: n, ChildrenConfirmable: n, ChildrenOpen: n, RecipientsTotal: n, EmailsPending: n}}, communication18JSON(t, rec.Body.Bytes()))
				case "declaration_status", "school_declaration_status":
					communication18Equal(t, communication18Declaration(n, chains, versions, submissions), communication18JSON(t, rec.Body.Bytes()))
				case "declaration_export", "school_declaration_export":
					communication18CSV(t, rec, n, submissions)
				}
				require.Zero(t, counter.WriteRows())
				communication18Budget(t, tc.name, n, counter.Queries())
				unauth := request(tc.path, "")
				require.Equal(t, 401, unauth.Code, tc.name)
				denied := request(tc.path, other)
				if tc.name == "list" {
					require.Equal(t, 200, denied.Code)
					communication18Equal(t, []any{communication18Announcement(foreignAnnouncement)}, communication18JSON(t, denied.Body.Bytes()))
				} else {
					require.Equal(t, 404, denied.Code, "%s: %s", tc.name, denied.Body.String())
				}
			}
		})
	}
}

func communication18Declaration(n int, chains []testpkg.ParentChain, versions []*communication.PortalDeclarationVersion, subs []*communication.PortalDeclarationSubmission) map[string]any {
	vs := []any{}
	for i := len(versions) - 1; i >= 0; i-- {
		v := versions[i]
		vs = append(vs, map[string]any{"id": fmt.Sprint(v.ID), "version_no": v.VersionNo, "title": v.Title, "body": v.Body, "content_hash": v.ContentHash, "published_at": v.PublishedAt, "attachments": []any{}, "integrity_ok": true})
	}
	children := []any{}
	for i, c := range chains {
		s := subs[i]
		children = append(children, map[string]any{"student_id": fmt.Sprint(c.StudentID), "first_name": fmt.Sprintf("Child%02d", i), "last_name": "Growth", "school_class": "1a", "state": "agreed", "signers": []any{map[string]any{"account_id": fmt.Sprint(c.AccountID), "first_name": fmt.Sprintf("Guardian%02d", i), "last_name": "Growth", "action": "agreed", "submitted_at": s.SubmittedAt}}})
	}
	ss := []any{}
	for i := len(subs) - 1; i >= 0; i-- {
		s := subs[i]
		ss = append(ss, map[string]any{"id": fmt.Sprint(s.ID), "student_id": fmt.Sprint(s.StudentID), "student_first_name": fmt.Sprintf("Child%02d", i), "student_last_name": "Growth", "signer_name": s.SignerName, "guardian_role": s.GuardianRole, "action": s.Action, "method": s.Method, "password_confirmed": false, "version_no": n, "content_hash": s.ContentHash, "record_hash": s.RecordHash, "submitted_at": s.SubmittedAt, "integrity_ok": true, "signature": nil})
	}
	return map[string]any{"kind": "consent", "signers": "any", "revocable": true, "requires_password": false, "requires_signature": false, "deadline": nil, "current_version": vs[0], "versions": vs, "summary": map[string]int{"children_total": n, "agreed": n, "declined": 0, "revoked": 0, "partial": 0, "open": 0, "no_signer": 0, "expired": 0}, "children": children, "submissions": ss, "integrity_ok": true}
}
func communication18CSV(t *testing.T, rec *httptest.ResponseRecorder, n int, subs []*communication.PortalDeclarationSubmission) {
	t.Helper()
	require.Equal(t, "text/csv; charset=utf-8", rec.Header().Get("Content-Type"))
	require.Equal(t, `attachment; filename="nachweis-erklaerung.csv"`, rec.Header().Get("Content-Disposition"))
	require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	require.Equal(t, fmt.Sprint(rec.Body.Len()), rec.Header().Get("Content-Length"))
	require.True(t, strings.HasPrefix(rec.Body.String(), "\xef\xbb\xbf"))
	reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(rec.Body.String(), "\xef\xbb\xbf")))
	reader.Comma = ';'
	got, err := reader.ReadAll()
	require.NoError(t, err)
	want := [][]string{{"Eintrag", "Kind", "Aktion", "Zeitpunkt", "Erklärt von", "Berechtigung", "Verfahren", "Passwort bestätigt", "Fassung", "Prüfsumme Fassung", "Prüfsumme Eintrag", "Eintrag unverändert", "Unterschrift", "Unterschrift getippter Name", "Prüfsumme Unterschrift", "Unterschrift unverändert"}}
	berlin, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)
	for i := len(subs) - 1; i >= 0; i-- {
		s := subs[i]
		want = append(want, []string{fmt.Sprint(s.ID), fmt.Sprintf("Child%02d Growth", i), "Zugestimmt", s.SubmittedAt.In(berlin).Format("02.01.2006, 15:04:05") + " Uhr", s.SignerName, "Hauptberechtigt", "Einfache elektronische Erklärung im angemeldeten Eltern-Konto", "nein", fmt.Sprint(n), s.ContentHash, s.RecordHash, "ja", "nein", "", "", ""})
	}
	require.Equal(t, want, got)
}

func communication18Budget(t *testing.T, name string, n int, queries []string) {
	t.Helper()
	switch name {
	case "list":
		if n == 3 {
			querybudgets.AssertQueryBudget(t, "communication18.list.n3", queries)
		} else {
			querybudgets.AssertQueryBudget(t, "communication18.list.n8", queries)
		}
	case "get":
		if n == 3 {
			querybudgets.AssertQueryBudget(t, "communication18.get.n3", queries)
		} else {
			querybudgets.AssertQueryBudget(t, "communication18.get.n8", queries)
		}
	case "recipients":
		if n == 3 {
			querybudgets.AssertQueryBudget(t, "communication18.recipients.n3", queries)
		} else {
			querybudgets.AssertQueryBudget(t, "communication18.recipients.n8", queries)
		}
	case "poll_children":
		if n == 3 {
			querybudgets.AssertQueryBudget(t, "communication18.poll_children.n3", queries)
		} else {
			querybudgets.AssertQueryBudget(t, "communication18.poll_children.n8", queries)
		}
	case "poll_results":
		if n == 3 {
			querybudgets.AssertQueryBudget(t, "communication18.poll_results.n3", queries)
		} else {
			querybudgets.AssertQueryBudget(t, "communication18.poll_results.n8", queries)
		}
	case "letter_status":
		if n == 3 {
			querybudgets.AssertQueryBudget(t, "communication18.letter_status.n3", queries)
		} else {
			querybudgets.AssertQueryBudget(t, "communication18.letter_status.n8", queries)
		}
	case "declaration_status":
		if n == 3 {
			querybudgets.AssertQueryBudget(t, "communication18.declaration_status.n3", queries)
		} else {
			querybudgets.AssertQueryBudget(t, "communication18.declaration_status.n8", queries)
		}
	case "declaration_export":
		if n == 3 {
			querybudgets.AssertQueryBudget(t, "communication18.declaration_export.n3", queries)
		} else {
			querybudgets.AssertQueryBudget(t, "communication18.declaration_export.n8", queries)
		}
	case "school_get":
		if n == 3 {
			querybudgets.AssertQueryBudget(t, "communication18.school_get.n3", queries)
		} else {
			querybudgets.AssertQueryBudget(t, "communication18.school_get.n8", queries)
		}
	case "school_recipients":
		if n == 3 {
			querybudgets.AssertQueryBudget(t, "communication18.school_recipients.n3", queries)
		} else {
			querybudgets.AssertQueryBudget(t, "communication18.school_recipients.n8", queries)
		}
	case "school_letter_status":
		if n == 3 {
			querybudgets.AssertQueryBudget(t, "communication18.school_letter_status.n3", queries)
		} else {
			querybudgets.AssertQueryBudget(t, "communication18.school_letter_status.n8", queries)
		}
	case "school_declaration_status":
		if n == 3 {
			querybudgets.AssertQueryBudget(t, "communication18.school_declaration_status.n3", queries)
		} else {
			querybudgets.AssertQueryBudget(t, "communication18.school_declaration_status.n8", queries)
		}
	case "school_declaration_export":
		if n == 3 {
			querybudgets.AssertQueryBudget(t, "communication18.school_declaration_export.n3", queries)
		} else {
			querybudgets.AssertQueryBudget(t, "communication18.school_declaration_export.n8", queries)
		}
	}
}
