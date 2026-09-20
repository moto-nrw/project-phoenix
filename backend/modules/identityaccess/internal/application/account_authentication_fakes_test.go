package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
	"github.com/stretchr/testify/require"
)

// In-memory doubles of every port the account-authentication flows consume.
// They model the facts the flows decide on (accounts, mappings, roles,
// permissions, schools, persons, sessions) and let a test inject failures at
// one seam to prove the flow's error classification.

// --- context markers of the fake runtime ---------------------------------

type fakeTxKey struct{}
type fakeAdminKey struct{}
type fakeTenantKey struct{}
type fakeScopeKey struct{}
type fakeOrgKey struct{}
type fakeHooksKey struct{}
type fakeProofKey struct{}

type fakeHooks struct{ fns []func() }

type fakeRuntime struct {
	// adminTxCount counts the administrative transactions opened.
	adminTxCount int
	locks        []string
	lockHook     func(string)
}

func (r *fakeRuntime) WithAdminTx(ctx context.Context, fn func(context.Context) error) error {
	r.adminTxCount++
	hooks := &fakeHooks{}
	txCtx := context.WithValue(context.WithValue(ctx, fakeTxKey{}, true), fakeAdminKey{}, true)
	txCtx = context.WithValue(txCtx, fakeHooksKey{}, hooks)
	if err := fn(txCtx); err != nil {
		return err
	}
	for _, hook := range hooks.fns {
		hook()
	}
	return nil
}

func (r *fakeRuntime) WithTenantTx(ctx context.Context, tenantID int64, fn func(context.Context) error) error {
	if tenantID <= 0 {
		return errors.New("tenant: tenant ID is required")
	}
	hooks := &fakeHooks{}
	txCtx := context.WithValue(context.WithValue(ctx, fakeTxKey{}, true), fakeTenantKey{}, tenantID)
	txCtx = context.WithValue(txCtx, fakeHooksKey{}, hooks)
	if err := fn(txCtx); err != nil {
		return err
	}
	for _, hook := range hooks.fns {
		hook()
	}
	return nil
}

func (r *fakeRuntime) RunInTx(ctx context.Context, fn func(context.Context) error) error {
	if r.HasTransaction(ctx) {
		return fn(ctx)
	}
	if r.TenantID(ctx) == 0 {
		return r.WithAdminTx(ctx, fn)
	}
	return r.WithTenantTx(ctx, r.TenantID(ctx), fn)
}

func (r *fakeRuntime) IsAdminTx(ctx context.Context) bool {
	admin, _ := ctx.Value(fakeAdminKey{}).(bool)
	return admin
}

func (r *fakeRuntime) HasTransaction(ctx context.Context) bool {
	tx, _ := ctx.Value(fakeTxKey{}).(bool)
	return tx
}

func (r *fakeRuntime) HasAfterCommitHooks(ctx context.Context) bool {
	_, ok := ctx.Value(fakeHooksKey{}).(*fakeHooks)
	return ok
}

func (r *fakeRuntime) RegisterAfterCommit(ctx context.Context, fn func()) {
	hooks, ok := ctx.Value(fakeHooksKey{}).(*fakeHooks)
	if !ok {
		fn()
		return
	}
	hooks.fns = append(hooks.fns, fn)
}

func (r *fakeRuntime) TenantID(ctx context.Context) int64 {
	id, _ := ctx.Value(fakeTenantKey{}).(int64)
	return id
}

func (r *fakeRuntime) Scope(ctx context.Context) string {
	scope, _ := ctx.Value(fakeScopeKey{}).(string)
	return scope
}

func (r *fakeRuntime) OrgID(ctx context.Context) int64 {
	id, _ := ctx.Value(fakeOrgKey{}).(int64)
	return id
}

func (r *fakeRuntime) WithTenantID(ctx context.Context, tenantID int64) context.Context {
	return context.WithValue(ctx, fakeTenantKey{}, tenantID)
}

func (r *fakeRuntime) Detach(ctx context.Context) context.Context {
	ctx = context.WithValue(ctx, fakeTxKey{}, false)
	ctx = context.WithValue(ctx, fakeAdminKey{}, false)
	ctx = context.WithValue(ctx, fakeTenantKey{}, int64(0))
	return context.WithValue(ctx, fakeHooksKey{}, nil)
}

func (r *fakeRuntime) WithoutTransaction(ctx context.Context) context.Context {
	ctx = context.WithValue(ctx, fakeTxKey{}, false)
	return context.WithValue(ctx, fakeAdminKey{}, false)
}

func (r *fakeRuntime) AcquireLock(_ context.Context, key string) error {
	r.locks = append(r.locks, key)
	if r.lockHook != nil {
		r.lockHook(key)
	}
	return nil
}

func withScope(ctx context.Context, scope string, orgID int64) context.Context {
	return context.WithValue(context.WithValue(ctx, fakeScopeKey{}, scope), fakeOrgKey{}, orgID)
}

type fakeRotation struct{}

func (fakeRotation) RecoveryGrace() time.Duration { return 5 * time.Minute }
func (fakeRotation) MaxRecoveryHops() int         { return 8 }
func (fakeRotation) RecoveryProofHash(ctx context.Context) []byte {
	proof, _ := ctx.Value(fakeProofKey{}).([]byte)
	return proof
}
func (fakeRotation) MatchesRecoveryProof(ctx context.Context, expected []byte) bool {
	actual, _ := ctx.Value(fakeProofKey{}).([]byte)
	return len(expected) == 32 && len(actual) == 32 && string(expected) == string(actual)
}
func (fakeRotation) FamilyFingerprint(familyID string) string { return "fp:" + familyID }

type mappingKey struct{ accountID, tenantID int64 }

// fakeStore implements ports.Store (the guardian and school-role facts the
// session service is constructed with), ports.OperatorStore, the session
// store and ports.AccountLoginStore over maps.
type fakeStore struct {
	mu sync.Mutex

	accounts    map[int64]domain.LoginAccount
	mappings    []mappingKey // insertion order
	inactive    map[mappingKey]bool
	roles       map[mappingKey][]domain.RoleAssignment
	permissions map[mappingKey][]string
	operators   map[int64]domain.Operator

	sessions       map[int64]domain.AccountSession
	nextSessionID  int64
	operatorTokens map[int64]domain.OperatorSession

	// error injection
	findAccountErr      error
	findAccountByEmail  error
	listTenantsErr      error
	hasMappingErr       error
	lockMappingErr      error
	listRolesErr        error
	listPermissionsErr  error
	lockPermissionsErr  error
	recordLoginErr      error
	insertSessionErr    error
	insertSessionErrors []error
	findOperatorErr     error
	updateOperatorErr   error
	// findSessionErrs injects a lookup failure for one operator session
	// token, so a transient error mid-recovery can be told from a replay.
	findSessionErrs map[string]error

	// observations
	calls []string

	insertOperatorErr error
	// nextOperatorID hands out the identities an insert assigns; seeded
	// operators use their own ids, so it starts above the usual ones.
	nextOperatorID int64
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		accounts: map[int64]domain.LoginAccount{}, inactive: map[mappingKey]bool{},
		roles: map[mappingKey][]domain.RoleAssignment{}, permissions: map[mappingKey][]string{},
		operators: map[int64]domain.Operator{}, sessions: map[int64]domain.AccountSession{}, operatorTokens: map[int64]domain.OperatorSession{},
		findSessionErrs: map[string]error{}, nextOperatorID: 1000,
	}
}

func (s *fakeStore) record(call string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, call)
}

// recorded returns the statements in the order they ran.
func (s *fakeStore) recorded() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.calls...)
}

func (s *fakeStore) addAccount(id int64, email, passwordHash string, active bool) {
	s.accounts[id] = domain.LoginAccount{ID: id, Email: email, PasswordHash: passwordHash, Active: active, Username: "user" + fmt.Sprint(id)}
}

func (s *fakeStore) addMapping(accountID, tenantID int64, roles ...domain.RoleAssignment) {
	key := mappingKey{accountID, tenantID}
	s.mappings = append(s.mappings, key)
	s.roles[key] = append(s.roles[key], roles...)
}

func (s *fakeStore) sessionsOf(accountID int64) []domain.AccountSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result []domain.AccountSession
	for _, session := range s.sessions {
		if session.AccountID == accountID {
			result = append(result, session)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func stats() domain.OperationStats { return domain.OperationStats{Queries: 1} }

// ports.AccountLoginStore

func (s *fakeStore) HasActiveAccountTenant(_ context.Context, accountID, tenantID int64) (bool, domain.OperationStats, error) {
	s.record("HasActiveAccountTenant")
	if s.hasMappingErr != nil {
		return false, stats(), s.hasMappingErr
	}
	key := mappingKey{accountID, tenantID}
	for _, mapping := range s.mappings {
		if mapping == key && !s.inactive[key] {
			return true, stats(), nil
		}
	}
	return false, stats(), nil
}

func (s *fakeStore) FindLoginAccountByEmail(_ context.Context, email string) (domain.LoginAccount, bool, domain.OperationStats, error) {
	if s.findAccountByEmail != nil {
		return domain.LoginAccount{}, false, stats(), s.findAccountByEmail
	}
	for _, account := range s.accounts {
		if strings.EqualFold(account.Email, email) {
			return account, true, stats(), nil
		}
	}
	return domain.LoginAccount{}, false, stats(), nil
}

func (s *fakeStore) FindLoginAccount(_ context.Context, id int64, forUpdate bool) (domain.LoginAccount, bool, domain.OperationStats, error) {
	if forUpdate {
		s.record("FindLoginAccount(forUpdate)")
	} else {
		s.record("FindLoginAccount")
	}
	if s.findAccountErr != nil {
		return domain.LoginAccount{}, false, stats(), s.findAccountErr
	}
	account, ok := s.accounts[id]
	return account, ok, stats(), nil
}

func (s *fakeStore) RecordAccountLogin(_ context.Context, id int64, _ time.Time) (domain.OperationStats, error) {
	s.record("RecordAccountLogin")
	return stats(), s.recordLoginErr
}

func (s *fakeStore) ListActiveTenantIDs(_ context.Context, accountID int64) ([]int64, domain.OperationStats, error) {
	if s.listTenantsErr != nil {
		return nil, stats(), s.listTenantsErr
	}
	var ids []int64
	for _, mapping := range s.mappings {
		if mapping.accountID == accountID && !s.inactive[mapping] {
			ids = append(ids, mapping.tenantID)
		}
	}
	return ids, stats(), nil
}

func (s *fakeStore) LockActiveTenantMappingShared(ctx context.Context, accountID, tenantID int64) (bool, domain.OperationStats, error) {
	s.record("LockActiveTenantMappingShared")
	if s.lockMappingErr != nil {
		return false, stats(), s.lockMappingErr
	}
	return s.HasActiveAccountTenant(ctx, accountID, tenantID)
}

func (s *fakeStore) ListAccountRolesAtTenant(_ context.Context, accountID, tenantID int64, forShare bool) ([]domain.RoleAssignment, domain.OperationStats, error) {
	if forShare {
		s.record("ListAccountRolesAtTenant(forShare)")
	} else {
		s.record("ListAccountRolesAtTenant")
	}
	if s.listRolesErr != nil {
		return nil, stats(), s.listRolesErr
	}
	return append([]domain.RoleAssignment(nil), s.roles[mappingKey{accountID, tenantID}]...), stats(), nil
}

func (s *fakeStore) ListAccountPermissionsAtTenant(_ context.Context, accountID, tenantID int64) ([]string, domain.OperationStats, error) {
	s.record("ListAccountPermissionsAtTenant")
	if s.listPermissionsErr != nil {
		return nil, stats(), s.listPermissionsErr
	}
	return append([]string(nil), s.permissions[mappingKey{accountID, tenantID}]...), stats(), nil
}

func (s *fakeStore) LockAccountPermissionSources(_ context.Context, _, _ int64) (domain.OperationStats, error) {
	s.record("LockAccountPermissionSources")
	return stats(), s.lockPermissionsErr
}

// ports.Store (only what the session service touches on these paths)

func (s *fakeStore) ListSchoolRoles(context.Context, int64) ([]*domain.SchoolRole, domain.OperationStats, error) {
	panic("not used")
}
func (s *fakeStore) FindSchoolRoleByName(context.Context, string, int64) (*domain.SchoolRole, domain.OperationStats, error) {
	panic("not used")
}
func (s *fakeStore) FindRolePermissions(context.Context, int64, int64) ([]string, domain.OperationStats, error) {
	panic("not used")
}
func (s *fakeStore) FindInvitedPersonIDs(context.Context, string, int64) ([]int64, domain.OperationStats, error) {
	panic("not used")
}
func (s *fakeStore) CountStudentGuardianInvitations(context.Context, int64, int64) (int, domain.OperationStats, error) {
	panic("not used")
}
func (s *fakeStore) ListAccountRoleIDs(context.Context, int64, int64) ([]int64, domain.OperationStats, error) {
	panic("not used")
}
func (s *fakeStore) ListActiveAccountIDs(context.Context, int64) ([]int64, domain.OperationStats, error) {
	panic("not used")
}
func (s *fakeStore) FindRFIDCard(context.Context, string, int64) (string, bool, domain.OperationStats, error) {
	panic("not used")
}
func (s *fakeStore) FindAccount(_ context.Context, id int64) (domain.Account, bool, domain.OperationStats, error) {
	account, ok := s.accounts[id]
	return domain.Account{ID: account.ID, Email: account.Email}, ok, stats(), nil
}
func (s *fakeStore) FindAccountByEmail(context.Context, string) (domain.Account, bool, domain.OperationStats, error) {
	panic("not used")
}
func (s *fakeStore) FindAccountsByEmails(context.Context, []string) (map[string]domain.Account, domain.OperationStats, error) {
	panic("not used")
}
func (s *fakeStore) EnsureActiveTenantMapping(context.Context, int64, int64) (domain.OperationStats, error) {
	panic("not used")
}
func (s *fakeStore) FindRoleByName(context.Context, string, int64) (int64, bool, domain.OperationStats, error) {
	panic("not used")
}
func (s *fakeStore) AssignAccountRole(context.Context, int64, int64, int64) (bool, domain.OperationStats, error) {
	panic("not used")
}

// ports.OperatorStore

func (s *fakeStore) ListOperators(context.Context) ([]domain.Operator, domain.OperationStats, error) {
	panic("not used")
}

// InsertOperator serves the invitation acceptance, which is the one flow
// that creates an operator (#3332).
func (s *fakeStore) InsertOperator(_ context.Context, operator domain.Operator) (domain.Operator, domain.OperationStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, "InsertOperator")
	if s.insertOperatorErr != nil {
		return domain.Operator{}, stats(), s.insertOperatorErr
	}
	s.nextOperatorID++
	operator.ID = s.nextOperatorID
	operator.CreatedAt = time.Now()
	operator.UpdatedAt = operator.CreatedAt
	s.operators[operator.ID] = operator
	return operator, stats(), nil
}
func (s *fakeStore) DeleteOperator(context.Context, int64) (domain.OperationStats, error) {
	panic("not used")
}

// IncrementOperatorMFAAttempts models the single UPDATE: the counter grows
// and, exactly when this increment reaches the threshold, the cooldown
// stamp is written.
func (s *fakeStore) IncrementOperatorMFAAttempts(_ context.Context, id int64, threshold int, lockedUntil time.Time) (domain.OperatorMFAAttempts, bool, domain.OperationStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	operator, ok := s.operators[id]
	if !ok {
		return domain.OperatorMFAAttempts{}, false, domain.OperationStats{}, nil
	}
	operator.MFAAttempts++
	if operator.MFAAttempts >= threshold {
		stamp := lockedUntil
		operator.MFALockedUntil = &stamp
	}
	s.operators[id] = operator
	return domain.OperatorMFAAttempts{Attempts: operator.MFAAttempts, LockedUntil: operator.MFALockedUntil}, true, domain.OperationStats{}, nil
}

func (s *fakeStore) ResetOperatorMFAAttempts(_ context.Context, id int64) (domain.OperationStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if operator, ok := s.operators[id]; ok {
		operator.MFAAttempts = 0
		operator.MFALockedUntil = nil
		s.operators[id] = operator
	}
	return domain.OperationStats{}, nil
}
func (s *fakeStore) DeleteOperatorSession(context.Context, int64) (domain.OperationStats, error) {
	panic("not used")
}
func (s *fakeStore) DeleteExpiredOperatorSessions(context.Context, time.Time) (int, domain.OperationStats, error) {
	panic("not used")
}

// ports.AccountSessionStore

func (s *fakeStore) FindAccountSessionByToken(_ context.Context, token string, forUpdate bool) (domain.AccountSession, bool, domain.OperationStats, error) {
	if forUpdate {
		s.record("FindAccountSessionByToken(forUpdate)")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, session := range s.sessions {
		if session.Token == token {
			return session, true, stats(), nil
		}
	}
	return domain.AccountSession{}, false, stats(), nil
}

func (s *fakeStore) LatestAccountSessionInFamily(_ context.Context, familyID string) (domain.AccountSession, bool, domain.OperationStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var latest domain.AccountSession
	found := false
	for _, session := range s.sessions {
		if session.FamilyID == familyID && (!found || session.Generation > latest.Generation) {
			latest, found = session, true
		}
	}
	return latest, found, stats(), nil
}

func (s *fakeStore) ListAccountSessions(_ context.Context, filter domain.AccountSessionFilter, now time.Time) ([]domain.AccountSession, domain.OperationStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result []domain.AccountSession
	for _, session := range s.sessions {
		if filter.AccountID != 0 && session.AccountID != filter.AccountID {
			continue
		}
		if filter.FamilyID != "" && session.FamilyID != filter.FamilyID {
			continue
		}
		if filter.Liveness == domain.AccountSessionsLive && !session.Expiry.After(now) {
			continue
		}
		if filter.Liveness == domain.AccountSessionsExpired && session.Expiry.After(now) {
			continue
		}
		result = append(result, session)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, stats(), nil
}

func (s *fakeStore) CountExpiredAccountSessions(_ context.Context, now time.Time) (int, domain.OperationStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for _, session := range s.sessions {
		if !session.Expiry.After(now) {
			count++
		}
	}
	return count, stats(), nil
}

func (s *fakeStore) ListInactiveAccountIDsWithLiveSessions(_ context.Context, now time.Time) ([]int64, domain.OperationStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	seen := map[int64]bool{}
	var ids []int64
	for _, session := range s.sessions {
		account := s.accounts[session.AccountID]
		if !account.Active && session.RotatedAt == nil && session.Expiry.After(now) && !seen[session.AccountID] {
			seen[session.AccountID] = true
			ids = append(ids, session.AccountID)
		}
	}
	return ids, stats(), nil
}

func (s *fakeStore) HasLiveAccountSessionsCreatedAfter(_ context.Context, accountID int64, since, now time.Time) (bool, domain.OperationStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, session := range s.sessions {
		if session.AccountID == accountID && session.CreatedAt.After(since) && session.Expiry.After(now) && session.RotatedAt == nil {
			return true, stats(), nil
		}
	}
	return false, stats(), nil
}

func (s *fakeStore) InsertAccountSession(_ context.Context, session domain.AccountSession) (domain.AccountSession, domain.OperationStats, error) {
	s.record("InsertAccountSession")
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.insertSessionErrors) > 0 {
		err := s.insertSessionErrors[0]
		s.insertSessionErrors = s.insertSessionErrors[1:]
		if err != nil {
			return domain.AccountSession{}, stats(), err
		}
	} else if s.insertSessionErr != nil {
		return domain.AccountSession{}, stats(), s.insertSessionErr
	}
	for _, existing := range s.sessions {
		if existing.FamilyID != "" && existing.FamilyID == session.FamilyID && existing.Generation == session.Generation {
			return domain.AccountSession{}, stats(), errors.New(`duplicate key value violates unique constraint "uk_tokens_family_generation"`)
		}
	}
	s.nextSessionID++
	session.ID = s.nextSessionID
	if session.CreatedAt.IsZero() {
		session.CreatedAt = time.Now()
	}
	s.sessions[session.ID] = session
	return session, stats(), nil
}

func (s *fakeStore) MarkAccountSessionRotated(_ context.Context, id int64, replacementToken string, recoveryProofHash []byte, rotatedAt time.Time) (bool, domain.OperationStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[id]
	if !ok || session.RotatedAt != nil {
		return false, stats(), nil
	}
	session.RotatedAt = &rotatedAt
	session.ReplacementToken = &replacementToken
	session.RecoveryProofHash = recoveryProofHash
	s.sessions[id] = session
	return true, stats(), nil
}

func (s *fakeStore) DeleteExpiredRotatedAccountSessions(_ context.Context, accountID int64, now time.Time) (domain.OperationStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, session := range s.sessions {
		if session.AccountID == accountID && session.RotatedAt != nil && !session.Expiry.After(now) {
			delete(s.sessions, id)
		}
	}
	return stats(), nil
}

func (s *fakeStore) RetireAccountSessionFamily(_ context.Context, accountID int64, familyID string, expiry time.Time) (domain.OperationStats, error) {
	s.record("RetireAccountSessionFamily")
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, session := range s.sessions {
		if session.AccountID == accountID && session.FamilyID == familyID && session.Expiry.After(expiry) {
			session.Expiry = expiry
			cap := expiry
			session.FamilyExpiryCap = &cap
			s.sessions[id] = session
		}
	}
	return stats(), nil
}

func (s *fakeStore) ListLiveAccountSessionsForCap(_ context.Context, accountID int64, portalScopes []string, now time.Time) ([]domain.AccountSession, domain.OperationStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result []domain.AccountSession
	for _, session := range s.sessions {
		if session.AccountID != accountID || session.RotatedAt != nil || !session.Expiry.After(now) {
			continue
		}
		for _, scope := range portalScopes {
			if session.PortalScope == scope {
				result = append(result, session)
				break
			}
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Expiry.After(result[j].Expiry) })
	return result, stats(), nil
}

func (s *fakeStore) DeleteAccountSessionsByID(_ context.Context, ids []int64) ([]domain.AccountSession, domain.OperationStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var deleted []domain.AccountSession
	for _, id := range ids {
		if session, ok := s.sessions[id]; ok {
			deleted = append(deleted, session)
			delete(s.sessions, id)
		}
	}
	return deleted, stats(), nil
}

func (s *fakeStore) DeleteAccountSession(_ context.Context, id int64) (domain.OperationStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
	return stats(), nil
}

func (s *fakeStore) DeleteAccountSessionsByFamily(_ context.Context, familyID string) ([]domain.AccountSession, domain.OperationStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var deleted []domain.AccountSession
	for id, session := range s.sessions {
		if session.FamilyID == familyID {
			deleted = append(deleted, session)
			delete(s.sessions, id)
		}
	}
	return deleted, stats(), nil
}

func (s *fakeStore) DeleteAccountSessionsByAccount(ctx context.Context, accountID int64, tenantScoped bool, cutoff time.Time) ([]domain.AccountSession, domain.OperationStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var deleted []domain.AccountSession
	for id, session := range s.sessions {
		if session.AccountID != accountID {
			continue
		}
		if !cutoff.IsZero() && session.CreatedAt.After(cutoff) {
			continue
		}
		deleted = append(deleted, session)
		delete(s.sessions, id)
	}
	return deleted, stats(), nil
}

func (s *fakeStore) DeleteAccountSessionsByTenant(_ context.Context, tenantID int64) ([]domain.AccountSession, domain.OperationStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var deleted []domain.AccountSession
	for id, session := range s.sessions {
		if session.TenantID == tenantID {
			deleted = append(deleted, session)
			delete(s.sessions, id)
		}
	}
	return deleted, stats(), nil
}

func (s *fakeStore) DeleteExpiredAccountSessions(_ context.Context, now time.Time) (int, domain.OperationStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for id, session := range s.sessions {
		if !session.Expiry.After(now) {
			delete(s.sessions, id)
			count++
		}
	}
	return count, stats(), nil
}

type fakeTransaction struct{}

func (fakeTransaction) RunWrite(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}
func (fakeTransaction) RunRead(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}
func (fakeTransaction) RunPlatform(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

// --- foreign facts --------------------------------------------------------

type fakeSchools struct {
	schools       map[int64]domain.School
	subdomains    map[string]int64
	findErr       error
	subdomainErr  error
	lockErr       error
	listActiveErr error
	calls         []string
}

func newFakeSchools() *fakeSchools {
	return &fakeSchools{schools: map[int64]domain.School{}, subdomains: map[string]int64{}}
}

func (f *fakeSchools) add(id, orgID int64, subdomain string, active, deleted bool) {
	f.schools[id] = domain.School{ID: id, OrganizationID: orgID, Active: active, Deleted: deleted}
	f.subdomains[subdomain] = id
}

func (f *fakeSchools) FindSchool(_ context.Context, id int64) (domain.School, bool, error) {
	f.calls = append(f.calls, "FindSchool")
	if f.findErr != nil {
		return domain.School{}, false, f.findErr
	}
	school, ok := f.schools[id]
	return school, ok, nil
}

func (f *fakeSchools) FindSchoolBySubdomain(_ context.Context, subdomain string) (domain.School, bool, error) {
	if f.subdomainErr != nil {
		return domain.School{}, false, f.subdomainErr
	}
	id, ok := f.subdomains[subdomain]
	if !ok {
		return domain.School{}, false, nil
	}
	return f.schools[id], true, nil
}

func (f *fakeSchools) LockSchoolShared(_ context.Context, id int64) (domain.School, bool, error) {
	f.calls = append(f.calls, "LockSchoolShared")
	if f.lockErr != nil {
		return domain.School{}, false, f.lockErr
	}
	school, ok := f.schools[id]
	return school, ok, nil
}

func (f *fakeSchools) ListActiveSchoolsByID(_ context.Context, schoolIDs []int64) ([]domain.School, error) {
	if f.listActiveErr != nil {
		return nil, f.listActiveErr
	}
	var result []domain.School
	for _, id := range schoolIDs {
		school, found := f.schools[id]
		if found && school.Live() {
			result = append(result, school)
		}
	}
	return result, nil
}

type personKey struct{ accountID, tenantID int64 }

type fakePersons struct {
	names   map[personKey][2]string
	findErr error
	runtime *fakeRuntime
}

func newFakePersons(runtime *fakeRuntime) *fakePersons {
	return &fakePersons{names: map[personKey][2]string{}, runtime: runtime}
}

func (f *fakePersons) FindPersonName(ctx context.Context, accountID int64) (string, string, bool, error) {
	if f.findErr != nil {
		return "", "", false, f.findErr
	}
	name, ok := f.names[personKey{accountID, f.runtime.TenantID(ctx)}]
	if !ok {
		return "", "", false, nil
	}
	return name[0], name[1], true, nil
}

type fakePasswords struct{}

func (fakePasswords) VerifyPassword(password, hash string) (bool, error) {
	return hash == "hash:"+password, nil
}

// fakeCodec encodes claims as prefixed JSON so tests can read them back.
type fakeCodec struct {
	issueErr error
}

func (fakeCodec) encode(prefix string, value any) string {
	raw, _ := json.Marshal(value)
	return prefix + string(raw)
}

func (c fakeCodec) IssueTokenPair(access domain.SessionClaims, refresh domain.RefreshClaims) (string, string, error) {
	if c.issueErr != nil {
		return "", "", c.issueErr
	}
	return c.encode("access:", access), c.encode("refresh:", refresh), nil
}

func (c fakeCodec) IssueMFAEnrollmentToken(accountID, tenantID int64, scope string, _ time.Duration) (string, error) {
	return fmt.Sprintf("enroll:%d:%d:%s", accountID, tenantID, scope), nil
}

func (fakeCodec) ParseAccessToken(token string) (domain.SessionClaims, error) {
	var claims domain.SessionClaims
	if !strings.HasPrefix(token, "access:") {
		return claims, errors.New("not an access token")
	}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(token, "access:")), &claims); err != nil {
		return claims, err
	}
	if claims.ExpiresAt > 0 && claims.ExpiresAt < time.Now().Unix() {
		return claims, errors.New("access token expired")
	}
	return claims, nil
}

func (fakeCodec) ParseRefreshToken(token string) (domain.RefreshClaims, error) {
	var claims domain.RefreshClaims
	if !strings.HasPrefix(token, "refresh:") {
		return claims, errors.New("not a refresh token")
	}
	err := json.Unmarshal([]byte(strings.TrimPrefix(token, "refresh:")), &claims)
	return claims, err
}

func (fakeCodec) RefreshExpiry() time.Duration { return time.Hour }

func decodeAccess(t *testing.T, token string) domain.SessionClaims {
	t.Helper()
	claims, err := fakeCodec{}.ParseAccessToken(token)
	require.NoError(t, err)
	return claims
}

func decodeRefresh(t *testing.T, token string) domain.RefreshClaims {
	t.Helper()
	claims, err := fakeCodec{}.ParseRefreshToken(token)
	require.NoError(t, err)
	return claims
}

type fakePolicy struct{ required func([]string) bool }

func (p fakePolicy) RequiredFor(roleNames []string) bool { return p.required(roleNames) }

type fakeMFA struct {
	configured          bool
	required            bool
	requiredErr         error
	enrolled            bool
	enrolledErr         error
	trusted             bool
	policy              ports.MFAPolicy
	policyErr           error
	policyInTx          ports.MFAPolicy
	policyInTxErr       error
	policyInTxInAdminTx []bool
	challenge           string
	challengeErr        error
	challenges          []string
}

func (m *fakeMFA) Configured() bool { return m.configured }
func (m *fakeMFA) IsRequired(context.Context, int64, string, []string, int64) (bool, error) {
	return m.required, m.requiredErr
}
func (m *fakeMFA) ResolvePolicy(context.Context, int64, int64) (ports.MFAPolicy, error) {
	if m.policyErr != nil {
		return nil, m.policyErr
	}
	if m.policy != nil {
		return m.policy, nil
	}
	return fakePolicy{func([]string) bool { return m.required }}, nil
}
func (m *fakeMFA) ResolvePolicyInTx(ctx context.Context, _, _ int64) (ports.MFAPolicy, error) {
	m.policyInTxInAdminTx = append(m.policyInTxInAdminTx, ctx.Value(fakeAdminKey{}) == true)
	if m.policyInTxErr != nil {
		return nil, m.policyInTxErr
	}
	if m.policyInTx != nil {
		return m.policyInTx, nil
	}
	return fakePolicy{func([]string) bool { return false }}, nil
}
func (m *fakeMFA) HasEnrollment(context.Context, int64) (bool, error) {
	return m.enrolled, m.enrolledErr
}
func (m *fakeMFA) VerifyTrustedDevice(context.Context, int64, int64, string) (bool, error) {
	return m.trusted, nil
}
func (m *fakeMFA) StartChallenge(_ context.Context, _, _ int64, scope, _ string) (string, error) {
	m.challenges = append(m.challenges, scope)
	if m.challengeErr != nil {
		return "", m.challengeErr
	}
	if m.challenge == "" {
		return "challenge-token", nil
	}
	return m.challenge, nil
}
func (m *fakeMFA) IsTrustedDeviceEnabled(context.Context, int64) bool { return true }
func (m *fakeMFA) TrustedDeviceDays(context.Context, int64) int       { return 30 }

type fakeMFALock struct {
	err   error
	calls []string
	store *fakeStore
}

func (l *fakeMFALock) LockMFAPolicySharedForTenant(context.Context, int64) error {
	l.calls = append(l.calls, "LockMFAPolicy")
	if l.store != nil {
		l.store.record("LockMFAPolicy")
	}
	return l.err
}

type fakeAudit struct {
	mu      sync.Mutex
	events  []domain.AuthEvent
	pending []domain.PendingAccountWideWipe
	err     error
}

func (a *fakeAudit) RecordAuthEvent(_ context.Context, event domain.AuthEvent) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.err != nil {
		return a.err
	}
	a.events = append(a.events, event)
	return nil
}

func (a *fakeAudit) ListPendingAccountWideWipes(context.Context) ([]domain.PendingAccountWideWipe, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]domain.PendingAccountWideWipe(nil), a.pending...), nil
}

func (a *fakeAudit) ClaimPendingAccountWideWipes(_ context.Context, accountID int64) ([]domain.PendingAccountWideWipe, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	var claimed, rest []domain.PendingAccountWideWipe
	for _, wipe := range a.pending {
		if wipe.AccountID == accountID {
			claimed = append(claimed, wipe)
		} else {
			rest = append(rest, wipe)
		}
	}
	a.pending = rest
	return claimed, nil
}

func (a *fakeAudit) eventsOfType(eventType string) []domain.AuthEvent {
	a.mu.Lock()
	defer a.mu.Unlock()
	var result []domain.AuthEvent
	for _, event := range a.events {
		if event.Type == eventType {
			result = append(result, event)
		}
	}
	return result
}

type fakePush struct {
	mu    sync.Mutex
	calls []string
}

func (p *fakePush) record(call string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls = append(p.calls, call)
}

func (p *fakePush) DeleteStaffByAccount(_ context.Context, accountID int64) error {
	p.record(fmt.Sprintf("staff:%d", accountID))
	return nil
}
func (p *fakePush) DeleteSchoolByAccount(_ context.Context, accountID int64) error {
	p.record(fmt.Sprintf("school:%d", accountID))
	return nil
}
func (p *fakePush) DeleteParentByAccount(_ context.Context, accountID int64) error {
	p.record(fmt.Sprintf("parent:%d", accountID))
	return nil
}
func (p *fakePush) DeleteByTokenFamily(_ context.Context, accountID int64, familyID string) error {
	p.record(fmt.Sprintf("family:%d:%s", accountID, familyID))
	return nil
}
func (p *fakePush) DeleteUnboundByAccount(_ context.Context, accountID, tenantID int64, portal string) error {
	p.record(fmt.Sprintf("unbound:%s:%d:%d", portal, accountID, tenantID))
	return nil
}
func (p *fakePush) DeleteOrphaned(context.Context) error {
	p.record("orphaned")
	return nil
}

// --- fixture ----------------------------------------------------------------

type authFixture struct {
	auth     *AccountAuthentication
	store    *fakeStore
	schools  *fakeSchools
	persons  *fakePersons
	mfa      *fakeMFA
	mfaLock  *fakeMFALock
	audit    *fakeAudit
	push     *fakePush
	runtime  *fakeRuntime
	sessions *Service
}

func newAuthFixture(t *testing.T) *authFixture {
	t.Helper()
	store := newFakeStore()
	runtime := &fakeRuntime{}
	sessions := New(store, store, store, fakeTransaction{}, runtime.TenantID, func(ports.Observation) {})
	schools := newFakeSchools()
	persons := newFakePersons(runtime)
	mfa := &fakeMFA{}
	lock := &fakeMFALock{store: store}
	audit := &fakeAudit{}
	push := &fakePush{}
	auth, err := NewAccountAuthentication(sessions, AccountAuthenticationDependencies{
		Store: store, Schools: schools, Persons: persons, Passwords: fakePasswords{}, Codec: fakeCodec{},
		MFA: mfa, MFALock: lock, Audit: audit, Push: push, Runtime: runtime, Rotation: fakeRotation{},
		Logger: slog.New(slog.NewTextHandler(testWriter{t}, nil)),
	})
	require.NoError(t, err)
	return &authFixture{auth: auth, store: store, schools: schools, persons: persons, mfa: mfa, mfaLock: lock, audit: audit, push: push, runtime: runtime, sessions: sessions}
}

type testWriter struct{ t *testing.T }

func (w testWriter) Write(p []byte) (int, error) {
	w.t.Log(strings.TrimSpace(string(p)))
	return len(p), nil
}

// tenantLogin seeds an active tenant-portal account at one live school.
func (f *authFixture) seedStaff(accountID, tenantID int64, roleNames ...string) {
	f.store.addAccount(accountID, fmt.Sprintf("user%d@example.com", accountID), "hash:secret", true)
	roles := make([]domain.RoleAssignment, 0, len(roleNames))
	for i, name := range roleNames {
		roles = append(roles, domain.RoleAssignment{RoleID: int64(100 + i), Name: name})
	}
	f.store.addMapping(accountID, tenantID, roles...)
	if _, ok := f.schools.schools[tenantID]; !ok {
		f.schools.add(tenantID, tenantID*10, fmt.Sprintf("school-%d", tenantID), true, false)
	}
}

func lehrkraftRole() domain.RoleAssignment {
	return domain.RoleAssignment{RoleID: 7, Name: "lehrkraft", IsSystem: true}
}
