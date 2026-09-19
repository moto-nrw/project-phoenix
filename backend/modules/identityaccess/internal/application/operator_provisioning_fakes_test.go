package application

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// The doubles the operator invitation and e-mail change flows (#3332) run
// against: an in-memory operator token store and the two mail transports.

type fakeOperatorTokenStore struct {
	mu sync.Mutex

	invitations  map[int64]domain.OperatorInvitation
	emailChanges map[int64]domain.OperatorEmailChange
	nextID       int64

	// calls records the statements in order, so a test can assert that the
	// rate limit ran before anything was spent.
	calls []string

	insertInvitationErr  error
	countInvitationsErr  error
	revokeForEmailErr    error
	redeemInvitationErr  error
	extendInvitationErr  error
	recordDeliveryErr    error
	insertEmailChangeErr error
	countEmailChangesErr error
	revokeChangesErr     error
	redeemChangeErr      error
	revokeExpiredErr     error
	deleteStaleErr       error
}

func newFakeOperatorTokenStore() *fakeOperatorTokenStore {
	return &fakeOperatorTokenStore{
		invitations:  map[int64]domain.OperatorInvitation{},
		emailChanges: map[int64]domain.OperatorEmailChange{},
	}
}

func (s *fakeOperatorTokenStore) record(call string) {
	s.calls = append(s.calls, call)
}

func (s *fakeOperatorTokenStore) recorded() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.calls...)
}

func (s *fakeOperatorTokenStore) nextIdentity() int64 {
	s.nextID++
	return s.nextID
}

func (s *fakeOperatorTokenStore) InsertOperatorInvitation(_ context.Context, invitation domain.OperatorInvitation) (domain.OperatorInvitation, domain.OperationStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record("InsertOperatorInvitation")
	if s.insertInvitationErr != nil {
		return domain.OperatorInvitation{}, stats(), s.insertInvitationErr
	}
	invitation.ID = s.nextIdentity()
	invitation.CreatedAt = time.Now()
	invitation.UpdatedAt = invitation.CreatedAt
	s.invitations[invitation.ID] = invitation
	return invitation, stats(), nil
}

func (s *fakeOperatorTokenStore) FindOperatorInvitation(_ context.Context, id int64) (domain.OperatorInvitation, bool, domain.OperationStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record("FindOperatorInvitation")
	invitation, ok := s.invitations[id]
	return invitation, ok, stats(), nil
}

func (s *fakeOperatorTokenStore) FindRedeemableOperatorInvitation(_ context.Context, token string, now time.Time) (domain.OperatorInvitation, bool, domain.OperationStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record("FindRedeemableOperatorInvitation")
	for _, invitation := range s.invitations {
		if invitation.Token == token && invitation.UsedAt == nil && invitation.ExpiresAt.After(now) {
			return invitation, true, stats(), nil
		}
	}
	return domain.OperatorInvitation{}, false, stats(), nil
}

func (s *fakeOperatorTokenStore) ListRedeemableOperatorInvitations(_ context.Context, now time.Time) ([]domain.OperatorInvitation, domain.OperationStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record("ListRedeemableOperatorInvitations")
	var result []domain.OperatorInvitation
	for _, invitation := range s.invitations {
		if invitation.UsedAt == nil && invitation.ExpiresAt.After(now) {
			result = append(result, invitation)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID > result[j].ID })
	return result, stats(), nil
}

func (s *fakeOperatorTokenStore) CountOperatorInvitationsCreatedAfter(_ context.Context, createdBy int64, since time.Time) (int, domain.OperationStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record("CountOperatorInvitationsCreatedAfter")
	if s.countInvitationsErr != nil {
		return 0, stats(), s.countInvitationsErr
	}
	count := 0
	for _, invitation := range s.invitations {
		if invitation.CreatedBy == createdBy && invitation.CreatedAt.After(since) {
			count++
		}
	}
	return count, stats(), nil
}

func (s *fakeOperatorTokenStore) RedeemOperatorInvitation(_ context.Context, token string, now time.Time) (domain.OperatorInvitation, bool, domain.OperationStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record("RedeemOperatorInvitation")
	if s.redeemInvitationErr != nil {
		return domain.OperatorInvitation{}, false, stats(), s.redeemInvitationErr
	}
	for id, invitation := range s.invitations {
		if invitation.Token != token || invitation.UsedAt != nil || !invitation.ExpiresAt.After(now) {
			continue
		}
		spent := now
		invitation.UsedAt = &spent
		s.invitations[id] = invitation
		return invitation, true, stats(), nil
	}
	return domain.OperatorInvitation{}, false, stats(), nil
}

func (s *fakeOperatorTokenStore) RevokeOperatorInvitation(_ context.Context, id int64, now time.Time) (bool, domain.OperationStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record("RevokeOperatorInvitation")
	invitation, ok := s.invitations[id]
	if !ok || invitation.UsedAt != nil || !invitation.ExpiresAt.After(now) {
		return false, stats(), nil
	}
	spent := now
	invitation.UsedAt = &spent
	s.invitations[id] = invitation
	return true, stats(), nil
}

func (s *fakeOperatorTokenStore) RevokeOperatorInvitationsForEmail(_ context.Context, email string, now time.Time) (int, domain.OperationStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record("RevokeOperatorInvitationsForEmail")
	if s.revokeForEmailErr != nil {
		return 0, stats(), s.revokeForEmailErr
	}
	revoked := 0
	for id, invitation := range s.invitations {
		if invitation.Email != email || invitation.UsedAt != nil {
			continue
		}
		spent := now
		invitation.UsedAt = &spent
		s.invitations[id] = invitation
		revoked++
	}
	return revoked, stats(), nil
}

func (s *fakeOperatorTokenStore) ExtendOperatorInvitation(_ context.Context, id int64, expiresAt, now time.Time) (bool, domain.OperationStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record("ExtendOperatorInvitation")
	if s.extendInvitationErr != nil {
		return false, stats(), s.extendInvitationErr
	}
	invitation, ok := s.invitations[id]
	if !ok || invitation.UsedAt != nil || !invitation.ExpiresAt.After(now) {
		return false, stats(), nil
	}
	invitation.ExpiresAt = expiresAt
	s.invitations[id] = invitation
	return true, stats(), nil
}

func (s *fakeOperatorTokenStore) RecordOperatorInvitationDelivery(_ context.Context, id int64, delivery domain.TokenDelivery) (domain.OperationStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record("RecordOperatorInvitationDelivery")
	if s.recordDeliveryErr != nil {
		return stats(), s.recordDeliveryErr
	}
	invitation, ok := s.invitations[id]
	if !ok {
		return stats(), nil
	}
	invitation.Delivery = delivery
	s.invitations[id] = invitation
	return stats(), nil
}

func (s *fakeOperatorTokenStore) DeleteExpiredOperatorInvitations(_ context.Context, now time.Time) (int, domain.OperationStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record("DeleteExpiredOperatorInvitations")
	deleted := 0
	for id, invitation := range s.invitations {
		if !invitation.ExpiresAt.After(now) {
			delete(s.invitations, id)
			deleted++
		}
	}
	return deleted, stats(), nil
}

func (s *fakeOperatorTokenStore) InsertOperatorEmailChange(_ context.Context, change domain.OperatorEmailChange) (domain.OperatorEmailChange, domain.OperationStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record("InsertOperatorEmailChange")
	if s.insertEmailChangeErr != nil {
		return domain.OperatorEmailChange{}, stats(), s.insertEmailChangeErr
	}
	change.ID = s.nextIdentity()
	change.CreatedAt = time.Now()
	change.UpdatedAt = change.CreatedAt
	s.emailChanges[change.ID] = change
	return change, stats(), nil
}

func (s *fakeOperatorTokenStore) CountOperatorEmailChangesCreatedAfter(_ context.Context, operatorID int64, since time.Time) (int, domain.OperationStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record("CountOperatorEmailChangesCreatedAfter")
	if s.countEmailChangesErr != nil {
		return 0, stats(), s.countEmailChangesErr
	}
	count := 0
	for _, change := range s.emailChanges {
		if change.OperatorID == operatorID && change.CreatedAt.After(since) {
			count++
		}
	}
	return count, stats(), nil
}

func (s *fakeOperatorTokenStore) RedeemOperatorEmailChange(_ context.Context, token string, now time.Time) (domain.OperatorEmailChange, bool, domain.OperationStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record("RedeemOperatorEmailChange")
	if s.redeemChangeErr != nil {
		return domain.OperatorEmailChange{}, false, stats(), s.redeemChangeErr
	}
	for id, change := range s.emailChanges {
		if change.Token != token || change.Used || !change.Expiry.After(now) {
			continue
		}
		change.Used = true
		s.emailChanges[id] = change
		return change, true, stats(), nil
	}
	return domain.OperatorEmailChange{}, false, stats(), nil
}

func (s *fakeOperatorTokenStore) RevokeOperatorEmailChanges(_ context.Context, operatorID int64) (domain.OperationStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record("RevokeOperatorEmailChanges")
	if s.revokeChangesErr != nil {
		return stats(), s.revokeChangesErr
	}
	for id, change := range s.emailChanges {
		if change.OperatorID == operatorID && !change.Used {
			change.Used = true
			s.emailChanges[id] = change
		}
	}
	return stats(), nil
}

func (s *fakeOperatorTokenStore) RecordOperatorEmailChangeDelivery(_ context.Context, id int64, delivery domain.TokenDelivery) (domain.OperationStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record("RecordOperatorEmailChangeDelivery")
	change, ok := s.emailChanges[id]
	if !ok {
		return stats(), nil
	}
	change.Delivery = delivery
	s.emailChanges[id] = change
	return stats(), nil
}

func (s *fakeOperatorTokenStore) RevokeExpiredOperatorEmailChanges(_ context.Context, now time.Time) (int, domain.OperationStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record("RevokeExpiredOperatorEmailChanges")
	if s.revokeExpiredErr != nil {
		return 0, stats(), s.revokeExpiredErr
	}
	revoked := 0
	for id, change := range s.emailChanges {
		if !change.Used && !change.Expiry.After(now) {
			change.Used = true
			s.emailChanges[id] = change
			revoked++
		}
	}
	return revoked, stats(), nil
}

func (s *fakeOperatorTokenStore) DeleteStaleOperatorEmailChanges(_ context.Context, createdBefore, now time.Time) (int, domain.OperationStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record("DeleteStaleOperatorEmailChanges")
	if s.deleteStaleErr != nil {
		return 0, stats(), s.deleteStaleErr
	}
	deleted := 0
	for id, change := range s.emailChanges {
		if change.CreatedAt.Before(createdBefore) && (change.Used || !change.Expiry.After(now)) {
			delete(s.emailChanges, id)
			deleted++
		}
	}
	return deleted, stats(), nil
}

// --- delivery doubles ------------------------------------------------------

type dispatchedInvitation struct {
	invitation  domain.OperatorInvitation
	inviterName string
}

type fakeOperatorInvitationDelivery struct {
	dispatched []dispatchedInvitation
}

func (d *fakeOperatorInvitationDelivery) DispatchOperatorInvitation(_ context.Context, invitation domain.OperatorInvitation, inviterName string) {
	d.dispatched = append(d.dispatched, dispatchedInvitation{invitation: invitation, inviterName: inviterName})
}

type fakeOperatorEmailChangeDelivery struct {
	verifications []domain.OperatorEmailChange
	requested     []string
	confirmed     []string
}

func (d *fakeOperatorEmailChangeDelivery) DispatchOperatorEmailChangeVerification(_ context.Context, change domain.OperatorEmailChange) {
	d.verifications = append(d.verifications, change)
}

func (d *fakeOperatorEmailChangeDelivery) DispatchOperatorEmailChangeRequested(_ context.Context, operator domain.Operator, maskedNewEmail string) {
	d.requested = append(d.requested, operator.Email+" -> "+maskedNewEmail)
}

func (d *fakeOperatorEmailChangeDelivery) DispatchOperatorEmailChangeConfirmed(_ context.Context, oldEmail, displayName string) {
	d.confirmed = append(d.confirmed, oldEmail+" ("+displayName+")")
}

// rotatingOperatorStore commits a concurrent change just before the
// locking read, so the e-mail change's TOCTOU guard can be exercised. The
// hook lives here rather than on the shared fake store, which every
// composition-surface assignment would then have to account for.
type rotatingOperatorStore struct {
	*fakeStore
	rotate func()
}

func (s *rotatingOperatorStore) FindOperator(ctx context.Context, id int64, forUpdate bool) (domain.Operator, bool, domain.OperationStats, error) {
	if forUpdate && s.rotate != nil {
		s.rotate()
	}
	return s.fakeStore.FindOperator(ctx, id, forUpdate)
}

// fakeEmailFormat mirrors the People Directory rule closely enough for the
// flows: an address needs a local part, an @ and a dotted domain.
type fakeEmailFormat struct{}

func (fakeEmailFormat) IsRoutable(address string) bool {
	parts := strings.SplitN(address, "@", 2)
	return len(parts) == 2 && parts[0] != "" && strings.Contains(parts[1], ".")
}

// --- fixture ----------------------------------------------------------------

type operatorProvisioningFixture struct {
	provisioning *OperatorProvisioning
	store        *fakeStore
	// operators is the store the flows read through; setting its rotate
	// hook commits a change between a flow's first read and its transaction.
	operators *rotatingOperatorStore
	tokens    *fakeOperatorTokenStore
	audit     *fakeOperatorAudit
	invites   *fakeOperatorInvitationDelivery
	changes   *fakeOperatorEmailChangeDelivery
	runtime   *fakeRuntime
	now       time.Time
}

func newOperatorProvisioningFixture(t *testing.T) *operatorProvisioningFixture {
	t.Helper()
	store := newFakeStore()
	rotating := &rotatingOperatorStore{fakeStore: store}
	tokenStore := newFakeOperatorTokenStore()
	runtime := &fakeRuntime{}
	operators := New(store, rotating, store, fakeTransaction{}, runtime.TenantID, func(ports.Observation) {})
	audit := &fakeOperatorAudit{}
	invites := &fakeOperatorInvitationDelivery{}
	changes := &fakeOperatorEmailChangeDelivery{}
	provisioning, err := NewOperatorProvisioning(operators, OperatorProvisioningDependencies{
		Tokens:    NewOperatorTokens(operators, tokenStore),
		Passwords: fakePasswords{},
		Hasher:    fakeHasher{},
		Format:    fakeEmailFormat{},
		Audit:     audit,
		Invites:   invites,
		Changes:   changes,
		Runtime:   runtime,
		Expiry:    OperatorProvisioningExpiry{Invitation: 48 * time.Hour, EmailChange: 30 * time.Minute},
		Logger:    slog.New(slog.NewTextHandler(testWriter{t}, nil)),
	})
	require.NoError(t, err)
	return &operatorProvisioningFixture{
		provisioning: provisioning, store: store, operators: rotating, tokens: tokenStore, audit: audit,
		invites: invites, changes: changes, runtime: runtime, now: time.Now(),
	}
}

// seedInviter adds the active operator an invitation is created by.
func (f *operatorProvisioningFixture) seedInviter(id int64) domain.Operator {
	f.store.addOperator(id, "inviter@example.com", "hash:secret", true)
	return f.store.operators[id]
}

// seedOperatorWithEmail adds an active operator whose password is "secret".
func (f *operatorProvisioningFixture) seedOperatorWithEmail(id int64, email string) domain.Operator {
	f.store.addOperator(id, email, "hash:secret", true)
	return f.store.operators[id]
}

// seedInvitation stores a redeemable invitation for the address.
func (f *operatorProvisioningFixture) seedInvitation(email, token string, createdBy int64) domain.OperatorInvitation {
	stored, _, err := f.tokens.InsertOperatorInvitation(context.Background(), domain.OperatorInvitation{
		Email: email, Token: token, ExpiresAt: time.Now().Add(time.Hour), CreatedBy: createdBy,
	})
	if err != nil {
		panic(err)
	}
	return stored
}

// seedEmailChange stores a redeemable e-mail change link.
func (f *operatorProvisioningFixture) seedEmailChange(operatorID int64, newEmail, token string) domain.OperatorEmailChange {
	stored, _, err := f.tokens.InsertOperatorEmailChange(context.Background(), domain.OperatorEmailChange{
		OperatorID: operatorID, NewEmail: newEmail, Token: token, Expiry: time.Now().Add(time.Hour),
	})
	if err != nil {
		panic(err)
	}
	return stored
}

var errFakeStore = errors.New("store unavailable")
