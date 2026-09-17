package application

import (
	"context"
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

// The operator half of the fake store (ports.OperatorStore) and the doubles
// of the operator-only ports (#3252): the MFA gate, the audit ledger, the
// e-mail change cleanup and the password policy.

func (s *fakeStore) addOperator(id int64, email, passwordHash string, active bool) {
	s.operators[id] = domain.Operator{ID: id, Email: email, DisplayName: "Operator " + fmt.Sprint(id), PasswordHash: passwordHash, Active: active}
}

func (s *fakeStore) operatorSessionsOf(operatorID int64) []domain.OperatorSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result []domain.OperatorSession
	for _, session := range s.operatorTokens {
		if session.OperatorID == operatorID {
			result = append(result, session)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func (s *fakeStore) FindOperator(_ context.Context, id int64, forUpdate bool) (domain.Operator, bool, domain.OperationStats, error) {
	if forUpdate {
		s.record("FindOperator(forUpdate)")
	} else {
		s.record("FindOperator")
	}
	if s.findOperatorErr != nil {
		return domain.Operator{}, false, stats(), s.findOperatorErr
	}
	operator, ok := s.operators[id]
	return operator, ok, stats(), nil
}

func (s *fakeStore) FindOperatorByEmail(_ context.Context, email string) (domain.Operator, bool, domain.OperationStats, error) {
	if s.findOperatorErr != nil {
		return domain.Operator{}, false, stats(), s.findOperatorErr
	}
	for _, operator := range s.operators {
		if operator.Email == email {
			return operator, true, stats(), nil
		}
	}
	return domain.Operator{}, false, stats(), nil
}

func (s *fakeStore) UpdateOperator(_ context.Context, operator domain.Operator) (bool, domain.OperationStats, error) {
	s.record("UpdateOperator")
	if s.updateOperatorErr != nil {
		return false, stats(), s.updateOperatorErr
	}
	if _, ok := s.operators[operator.ID]; !ok {
		return false, stats(), nil
	}
	s.operators[operator.ID] = operator
	return true, stats(), nil
}

func (s *fakeStore) RecordOperatorLogin(_ context.Context, id int64, at time.Time) (domain.OperationStats, error) {
	s.record("RecordOperatorLogin")
	if s.recordLoginErr != nil {
		return stats(), s.recordLoginErr
	}
	if operator, ok := s.operators[id]; ok {
		operator.LastLogin = &at
		s.operators[id] = operator
	}
	return stats(), nil
}

func (s *fakeStore) FindOperatorSessionByToken(_ context.Context, token string, forUpdate bool) (domain.OperatorSession, bool, domain.OperationStats, error) {
	if forUpdate {
		s.record("FindOperatorSessionByToken(forUpdate)")
	}
	if err := s.findSessionErrs[token]; err != nil {
		return domain.OperatorSession{}, false, stats(), err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, session := range s.operatorTokens {
		if session.Token == token {
			return session, true, stats(), nil
		}
	}
	return domain.OperatorSession{}, false, stats(), nil
}

func (s *fakeStore) LatestOperatorSessionInFamily(_ context.Context, familyID string) (domain.OperatorSession, bool, domain.OperationStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var latest domain.OperatorSession
	found := false
	for _, session := range s.operatorTokens {
		if session.FamilyID == familyID && (!found || session.Generation > latest.Generation) {
			latest, found = session, true
		}
	}
	return latest, found, stats(), nil
}

func (s *fakeStore) InsertOperatorSession(_ context.Context, session domain.OperatorSession) (domain.OperatorSession, domain.OperationStats, error) {
	s.record("InsertOperatorSession")
	if s.insertSessionErr != nil {
		return domain.OperatorSession{}, stats(), s.insertSessionErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextSessionID++
	session.ID = s.nextSessionID
	if session.CreatedAt.IsZero() {
		session.CreatedAt = time.Now()
	}
	s.operatorTokens[session.ID] = session
	return session, stats(), nil
}

func (s *fakeStore) MarkOperatorSessionRotated(_ context.Context, id int64, replacementToken string, recoveryProofHash []byte, rotatedAt time.Time) (bool, domain.OperationStats, error) {
	s.record("MarkOperatorSessionRotated")
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.operatorTokens[id]
	if !ok || session.RotatedAt != nil {
		return false, stats(), nil
	}
	session.RotatedAt = &rotatedAt
	session.ReplacementToken = &replacementToken
	session.RecoveryProofHash = recoveryProofHash
	s.operatorTokens[id] = session
	return true, stats(), nil
}

func (s *fakeStore) DeleteExpiredRotatedOperatorSessions(_ context.Context, familyID string, now time.Time) (domain.OperationStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, session := range s.operatorTokens {
		if session.FamilyID == familyID && session.RotatedAt != nil && !session.Expiry.After(now) {
			delete(s.operatorTokens, id)
		}
	}
	return stats(), nil
}

func (s *fakeStore) DeleteOperatorSessionsByOperator(_ context.Context, operatorID int64) ([]domain.OperatorSession, domain.OperationStats, error) {
	s.record("DeleteOperatorSessionsByOperator")
	s.mu.Lock()
	defer s.mu.Unlock()
	var deleted []domain.OperatorSession
	for id, session := range s.operatorTokens {
		if session.OperatorID == operatorID {
			deleted = append(deleted, session)
			delete(s.operatorTokens, id)
		}
	}
	return deleted, stats(), nil
}

func (s *fakeStore) DeleteOperatorSessionsByFamily(_ context.Context, familyID string) ([]domain.OperatorSession, domain.OperationStats, error) {
	s.record("DeleteOperatorSessionsByFamily")
	s.mu.Lock()
	defer s.mu.Unlock()
	var deleted []domain.OperatorSession
	for id, session := range s.operatorTokens {
		if session.FamilyID == familyID {
			deleted = append(deleted, session)
			delete(s.operatorTokens, id)
		}
	}
	return deleted, stats(), nil
}

// --- operator-only ports ---------------------------------------------------

type fakeOperatorMFA struct {
	configured    bool
	enrolled      bool
	enrolledErr   error
	trustedCookie string
	challenge     string
	challengeErr  error
	calls         []string
}

func (m *fakeOperatorMFA) Configured() bool { return m.configured }
func (m *fakeOperatorMFA) HasEnrollment(context.Context, int64) (bool, error) {
	m.calls = append(m.calls, "HasEnrollment")
	return m.enrolled, m.enrolledErr
}
func (m *fakeOperatorMFA) VerifyTrustedDevice(_ context.Context, _ int64, cookie string) (bool, error) {
	m.calls = append(m.calls, "VerifyTrustedDevice")
	return cookie != "" && cookie == m.trustedCookie, nil
}
func (m *fakeOperatorMFA) StartChallenge(context.Context, int64, string) (string, error) {
	m.calls = append(m.calls, "StartChallenge")
	if m.challengeErr != nil {
		return "", m.challengeErr
	}
	if m.challenge == "" {
		return "challenge-token", nil
	}
	return m.challenge, nil
}
func (m *fakeOperatorMFA) TrustedDeviceDays() int { return 90 }

type fakeOperatorAudit struct {
	mu      sync.Mutex
	entries []domain.OperatorAuditEntry
	err     error
}

func (a *fakeOperatorAudit) RecordOperatorAction(_ context.Context, entry domain.OperatorAuditEntry) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.err != nil {
		return a.err
	}
	a.entries = append(a.entries, entry)
	return nil
}

func (a *fakeOperatorAudit) actions(action string) []domain.OperatorAuditEntry {
	a.mu.Lock()
	defer a.mu.Unlock()
	var result []domain.OperatorAuditEntry
	for _, entry := range a.entries {
		if entry.Action == action {
			result = append(result, entry)
		}
	}
	return result
}

type fakeCredentialCleanup struct {
	invalidated []int64
	err         error
}

func (c *fakeCredentialCleanup) InvalidateEmailChangeTokens(_ context.Context, operatorID int64) error {
	if c.err != nil {
		return c.err
	}
	c.invalidated = append(c.invalidated, operatorID)
	return nil
}

// fakeHasher hashes as "hash:<password>" so fakePasswords verifies it, and
// treats passwords shorter than eight characters as too weak.
type fakeHasher struct{}

func (fakeHasher) HashPassword(password string) (string, error) { return "hash:" + password, nil }
func (fakeHasher) ValidatePasswordStrength(password string) error {
	if len(password) < 8 {
		return errors.New("too short")
	}
	return nil
}

// --- fixture ----------------------------------------------------------------

type operatorFixture struct {
	auth        *OperatorAuthentication
	store       *fakeStore
	mfa         *fakeOperatorMFA
	audit       *fakeOperatorAudit
	credentials *fakeCredentialCleanup
	runtime     *fakeRuntime
}

func newOperatorFixture(t *testing.T) *operatorFixture {
	t.Helper()
	store := newFakeStore()
	runtime := &fakeRuntime{}
	operators := New(store, store, store, fakeTransaction{}, runtime.TenantID, func(ports.Observation) {})
	mfa := &fakeOperatorMFA{}
	audit := &fakeOperatorAudit{}
	credentials := &fakeCredentialCleanup{}
	auth, err := NewOperatorAuthentication(operators, OperatorAuthenticationDependencies{
		Passwords: fakePasswords{}, Hasher: fakeHasher{}, Codec: fakeCodec{}, MFA: mfa, Audit: audit,
		Credentials: credentials, Runtime: runtime, Rotation: fakeRotation{},
		Logger: slog.New(slog.NewTextHandler(testWriter{t}, nil)),
	})
	require.NoError(t, err)
	return &operatorFixture{auth: auth, store: store, mfa: mfa, audit: audit, credentials: credentials, runtime: runtime}
}

// seedOperator adds an active operator whose password is "secret".
func (f *operatorFixture) seedOperator(id int64) domain.Operator {
	f.store.addOperator(id, fmt.Sprintf("operator%d@example.com", id), "hash:secret", true)
	return f.store.operators[id]
}

// withProof attaches a recovery proof the fake rotation policy hashes.
func withProof(ctx context.Context, proof string) context.Context {
	padded := strings.Repeat("0", 32-len(proof)) + proof
	return context.WithValue(ctx, fakeProofKey{}, []byte(padded[:32]))
}
