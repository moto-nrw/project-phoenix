package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type externalStaffWire struct {
	ID                    string `json:"id"`
	PersonID              string `json:"person_id"`
	IsExternal            bool   `json:"is_external"`
	IsTeacher             bool   `json:"is_teacher"`
	ExternalOrganization  string `json:"external_organization"`
	ExternalAccountStatus string `json:"external_account_status"`
	InvitationToken       string `json:"invitation_token"`
	Person                *struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		AccountID *int64 `json:"account_id"`
	} `json:"person"`
}

type errorWire struct {
	Status int    `json:"status"`
	Code   string `json:"code"`
	Errors []struct {
		Field string `json:"field"`
	} `json:"errors"`
}

// externalRoute is the composed /staff router over a per-test school, with
// the Identity & Access owner the invitations go through (#4573).
type externalRoute struct {
	router   chi.Router
	db       *bun.DB
	identity identityaccess.Owner
	tenantID int64
	// actor is a Betreuer account of the school; the invitation names it as
	// its creator.
	actor int64
}

func setupExternalCaregiverRoute(t *testing.T, appEnv string) *externalRoute {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	svc, work, err := NewStaffTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	membership, err := owners.NewSchoolMembership(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	auth, err := NewAuthTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	resource := newStaffTestResource(membership, svc, work, auth.Auth, appEnv, testpkg.NewDatabase(t, db), slog.Default())

	router := chi.NewRouter()
	router.Use(testpkg.TenantRuntimeMiddleware(t, db))
	router.Mount("/staff", resource.Router())
	_, actor := testpkg.CreateTestStaffWithAccountForTenant(t, db, testpkg.Tenant(t), "Betreuer", "Konto")
	return &externalRoute{router: router, db: db, identity: auth.Auth, tenantID: testpkg.Tenant(t), actor: actor.ID}
}

// systemRoleID returns the ID of the platform system role with that name.
func systemRoleID(t *testing.T, db *bun.DB, name string) int64 {
	t.Helper()
	var roleID int64
	require.NoError(t, db.NewRaw(`SELECT id FROM auth.roles WHERE name = ? AND tenant_id IS NULL AND is_system`, name).
		Scan(context.Background(), &roleID))
	return roleID
}

// betreuer mints the token of the route's Betreuer: the unmodified user role
// holds schedules:read, the one permission of the system role ag-leitung,
// but not users:manage, which granting the role guest needs.
func (e *externalRoute) betreuer(t *testing.T, extra ...string) testutil.RequestOption {
	t.Helper()
	claims := testutil.DefaultTestClaims()
	claims.ID = int(e.actor)
	claims.Permissions = append(append([]string{}, betreuerPermissions...), append([]string{"schedules:read"}, extra...)...)
	return testutil.WithJWTBearer(testutil.MintTestJWT(t, claims))
}

func (e *externalRoute) post(t *testing.T, path string, body any, opts ...testutil.RequestOption) (int, []byte) {
	t.Helper()
	rr := testutil.ExecuteRequest(e.router, testutil.NewAuthenticatedRequest(t, http.MethodPost, path, body, opts...))
	return rr.Code, rr.Body.Bytes()
}

func (e *externalRoute) get(t *testing.T, path string, opts ...testutil.RequestOption) (int, []byte) {
	t.Helper()
	rr := testutil.ExecuteRequest(e.router, testutil.NewAuthenticatedRequest(t, http.MethodGet, path, nil, opts...))
	return rr.Code, rr.Body.Bytes()
}

func decodeExternal(t *testing.T, body []byte) externalStaffWire {
	t.Helper()
	var envelope struct {
		Data externalStaffWire `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &envelope), string(body))
	return envelope.Data
}

func decodeError(t *testing.T, body []byte) errorWire {
	t.Helper()
	var wire errorWire
	require.NoError(t, json.Unmarshal(body, &wire), string(body))
	return wire
}

func (e *externalRoute) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, e.db.NewRaw(query, args...).Scan(context.Background(), &n))
	return n
}

func (e *externalRoute) personsNamed(t *testing.T, lastName string) int {
	t.Helper()
	return e.count(t, `SELECT count(*) FROM users.persons WHERE tenant_id = ? AND last_name = ?`, e.tenantID, lastName)
}

// pendingToken reads the open invitation of a person.
func (e *externalRoute) pendingToken(t *testing.T, personID string) (token string, roleID, tenantID int64) {
	t.Helper()
	require.NoError(t, e.db.NewRaw(`SELECT token, role_id, tenant_id FROM auth.invitation_tokens
		WHERE person_id = ? AND used_at IS NULL AND expires_at > NOW()`, personID).Scan(context.Background(), &token, &roleID, &tenantID))
	return token, roleID, tenantID
}

func (e *externalRoute) accept(t *testing.T, token string) error {
	t.Helper()
	password := testpkg.NewTestPassword()
	_, err := e.identity.AcceptSchoolInvitation(context.Background(), token, identityaccess.InvitationRegistration{
		FirstName: "Jonas", LastName: "Becker", Password: password, ConfirmPassword: password,
	})
	return err
}

func uniqueAddress(t *testing.T, local string) string {
	t.Helper()
	return fmt.Sprintf("%s-%d@example.test", local, testpkg.Tenant(t))
}

// TestBetreuerRecordsAnExternalCaregiver pins #3823 and #4573: a Betreuer
// records an external caregiver with the access supervise; person, staff
// member, caregiver and external caregiver record exist, an invitation in
// the system role ag-leitung names the person, its mail sits in the outbox,
// and the directory marks the entry as external with an open invitation.
func TestBetreuerRecordsAnExternalCaregiver(t *testing.T) {
	t.Parallel()

	e := setupExternalCaregiverRoute(t, "")
	roleID := systemRoleID(t, e.db, identityaccess.AGLeitungRoleName)
	address := uniqueAddress(t, "lea")

	status, body := e.post(t, "/staff/externals",
		map[string]any{"first_name": "  Lea ", "last_name": "Gastmann", "email": " " + strings.ToUpper(address) + " ", "organization": " Musikschule ", "access": "supervise"}, e.betreuer(t))
	require.Equal(t, http.StatusCreated, status, string(body))
	created := decodeExternal(t, body)
	require.NotEmpty(t, created.ID)
	assert.True(t, created.IsExternal)
	assert.True(t, created.IsTeacher, "every external has a caregiver profile and stays plannable")
	assert.Equal(t, "Musikschule", created.ExternalOrganization)
	assert.Equal(t, "invitation_pending", created.ExternalAccountStatus)
	assert.Empty(t, created.InvitationToken, "the token never leaves the server outside the local seeder")
	require.NotNil(t, created.Person)
	assert.Equal(t, "Lea", created.Person.FirstName)
	assert.Nil(t, created.Person.AccountID, "the account exists once the invitation is accepted")

	assert.Equal(t, 1, e.count(t, `SELECT count(*) FROM users.teachers WHERE staff_id = ?`, created.ID))
	assert.Equal(t, 1, e.count(t, `SELECT count(*) FROM users.external_caregivers WHERE staff_id = ? AND organization = 'Musikschule'`, created.ID))
	_, invitedRole, invitedTenant := e.pendingToken(t, created.PersonID)
	assert.Equal(t, roleID, invitedRole, "supervise invites into the system role ag-leitung")
	assert.Equal(t, e.tenantID, invitedTenant, "the invitation belongs to the caller's school")
	assert.Equal(t, 1, e.count(t, `SELECT count(*) FROM auth.invitation_tokens WHERE person_id = ? AND email = ?`, created.PersonID, address),
		"the address is stored trimmed and in lower case")
	assert.Positive(t, e.count(t, `SELECT count(*) FROM platform.email_outbox WHERE tenant_id = ? AND payload::text LIKE ?`, e.tenantID, "%"+address+"%"),
		"the invitation mail is queued with the write")

	status, body = e.get(t, "/staff", e.betreuer(t))
	require.Equal(t, http.StatusOK, status, string(body))
	var listed struct {
		Data []externalStaffWire `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &listed))
	found := false
	for _, entry := range listed.Data {
		if entry.ID == created.ID {
			found = true
			assert.True(t, entry.IsExternal)
			assert.Equal(t, "invitation_pending", entry.ExternalAccountStatus)
		} else {
			assert.False(t, entry.IsExternal, "regular staff stay unmarked")
			assert.Empty(t, entry.ExternalAccountStatus, "only externals carry an account status")
		}
	}
	assert.True(t, found, "the directory lists the new entry")
}

func TestExternalCaregiverRequiresBothNamesAndCreatePermission(t *testing.T) {
	t.Parallel()

	ctx := setupStaffCompositionRoute(t)
	betreuer := testutil.WithJWTBearer(staffCompositionToken(t, betreuerPermissions...))
	for _, body := range []map[string]any{
		{"first_name": "Lea", "last_name": "  ", "email": "lea@example.test"},
		{"first_name": "", "last_name": "Gastmann", "email": "lea@example.test"},
	} {
		rr := testutil.ExecuteRequest(ctx.router, testutil.NewAuthenticatedRequest(t, http.MethodPost, "/staff/externals", body, betreuer))
		assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
	}

	readOnly := testutil.WithJWTBearer(staffCompositionToken(t, "users:read"))
	rr := testutil.ExecuteRequest(ctx.router, testutil.NewAuthenticatedRequest(t, http.MethodPost, "/staff/externals",
		map[string]any{"first_name": "Lea", "last_name": "Gastmann", "email": "lea@example.test", "access": "supervise"}, readOnly))
	assert.Equal(t, http.StatusForbidden, rr.Code, rr.Body.String())
	rr = testutil.ExecuteRequest(ctx.router, testutil.NewAuthenticatedRequest(t, http.MethodPost, "/staff/externals/1/invitation",
		map[string]any{"email": "lea@example.test", "access": "supervise"}, readOnly))
	assert.Equal(t, http.StatusForbidden, rr.Code, rr.Body.String())

	rr = testutil.ExecuteRequest(ctx.router, testutil.NewAuthenticatedRequest(t, http.MethodGet, "/staff", nil, betreuer))
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.NotContains(t, rr.Body.String(), "Gastmann", "refused requests leave no staff entry behind")
}

// TestExternalCaregiverAddressErrorsNameTheField: the form marks the address
// field, and nothing is written.
func TestExternalCaregiverAddressErrorsNameTheField(t *testing.T) {
	t.Parallel()

	e := setupExternalCaregiverRoute(t, "")
	for email, code := range map[string]string{
		"   ":                  "staff.external_email_required",
		"keine-adresse":        "staff.external_email_invalid",
		"Lea <lea@example.de>": "staff.external_email_invalid",
	} {
		status, body := e.post(t, "/staff/externals",
			map[string]any{"first_name": "Lea", "last_name": "Ohneadresse", "email": email, "access": "supervise"}, e.betreuer(t))
		require.Equal(t, http.StatusBadRequest, status, string(body))
		wire := decodeError(t, body)
		assert.Equal(t, code, wire.Code, email)
		require.Len(t, wire.Errors, 1)
		assert.Equal(t, "email", wire.Errors[0].Field)
	}
	assert.Zero(t, e.personsNamed(t, "Ohneadresse"))
}

// TestExternalCaregiverRefusesAnAddressAlreadyAtTheSchool: the request
// transaction commits 4xx answers, so the workflow's savepoint must undo the
// person, staff member and profiles written before the invitation refused.
func TestExternalCaregiverRefusesAnAddressAlreadyAtTheSchool(t *testing.T) {
	t.Parallel()

	e := setupExternalCaregiverRoute(t, "")
	_, colleague := testpkg.CreateTestStaffWithAccountForTenant(t, e.db, e.tenantID, "Schon", "Da")

	status, body := e.post(t, "/staff/externals",
		map[string]any{"first_name": "Doppelt", "last_name": "Angelegt", "email": colleague.Email, "access": "supervise"}, e.betreuer(t))
	require.Equal(t, http.StatusConflict, status, string(body))
	wire := decodeError(t, body)
	assert.Equal(t, "identity.account_already_has_tenant_access", wire.Code)
	require.Len(t, wire.Errors, 1)
	assert.Equal(t, "email", wire.Errors[0].Field)
	assert.Zero(t, e.personsNamed(t, "Angelegt"))
	assert.Zero(t, e.count(t, `SELECT count(*) FROM users.external_caregivers g JOIN users.staff_school_memberships s ON s.id = g.staff_id
		JOIN users.persons p ON p.id = s.person_id WHERE p.last_name = 'Angelegt'`))
}

// TestExternalCaregiverViewNeedsUsersManage: CanGrantRole applies
// unchanged. The system role guest has no base role, so only users:manage
// grants it; a Betreuer holding users:create is refused and nothing is
// written. The access view invites into guest.
func TestExternalCaregiverViewNeedsUsersManage(t *testing.T) {
	t.Parallel()

	e := setupExternalCaregiverRoute(t, "")
	status, body := e.post(t, "/staff/externals",
		map[string]any{"first_name": "Nicht", "last_name": "Erlaubt", "email": uniqueAddress(t, "grant"), "access": "view"}, e.betreuer(t))
	require.Equal(t, http.StatusForbidden, status, string(body))
	assert.Equal(t, "general.permission", decodeError(t, body).Code)
	assert.Zero(t, e.personsNamed(t, "Erlaubt"))

	legacy := testpkg.CreateTestExternalCaregiverRecord(t, e.db, "Musik")
	status, body = e.post(t, fmt.Sprintf("/staff/externals/%d/invitation", legacy.StaffID),
		map[string]any{"email": uniqueAddress(t, "grant-invite"), "access": "view"}, e.betreuer(t))
	require.Equal(t, http.StatusForbidden, status, string(body))
	assert.Equal(t, "general.permission", decodeError(t, body).Code)
	assert.Zero(t, e.count(t, `SELECT count(*) FROM auth.invitation_tokens WHERE person_id = ?`, legacy.Staff.PersonID))

	status, body = e.post(t, "/staff/externals",
		map[string]any{"first_name": "Doch", "last_name": "Erlaubt", "email": uniqueAddress(t, "grant"), "access": "view"}, e.betreuer(t, "users:manage"))
	require.Equal(t, http.StatusCreated, status, string(body), "users:manage may grant every role")
	created := decodeExternal(t, body)
	assert.Equal(t, "invitation_pending", created.ExternalAccountStatus)
	_, invitedRole, _ := e.pendingToken(t, created.PersonID)
	assert.Equal(t, systemRoleID(t, e.db, identityaccess.GuestRoleName), invitedRole, "view invites into the system role guest")
}

// TestExternalCaregiverWithoutAccess: access none records a placeholder
// with caregiver profile and external caregiver record, needs no address,
// ignores one that is sent and invites nobody.
func TestExternalCaregiverWithoutAccess(t *testing.T) {
	t.Parallel()

	e := setupExternalCaregiverRoute(t, "")
	for _, body := range []map[string]any{
		{"first_name": "Platz", "last_name": "Halter", "organization": "Förderverein", "access": "none"},
		{"first_name": "Platz", "last_name": "Halterin", "email": "keine-adresse", "access": "none"},
	} {
		status, raw := e.post(t, "/staff/externals", body, e.betreuer(t))
		require.Equal(t, http.StatusCreated, status, string(raw))
		created := decodeExternal(t, raw)
		assert.True(t, created.IsExternal)
		assert.True(t, created.IsTeacher, "the placeholder stays plannable")
		assert.Equal(t, "none", created.ExternalAccountStatus)
		assert.Empty(t, created.InvitationToken)
		assert.Equal(t, 1, e.count(t, `SELECT count(*) FROM users.external_caregivers WHERE staff_id = ?`, created.ID))
		assert.Zero(t, e.count(t, `SELECT count(*) FROM auth.invitation_tokens WHERE person_id = ?`, created.PersonID), "no invitation")
	}
}

// TestExternalCaregiverAccessErrorsNameTheField: a missing or unknown
// access, and none on „Konto einladen“, refuse on the field access before
// any write.
func TestExternalCaregiverAccessErrorsNameTheField(t *testing.T) {
	t.Parallel()

	e := setupExternalCaregiverRoute(t, "")
	legacy := testpkg.CreateTestExternalCaregiverRecord(t, e.db, "Musik")
	for path, bodies := range map[string][]map[string]any{
		"/staff/externals": {
			{"first_name": "Ohne", "last_name": "Zugang", "email": uniqueAddress(t, "zugang")},
			{"first_name": "Ohne", "last_name": "Zugang", "email": uniqueAddress(t, "zugang"), "access": "admin"},
		},
		fmt.Sprintf("/staff/externals/%d/invitation", legacy.StaffID): {
			{"email": uniqueAddress(t, "zugang-invite")},
			{"email": uniqueAddress(t, "zugang-invite"), "access": "none"},
		},
	} {
		for _, body := range bodies {
			status, raw := e.post(t, path, body, e.betreuer(t))
			require.Equal(t, http.StatusBadRequest, status, string(raw))
			wire := decodeError(t, raw)
			assert.Equal(t, "staff.external_access_invalid", wire.Code)
			require.Len(t, wire.Errors, 1)
			assert.Equal(t, "access", wire.Errors[0].Field)
		}
	}
	assert.Zero(t, e.personsNamed(t, "Zugang"))
	assert.Zero(t, e.count(t, `SELECT count(*) FROM auth.invitation_tokens WHERE person_id = ?`, legacy.Staff.PersonID))
}

// TestExternalCaregiverAcceptsOnTheSamePerson: accepting the invitation
// attaches the account to the recorded person; no second staff member
// appears and the planned blocks stay assigned.
func TestExternalCaregiverAcceptsOnTheSamePerson(t *testing.T) {
	t.Parallel()

	e := setupExternalCaregiverRoute(t, "")
	status, body := e.post(t, "/staff/externals",
		map[string]any{"first_name": "Jonas", "last_name": "Becker", "email": uniqueAddress(t, "jonas"), "access": "supervise"}, e.betreuer(t))
	require.Equal(t, http.StatusCreated, status, string(body))
	created := decodeExternal(t, body)
	staffID, err := strconv.ParseInt(created.ID, 10, 64)
	require.NoError(t, err)

	room := testpkg.CreateTestRoom(t, e.db, "AG-Raum")
	instance := testpkg.CreateTestActivityInstance(t, e.db, testpkg.TodayDate(), room.ID, testpkg.ActivityInstanceOpts{Title: "AG Musik"})
	testpkg.CreateTestInstanceStaff(t, e.db, instance.ID, staffID, testpkg.InstanceStaffOpts{IsPrimary: true})

	token, _, _ := e.pendingToken(t, created.PersonID)
	require.NoError(t, e.accept(t, token))

	var accountID *int64
	require.NoError(t, e.db.NewRaw(`SELECT account_id FROM users.persons WHERE id = ?`, created.PersonID).Scan(context.Background(), &accountID))
	require.NotNil(t, accountID, "the account belongs to the recorded person")
	assert.Equal(t, 1, e.count(t, `SELECT count(*) FROM users.staff_school_memberships WHERE person_id = ? AND deleted_at IS NULL`, created.PersonID),
		"no second staff member")
	assert.Equal(t, 1, e.count(t, `SELECT count(*) FROM users.persons WHERE tenant_id = ? AND last_name = 'Becker'`, e.tenantID),
		"no second person")
	assert.Equal(t, 1, e.count(t, `SELECT count(*) FROM schedule.instance_staff WHERE instance_id = ? AND staff_id = ?`, instance.ID, staffID),
		"the planned block stays assigned")

	status, body = e.get(t, "/staff/"+created.ID, e.betreuer(t))
	require.Equal(t, http.StatusOK, status, string(body))
	detail := decodeExternal(t, body)
	assert.True(t, detail.IsExternal, "an account does not end the external status")
	assert.Equal(t, "account", detail.ExternalAccountStatus)
}

// TestInviteExistingExternalCaregiver drives „Konto einladen“: a legacy
// external without caregiver profile is invited, a second invitation to a
// corrected address spends the first, and an account ends the action.
func TestInviteExistingExternalCaregiver(t *testing.T) {
	t.Parallel()

	e := setupExternalCaregiverRoute(t, "")
	legacy := testpkg.CreateTestExternalCaregiverRecord(t, e.db, "Musik")
	path := fmt.Sprintf("/staff/externals/%d/invitation", legacy.StaffID)
	personID := strconv.FormatInt(legacy.Staff.PersonID, 10)

	status, body := e.post(t, path, map[string]any{"email": uniqueAddress(t, "tippfehler"), "access": "supervise"}, e.betreuer(t))
	require.Equal(t, http.StatusOK, status, string(body))
	invited := decodeExternal(t, body)
	assert.Equal(t, strconv.FormatInt(legacy.StaffID, 10), invited.ID)
	assert.Equal(t, "invitation_pending", invited.ExternalAccountStatus)
	assert.Equal(t, 1, e.count(t, `SELECT count(*) FROM users.teachers WHERE staff_id = ?`, legacy.StaffID), "the caregiver profile is added")
	firstToken, _, _ := e.pendingToken(t, personID)

	status, body = e.post(t, path, map[string]any{"email": uniqueAddress(t, "richtig"), "access": "supervise"}, e.betreuer(t))
	require.Equal(t, http.StatusOK, status, string(body))
	assert.Equal(t, 1, e.count(t, `SELECT count(*) FROM users.teachers WHERE staff_id = ?`, legacy.StaffID), "the profile is not duplicated")
	secondToken, _, _ := e.pendingToken(t, personID)
	require.NotEqual(t, firstToken, secondToken)

	require.Error(t, e.accept(t, firstToken), "the mistyped address can no longer claim the person")
	require.NoError(t, e.accept(t, secondToken))

	status, body = e.get(t, fmt.Sprintf("/staff/%d", legacy.StaffID), e.betreuer(t))
	require.Equal(t, http.StatusOK, status, string(body))
	assert.Equal(t, "account", decodeExternal(t, body).ExternalAccountStatus)

	status, body = e.post(t, path, map[string]any{"email": uniqueAddress(t, "nochmal"), "access": "supervise"}, e.betreuer(t))
	require.Equal(t, http.StatusConflict, status, string(body))
	assert.Equal(t, "staff.external_has_account", decodeError(t, body).Code)
}

func TestInviteRefusesStaffThatIsNoExternalCaregiver(t *testing.T) {
	t.Parallel()

	e := setupExternalCaregiverRoute(t, "")
	internal := testpkg.CreateTestStaff(t, e.db, "Interne", "Kollegin")
	foreignTenantID, _ := testpkg.CreateTestTenant(t, e.db)
	foreign, _ := testpkg.CreateTestStaffWithAccountForTenant(t, e.db, foreignTenantID, "Fremde", "Schule")

	// A real external of another school: staff member with an external caregiver record
	// and without an account, which its own school could invite.
	foreignExternal := recordForeignExternal(t, e.db, foreignTenantID)

	status, body := e.post(t, fmt.Sprintf("/staff/externals/%d/invitation", internal.ID), map[string]any{"email": uniqueAddress(t, "intern"), "access": "supervise"}, e.betreuer(t))
	require.Equal(t, http.StatusConflict, status, string(body))
	assert.Equal(t, "staff.not_external", decodeError(t, body).Code)

	status, body = e.post(t, fmt.Sprintf("/staff/externals/%d/invitation", foreign.ID), map[string]any{"email": uniqueAddress(t, "fremd"), "access": "supervise"}, e.betreuer(t))
	require.Equal(t, http.StatusNotFound, status, string(body))
	status, body = e.post(t, fmt.Sprintf("/staff/externals/%d/invitation", foreignExternal.ID), map[string]any{"email": uniqueAddress(t, "fremdextern"), "access": "supervise"}, e.betreuer(t))
	require.Equal(t, http.StatusNotFound, status, string(body), "another school's external is unknown here")
	assert.Zero(t, e.count(t, `SELECT count(*) FROM auth.invitation_tokens WHERE person_id IN (?, ?, ?)`, internal.PersonID, foreign.PersonID, foreignExternal.PersonID),
		"no invitation names any of the persons")
	assert.Zero(t, e.count(t, `SELECT count(*) FROM users.teachers WHERE staff_id = ?`, foreignExternal.ID),
		"the other school's external gets no caregiver profile")

	status, body = e.post(t, "/staff/externals/abc/invitation", map[string]any{"email": uniqueAddress(t, "abc"), "access": "supervise"}, e.betreuer(t))
	require.Equal(t, http.StatusBadRequest, status, string(body))
}

// TestExternalCaregiverSeedTokenOnlyForTheLocalSeeder: the raw token lets
// whoever holds it accept for the address, so only the local seeder sees it.
func TestExternalCaregiverSeedTokenOnlyForTheLocalSeeder(t *testing.T) {
	t.Parallel()

	e := setupExternalCaregiverRoute(t, "development")
	seeder := func(req *http.Request) {
		req.Host = "localhost:8080"
		req.Header.Set("X-Phoenix-Seed-Token", "true")
	}

	status, body := e.post(t, "/staff/externals", map[string]any{"first_name": "Seed", "last_name": "Person", "email": uniqueAddress(t, "seed"), "access": "supervise"}, e.betreuer(t), seeder)
	require.Equal(t, http.StatusCreated, status, string(body))
	created := decodeExternal(t, body)
	token, _, _ := e.pendingToken(t, created.PersonID)
	assert.Equal(t, token, created.InvitationToken)

	status, body = e.post(t, "/staff/externals", map[string]any{"first_name": "Fremd", "last_name": "Host", "email": uniqueAddress(t, "host"), "access": "supervise"}, e.betreuer(t),
		func(req *http.Request) { req.Header.Set("X-Phoenix-Seed-Token", "true") })
	require.Equal(t, http.StatusCreated, status, string(body))
	assert.Empty(t, decodeExternal(t, body).InvitationToken)
}

// TestAccountBackedExternalCaregiverRecordIsAnExternalCaregiver pins #4573 (it inverts the
// #3823 rule): an external caregiver record makes a staff member external, with or without
// an account, and the directory reports the account status.
func TestAccountBackedExternalCaregiverRecordIsAnExternalCaregiver(t *testing.T) {
	t.Parallel()

	ctx := setupStaffCompositionRoute(t)
	accountExternalID := ctx.createAccountExternal("Konto", "Extern")
	betreuer := testutil.WithJWTBearer(staffCompositionToken(t, betreuerPermissions...))

	rr := testutil.ExecuteRequest(ctx.router, testutil.NewAuthenticatedRequest(t, http.MethodGet, "/staff", nil, betreuer))
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var listed struct {
		Data []externalStaffWire `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &listed))
	for _, entry := range listed.Data {
		if entry.ID == strconv.FormatInt(accountExternalID, 10) {
			assert.True(t, entry.IsExternal)
			assert.Equal(t, "account", entry.ExternalAccountStatus)
			return
		}
	}
	t.Fatalf("account-backed external caregiver %d missing from staff directory", accountExternalID)
}

// recordForeignExternal records an external caregiver without an account at another
// school.
func recordForeignExternal(t *testing.T, db *bun.DB, tenantID int64) *testpkg.Staff {
	t.Helper()
	staff := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Fremde", "Externe")
	_, err := db.NewRaw(`INSERT INTO users.external_caregivers (tenant_id, staff_id, activity_expertise) VALUES (?, ?, 'AG')`, tenantID, staff.ID).
		Exec(context.Background())
	require.NoError(t, err)
	return staff
}

// openInvitation issues a school invitation for the address the way the
// invitation form does, in the role, and returns its ID.
func (e *externalRoute) openInvitation(t *testing.T, tenantID, roleID int64, address string) int64 {
	t.Helper()
	invitation, err := e.identity.CreateSchoolInvitation(context.Background(), identityaccess.SchoolInvitationRequest{
		Email: address, RoleID: roleID, TenantID: tenantID, OperatorGrant: true,
	})
	require.NoError(t, err)
	return invitation.ID
}

func (e *externalRoute) stillOpen(t *testing.T, invitationID int64) bool {
	t.Helper()
	return e.count(t, `SELECT count(*) FROM auth.invitation_tokens WHERE id = ? AND used_at IS NULL`, invitationID) == 1
}

// TestExternalCaregiverKeepsAnotherPersonsOpenInvitation: an invitation for
// the address spends every open invitation for it, so an open invitation of
// someone else (here a staff invitation the Leitung sent) refuses both the
// quick create and „Konto einladen“ on the address field, and nothing is
// written. The external's own open invitation is sent again.
func TestExternalCaregiverKeepsAnotherPersonsOpenInvitation(t *testing.T) {
	t.Parallel()

	e := setupExternalCaregiverRoute(t, "")
	roleID := systemRoleID(t, e.db, identityaccess.AGLeitungRoleName)
	address := uniqueAddress(t, "vergeben")
	staffInvitation := e.openInvitation(t, e.tenantID, roleID, address)

	status, body := e.post(t, "/staff/externals",
		map[string]any{"first_name": "Doppelt", "last_name": "Eingeladen", "email": strings.ToUpper(address), "access": "supervise"}, e.betreuer(t))
	require.Equal(t, http.StatusConflict, status, string(body))
	wire := decodeError(t, body)
	assert.Equal(t, "staff.external_email_invited", wire.Code)
	require.Len(t, wire.Errors, 1)
	assert.Equal(t, "email", wire.Errors[0].Field)
	assert.Zero(t, e.personsNamed(t, "Eingeladen"), "nothing is written")
	assert.True(t, e.stillOpen(t, staffInvitation), "the staff invitation stays redeemable")

	legacy := testpkg.CreateTestExternalCaregiverRecord(t, e.db, "Musik")
	path := fmt.Sprintf("/staff/externals/%d/invitation", legacy.StaffID)
	status, body = e.post(t, path, map[string]any{"email": address, "access": "supervise"}, e.betreuer(t))
	require.Equal(t, http.StatusConflict, status, string(body))
	assert.Equal(t, "staff.external_email_invited", decodeError(t, body).Code)
	assert.True(t, e.stillOpen(t, staffInvitation))
	assert.Zero(t, e.count(t, `SELECT count(*) FROM auth.invitation_tokens WHERE person_id = ?`, legacy.Staff.PersonID))
	assert.Zero(t, e.count(t, `SELECT count(*) FROM users.teachers WHERE staff_id = ?`, legacy.StaffID), "nothing is written")

	own := uniqueAddress(t, "eigene")
	status, body = e.post(t, path, map[string]any{"email": own, "access": "supervise"}, e.betreuer(t))
	require.Equal(t, http.StatusOK, status, string(body))
	firstToken, _, _ := e.pendingToken(t, strconv.FormatInt(legacy.Staff.PersonID, 10))
	status, body = e.post(t, path, map[string]any{"email": own, "access": "supervise"}, e.betreuer(t))
	require.Equal(t, http.StatusOK, status, string(body), "the external's own invitation is sent again")
	secondToken, _, _ := e.pendingToken(t, strconv.FormatInt(legacy.Staff.PersonID, 10))
	assert.NotEqual(t, firstToken, secondToken)
}

// TestExternalCaregiverCannotReachAnotherSchoolsInvitation: another school's
// open invitation for the same address is invisible here. It neither
// refuses this school's create or invite, nor is it spent by them, nor does
// this school's pending-invitation list show it.
func TestExternalCaregiverCannotReachAnotherSchoolsInvitation(t *testing.T) {
	t.Parallel()

	e := setupExternalCaregiverRoute(t, "")
	foreignTenantID, _ := testpkg.CreateTestTenant(t, e.db)
	foreignRole := systemRoleID(t, e.db, identityaccess.AGLeitungRoleName)
	created, invited := uniqueAddress(t, "beide-schulen"), uniqueAddress(t, "auch-dort")
	foreignCreated := e.openInvitation(t, foreignTenantID, foreignRole, created)
	foreignInvited := e.openInvitation(t, foreignTenantID, foreignRole, invited)

	status, body := e.post(t, "/staff/externals",
		map[string]any{"first_name": "Zwei", "last_name": "Schulen", "email": created, "access": "supervise"}, e.betreuer(t))
	require.Equal(t, http.StatusCreated, status, string(body))
	legacy := testpkg.CreateTestExternalCaregiverRecord(t, e.db, "Musik")
	status, body = e.post(t, fmt.Sprintf("/staff/externals/%d/invitation", legacy.StaffID), map[string]any{"email": invited, "access": "supervise"}, e.betreuer(t))
	require.Equal(t, http.StatusOK, status, string(body))

	assert.True(t, e.stillOpen(t, foreignCreated), "the other school's invitation is not spent by the create")
	assert.True(t, e.stillOpen(t, foreignInvited), "the other school's invitation is not spent by the invite")
	pending, err := e.identity.ListPendingSchoolInvitations(testpkg.Ctx(t))
	require.NoError(t, err)
	require.NotEmpty(t, pending)
	for _, invitation := range pending {
		assert.Equal(t, e.tenantID, invitation.TenantID)
		assert.NotContains(t, []int64{foreignCreated, foreignInvited}, invitation.ID)
	}
}
