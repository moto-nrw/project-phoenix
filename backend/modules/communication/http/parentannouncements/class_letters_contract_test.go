package announcement_test

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rootAPI "github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil/routetest"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	announcement "github.com/moto-nrw/project-phoenix/modules/communication/http/parentannouncements"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// classLetterContract replaces the communication owner behind the class-letter
// routes of moto schule (#4800); every call records the author mark the
// route put on the request.
type classLetterContract struct {
	communication.ParentAnnouncementCapability
	t       *testing.T
	letters map[int64]communication.ParentAnnouncement
	calls   []string
	authors []int64
	input   communication.ParentAnnouncementInput
}

func (s *classLetterContract) record(ctx context.Context, name string) {
	s.t.Helper()
	author, ok := communication.ClassLetterAuthorFrom(ctx)
	require.True(s.t, ok, "%s runs as the Lehrkraft", name)
	s.calls = append(s.calls, name)
	s.authors = append(s.authors, author)
}

func (s *classLetterContract) ListParentAnnouncements(ctx context.Context, includeInactive bool) ([]communication.ParentAnnouncement, error) {
	require.True(s.t, includeInactive, "drafts belong to the list")
	s.record(ctx, "list")
	return []communication.ParentAnnouncement{s.letters[11], s.letters[12], s.letters[13]}, nil
}

func (s *classLetterContract) GetParentAnnouncement(ctx context.Context, id int64) (*communication.ParentAnnouncement, error) {
	s.record(ctx, "get")
	letter, ok := s.letters[id]
	if !ok {
		return nil, communication.ErrParentAnnouncementNotFound
	}
	return &letter, nil
}

func (s *classLetterContract) CreateParentAnnouncement(ctx context.Context, _ int64, in communication.ParentAnnouncementInput) (*communication.ParentAnnouncement, error) {
	s.record(ctx, "create")
	s.input = in
	return &communication.ParentAnnouncement{ID: 1, Title: in.Title, Sender: in.Sender}, nil
}

func (s *classLetterContract) PublishParentAnnouncement(ctx context.Context, id int64) (*communication.ParentAnnouncement, error) {
	s.record(ctx, "publish")
	letter := s.letters[id]
	return &letter, nil
}

func (s *classLetterContract) ParentAnnouncementLetterStatus(ctx context.Context, _ int64) (*communication.ParentAnnouncementLetterStatus, error) {
	s.record(ctx, "letter-status")
	return &communication.ParentAnnouncementLetterStatus{}, nil
}

type classReach struct{ classes []string }

func (c classReach) MySchoolClasses(context.Context) ([]string, error) { return c.classes, nil }
func (classReach) SenderName(context.Context) (string, error)          { return "Lena Lehrerin", nil }

type classLetterRoute struct {
	contract *classLetterContract
	router   chi.Router
	claims   sessiontoken.AppClaims
}

func newClassLetterRoute(t *testing.T) *classLetterRoute {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	_, account := testpkg.CreateTestStaffWithAccount(t, db, "Lena", "Lehrerin")
	own, other, ogs := int64(11), int64(12), int64(13)
	class := "3b"
	contract := &classLetterContract{t: t, letters: map[int64]communication.ParentAnnouncement{
		own:   {ID: own, Title: "Wandertag", CreatedBy: account.ID, Sender: &communication.ParentLetterSender{Name: "Lena Lehrerin", SchoolClass: "3b"}},
		other: {ID: other, Title: "Fremd", CreatedBy: account.ID + 1, Sender: &communication.ParentLetterSender{Name: "Bert", SchoolClass: "3b"}},
		ogs:   {ID: ogs, Title: "OGS", CreatedBy: account.ID, Targets: []communication.ParentAnnouncementTarget{{TargetType: "class", RefText: &class}}},
	}}
	resource := announcement.NewResource(contract, nil)
	resource.Classes = classReach{classes: []string{"3b"}}
	return &classLetterRoute{
		contract: contract,
		router:   resource.SchoolRouter(rootAPI.ProtectedSchoolRoutes),
		claims: sessiontoken.AppClaims{ID: int(account.ID), Sub: account.Email, Roles: []string{"lehrkraft"},
			TenantID: testpkg.Tenant(t), Scope: tenant.ScopeSchool},
	}
}

func (r *classLetterRoute) do(t *testing.T, method, path, body string, perms ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return routetest.ExecuteWithAuthPermissions(t, r.router, req, r.claims, perms)
}

const writePermission = permissions.ClassDayParentLetterWrite

func classLetterBody(class string) string {
	return `{"title":"Wandertag","body":"Feste Schuhe","delivery_mode":"letter","send_email":true,` +
		`"targets":[{"target_type":"class","ref_text":"` + class + `"}]}`
}

// TestClassLetterRoutesWriteOnlyToReachedClasses pins the class reach of
// #4800: the letter goes to one class the caller reaches, spelled as the
// school spells it, and names the caller as its sender.
func TestClassLetterRoutesWriteOnlyToReachedClasses(t *testing.T) {
	t.Parallel()
	route := newClassLetterRoute(t)

	out := route.do(t, "POST", "/", classLetterBody(" 3B "), writePermission)
	require.Equal(t, 201, out.Code, out.Body.String())
	require.NotNil(t, route.contract.input.Sender)
	assert.Equal(t, communication.ParentLetterSender{Name: "Lena Lehrerin", SchoolClass: "3b"}, *route.contract.input.Sender)
	assert.Contains(t, out.Body.String(), `"sender_name":"Lena Lehrerin"`, "the response names the sender")
	assert.Contains(t, out.Body.String(), `"sender_school_class":"3b"`)

	out = route.do(t, "POST", "/", classLetterBody("4a"), writePermission)
	require.Equal(t, 403, out.Code, out.Body.String())
	assert.Contains(t, out.Body.String(), "communication.class_letter_class_not_reachable")

	out = route.do(t, "POST", "/", `{"title":"A","body":"B","targets":[{"target_type":"school_all"}]}`, writePermission)
	require.Equal(t, 400, out.Code, out.Body.String())

	out = route.do(t, "POST", "/", classLetterBody("3b"))
	require.Equal(t, 403, out.Code, "the route needs class_day:parent_letter_write")
	assert.Equal(t, []string{"create"}, route.contract.calls, "refused requests never reach the owner")
}

// TestClassLetterRoutesReachOnlyTheOwnLetters pins the read side of #4800:
// the Lehrkraft sees the state of her own letters, never another one's or
// an OGS announcement, and writes again only while she reaches the class.
func TestClassLetterRoutesReachOnlyTheOwnLetters(t *testing.T) {
	t.Parallel()
	route := newClassLetterRoute(t)

	out := route.do(t, "GET", "/", "", writePermission)
	require.Equal(t, 200, out.Code, out.Body.String())
	assert.Contains(t, out.Body.String(), `"id":"11"`)
	assert.NotContains(t, out.Body.String(), `"id":"12"`, "another Lehrkraft's letter stays out")
	assert.NotContains(t, out.Body.String(), `"id":"13"`, "an OGS announcement stays out")

	out = route.do(t, "GET", "/11/letter-status", "", writePermission)
	require.Equal(t, 200, out.Code, out.Body.String())
	for _, id := range []string{"12", "13", "99"} {
		out = route.do(t, "GET", "/"+id+"/letter-status", "", writePermission)
		assert.Equal(t, 404, out.Code, "letter %s is not the caller's", id)
	}

	out = route.do(t, "POST", "/11/publish", "", writePermission)
	require.Equal(t, 200, out.Code, out.Body.String())

	route.contract.letters[11] = communication.ParentAnnouncement{ID: 11, CreatedBy: int64(route.claims.ID),
		Sender: &communication.ParentLetterSender{Name: "Lena Lehrerin", SchoolClass: "2c"}}
	out = route.do(t, "POST", "/11/publish", "", writePermission)
	require.Equal(t, 403, out.Code, "a class out of reach is not written to again")
	out = route.do(t, "GET", "/11/letter-status", "", writePermission)
	require.Equal(t, 200, out.Code, "the own letter stays readable")

	for _, author := range route.contract.authors {
		assert.Equal(t, int64(route.claims.ID), author)
	}
}

// TestOGSListShowsSentClassLettersOnly pins the OGS side of #4800: a sent
// class letter stands in the OGS list with its sender, a draft of the
// Lehrkraft stays hers until she sends it.
func TestOGSListShowsSentClassLettersOnly(t *testing.T) {
	t.Parallel()
	route := newClassLetterRoute(t)
	sent := time.Date(2026, 10, 27, 8, 0, 0, 0, time.UTC)
	letter := route.contract.letters[11]
	letter.PublishedAt = &sent
	route.contract.letters[11] = letter
	resource := announcement.NewResource(ogsListContract{route.contract}, nil)
	claims := routetest.DefaultTestClaims()
	claims.Permissions = []string{permissions.AdminWildcard}
	claims.IsAdmin = true
	req := routetest.NewAuthenticatedRequest(t, "GET", "/?include_inactive=true", nil, routetest.WithJWTBearer(routetest.MintTestJWT(t, claims)))
	out := routetest.ExecuteRequest(resource.Router(rootAPI.ProtectedTenantRoutes), req)

	require.Equal(t, 200, out.Code, out.Body.String())
	assert.Contains(t, out.Body.String(), `"id":"11"`)
	assert.Contains(t, out.Body.String(), `"sender_school_class":"3b"`)
	assert.Contains(t, out.Body.String(), `"id":"13"`, "the OGS's own announcements stay listed")
	assert.NotContains(t, out.Body.String(), `"id":"12"`, "a draft of a Lehrkraft stays out")
}

// ogsListContract serves the list without the author mark of moto schule.
type ogsListContract struct{ *classLetterContract }

func (s ogsListContract) ListParentAnnouncements(context.Context, bool) ([]communication.ParentAnnouncement, error) {
	c := s.classLetterContract
	return []communication.ParentAnnouncement{c.letters[11], c.letters[12], c.letters[13]}, nil
}

func (s ogsListContract) GetParentAnnouncement(_ context.Context, id int64) (*communication.ParentAnnouncement, error) {
	letter, ok := s.letters[id]
	if !ok {
		return nil, communication.ErrParentAnnouncementNotFound
	}
	return &letter, nil
}

func TestOGSDirectReadHidesClassLetterDrafts(t *testing.T) {
	t.Parallel()
	route := newClassLetterRoute(t)
	resource := announcement.NewResource(ogsListContract{route.contract}, nil)
	router := resource.Router(rootAPI.ProtectedTenantRoutes)
	claims := routetest.DefaultTestClaims()
	claims.ID = route.claims.ID // Even the same account needs the school portal for drafts.
	claims.Permissions = []string{permissions.AdminWildcard}
	claims.IsAdmin = true
	read := func(id string) *httptest.ResponseRecorder {
		req := routetest.NewAuthenticatedRequest(t, "GET", "/"+id, nil, routetest.WithJWTBearer(routetest.MintTestJWT(t, claims)))
		return routetest.ExecuteRequest(router, req)
	}
	out := read("11")
	require.Equal(t, 404, out.Code, out.Body.String())
	assert.NotContains(t, out.Body.String(), "Wandertag")
	out = route.do(t, "GET", "/11", "", writePermission)
	require.Equal(t, 200, out.Code, out.Body.String())
	assert.Contains(t, out.Body.String(), "Wandertag")
	require.Equal(t, 404, route.do(t, "GET", "/12", "", writePermission).Code)
	sent := time.Date(2026, 10, 27, 8, 0, 0, 0, time.UTC)
	letter := route.contract.letters[11]
	letter.PublishedAt = &sent
	route.contract.letters[11] = letter
	out = read("11")
	require.Equal(t, 200, out.Code, out.Body.String())
	assert.Contains(t, out.Body.String(), "Wandertag")
	require.Equal(t, 200, read("13").Code, "OGS drafts remain readable")
}
