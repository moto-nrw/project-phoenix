package application

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// Fake ports of the caller context. Each fake answers from its fields and
// counts the calls the tests assert on.

type callerTestMemoKey struct{ tenantID, accountID int64 }

type callerTestMemo struct {
	mu      sync.Mutex
	entries map[callerTestMemoKey]any
	keys    []callerTestMemoKey
}

func newCallerTestMemo() *callerTestMemo {
	return &callerTestMemo{entries: make(map[callerTestMemoKey]any)}
}

func (m *callerTestMemo) Entry(tenantID, accountID int64, create func() any) any {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := callerTestMemoKey{tenantID: tenantID, accountID: accountID}
	m.keys = append(m.keys, key)
	entry, ok := m.entries[key]
	if !ok {
		entry = create()
		m.entries[key] = entry
	}
	return entry
}

func (m *callerTestMemo) Evict(tenantID, accountID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.entries, callerTestMemoKey{tenantID: tenantID, accountID: accountID})
}

type callerTestAccounts struct{}

func (callerTestAccounts) FindAccountMetadata(context.Context, int64) (domain.AccountMetadata, error) {
	return domain.AccountMetadata{}, nil
}
func (callerTestAccounts) SetAccountUsername(context.Context, int64, string) error { return nil }
func (callerTestAccounts) SetAccountAvatar(context.Context, int64, string) error   { return nil }
func (callerTestAccounts) FindAccountProfile(context.Context, int64) (domain.AccountProfile, bool, error) {
	return domain.AccountProfile{}, false, nil
}
func (callerTestAccounts) SetAccountBio(context.Context, int64, string) error { return nil }

type callerTestPeople struct {
	person domain.CallerPerson
	found  bool
	err    error
}

func (p *callerTestPeople) FindPersonByAccount(context.Context, int64) (domain.CallerPerson, bool, error) {
	return p.person, p.found, p.err
}
func (p *callerTestPeople) CreatePerson(context.Context, int64, string, string) error { return nil }
func (p *callerTestPeople) RenamePerson(context.Context, int64, *string, *string) error {
	return nil
}

type callerTestMembership struct {
	staffID      int64
	staffFound   bool
	staffErr     error
	teacherID    int64
	teacherFound bool
	teacherErr   error
}

func (m *callerTestMembership) FindStaffByPerson(context.Context, int64) (int64, bool, error) {
	return m.staffID, m.staffFound, m.staffErr
}

func (m *callerTestMembership) FindTeacherByStaff(context.Context, int64) (int64, bool, error) {
	return m.teacherID, m.teacherFound, m.teacherErr
}

type callerTestStructure struct {
	mu            sync.Mutex
	teacherGroups []int64
	teacherErr    error
	subs          map[int64]bool
	subsErr       error
	classes       []string
	teacherCalls  int
	subsCalls     int
}

func (s *callerTestStructure) TeacherGroupIDs(context.Context, int64) ([]int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.teacherCalls++
	return s.teacherGroups, s.teacherErr
}

func (s *callerTestStructure) SubstitutedGroups(context.Context, int64) (map[int64]bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subsCalls++
	return s.subs, s.subsErr
}

func (s *callerTestStructure) SchoolClasses(context.Context, int64) ([]string, error) {
	return s.classes, nil
}

// groupCalls counts the group reads the old policy stub counted as
// GetMyGroups calls.
func (s *callerTestStructure) groupCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.teacherCalls + s.subsCalls
}

type callerTestActivities struct{}

func (callerTestActivities) SupervisedActivityGroupIDs(context.Context, int64) ([]int64, error) {
	return nil, nil
}

type callerTestSessions struct{}

func (callerTestSessions) OpenSessionIDsForActivities(context.Context, []int64) ([]int64, error) {
	return nil, nil
}
func (callerTestSessions) SupervisedSessionIDs(context.Context, int64) ([]int64, error) {
	return nil, nil
}
func (callerTestSessions) SessionExists(context.Context, int64) (bool, error) { return false, nil }
func (callerTestSessions) SessionVisits(context.Context, int64) ([]domain.CallerVisit, error) {
	return nil, nil
}

// callerTestLiveTopics stands in for Student Presence's live topics: every
// open session of the school, or the staff member's own supervisions.
type callerTestLiveTopics struct {
	open       []int64
	openErr    error
	staff      []int64
	staffErr   error
	openCalls  int
	staffCalls int
}

func (l *callerTestLiveTopics) OpenSessionIDs(context.Context) ([]int64, error) {
	l.openCalls++
	return l.open, l.openErr
}

func (l *callerTestLiveTopics) StaffSessionIDs(context.Context, int64) ([]int64, error) {
	l.staffCalls++
	if l.staffErr != nil {
		return nil, l.staffErr
	}
	if l.staff == nil {
		return []int64{}, nil
	}
	return l.staff, nil
}

// callerTestOverview fakes the operational overview port. It records the
// arguments the caller context binds and asks the staff presence it was
// handed, so the tests pin the wiring; decide stands for the settings-driven
// rule, which the services test pins against the real rule.
type callerTestOverview struct {
	decide             func(hasStaff, assignmentBound, admin bool) (bool, error)
	calls              int
	gotAssignmentBound bool
	gotAdmin           bool
	gotHasStaff        bool
}

func (o *callerTestOverview) HasOperationalOverview(ctx context.Context, staff ports.StaffPresence, assignmentBound, admin bool) (bool, error) {
	o.calls++
	o.gotAssignmentBound, o.gotAdmin = assignmentBound, admin
	hasStaff, err := staff.HasCurrentStaff(ctx)
	o.gotHasStaff = err == nil && hasStaff
	return o.decide(o.gotHasStaff, assignmentBound, admin)
}

// The overview scope values of the tenant setting, as the fake decides them.
const (
	overviewOwn      = "own"
	overviewAdmins   = "admins"
	overviewAllStaff = "all_staff"
)

func overviewForScope(scope string) *callerTestOverview {
	return &callerTestOverview{decide: func(hasStaff, assignmentBound, admin bool) (bool, error) {
		if assignmentBound {
			return false, nil
		}
		if admin {
			return true, nil
		}
		return scope == overviewAllStaff && hasStaff, nil
	}}
}

func failingOverview() *callerTestOverview {
	return &callerTestOverview{decide: func(bool, bool, bool) (bool, error) {
		return false, errors.New("settings unavailable")
	}}
}

// callerHarness binds a CallerContext to fakes. A nil memo is the "no
// request memo" case; nil live topics or overview leave the port unbound.
type callerHarness struct {
	caller     domain.Caller
	memo       *callerTestMemo
	people     *callerTestPeople
	membership *callerTestMembership
	structure  *callerTestStructure
	live       *callerTestLiveTopics
	overview   *callerTestOverview
}

// Throwaway IDs of the fakes, mirroring the old service tests.
const (
	testAccountID = int64(42)
	testTenantID  = int64(10)
	testPersonID  = int64(70)
	testStaffID   = int64(42)
)

func authenticatedCaller(admin bool) domain.Caller {
	return domain.Caller{
		Authenticated: true, AccountID: testAccountID, TenantID: testTenantID,
		ClaimsTenantID: testTenantID, AdminRole: admin,
	}
}

// newCallerHarness links the caller to a person without a staff record.
func newCallerHarness(caller domain.Caller) *callerHarness {
	return &callerHarness{
		caller:     caller,
		people:     &callerTestPeople{person: domain.CallerPerson{ID: testPersonID}, found: true},
		membership: &callerTestMembership{},
		structure:  &callerTestStructure{},
	}
}

// withStaff links the caller's person to a staff member.
func (h *callerHarness) withStaff() *callerHarness {
	h.membership.staffID, h.membership.staffFound = testStaffID, true
	return h
}

func (h *callerHarness) build(t *testing.T) *CallerContext {
	t.Helper()
	deps := CallerContextDependencies{
		Caller: func(context.Context) domain.Caller { return h.caller },
		Memo: func(context.Context) ports.CallerMemo {
			if h.memo == nil {
				return nil
			}
			return h.memo
		},
		Accounts:   callerTestAccounts{},
		People:     h.people,
		Membership: h.membership,
		Structure:  h.structure,
		Activities: callerTestActivities{},
		Sessions:   callerTestSessions{},
		Write:      func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) },
		RemoveFile: func(string) error { return nil },
	}
	if h.live != nil {
		deps.LiveTopics = h.live
	}
	if h.overview != nil {
		deps.Overview = h.overview
	}
	c, err := NewCallerContext(deps)
	require.NoError(t, err)
	return c
}
