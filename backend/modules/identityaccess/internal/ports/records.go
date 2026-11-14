package ports

import (
	"context"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
)

// The record ports of the operator and account sessions, the operator second
// factor and the operator links. Only the module's
// flows read and write these rows; the public contract carries the flows.
// OperatorSession is one persisted, revocable operator refresh session.
// Rotation records the hand-off on the predecessor (RotatedAt,
// ReplacementTokenHash, RecoveryProofHash) so a replay can be detected and a
// lost response recovered within the rotation grace.
//
// Token is the raw value the refresh JWT carries. Only the minting or
// presenting request knows it; the store keeps TokenHash, its hex SHA-256,
// and ReplacementTokenHash likewise names the successor by hash (#4902).
// Listings and revocations return Token empty.
type OperatorSession struct {
	ID                   int64
	OperatorID           int64
	Token                string
	TokenHash            string
	Expiry               time.Time
	FamilyID             string
	Generation           int
	RotatedAt            *time.Time
	ReplacementTokenHash *string
	RecoveryProofHash    []byte
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// OperatorQuery resolves operators. Every lookup returns ErrOperatorNotFound
// for an unknown operator; e-mail matching is exact on the stored,
// lower-cased address.
type OperatorQuery interface {
	FindOperator(ctx context.Context, id int64) (identityaccess.Operator, error)
	// FindOperatorForUpdate locks the operator row for the caller's
	// transaction so concurrent credential changes serialize.
	FindOperatorForUpdate(ctx context.Context, id int64) (identityaccess.Operator, error)
	FindOperatorByEmail(ctx context.Context, email string) (identityaccess.Operator, error)
	ListOperators(ctx context.Context) ([]identityaccess.Operator, error)
}

// OperatorCommand changes operator identity rows. Create and Update validate
// and normalize the address and display name. Update locks the row, lets
// change edit the row as committed, writes every mutable column and returns
// ErrOperatorNotFound when the row is gone; that and an error from change are
// returned as is, and nothing is written.
type OperatorCommand interface {
	CreateOperator(ctx context.Context, operator identityaccess.Operator) (identityaccess.Operator, error)
	UpdateOperator(ctx context.Context, id int64, change func(*identityaccess.Operator) error) (identityaccess.Operator, error)
	DeleteOperator(ctx context.Context, id int64) error
	RecordOperatorLogin(ctx context.Context, id int64) error
	// IncrementOperatorMFAAttempts bumps the failed-attempt counter atomically
	// and applies the lockout window once threshold is reached.
	IncrementOperatorMFAAttempts(ctx context.Context, id int64, threshold int, lockout time.Duration) (identityaccess.OperatorMFAAttempts, error)
	ResetOperatorMFAAttempts(ctx context.Context, id int64) error
}

// OperatorSessionQuery resolves refresh sessions. Both lookups return
// ErrOperatorSessionNotFound when nothing matches.
type OperatorSessionQuery interface {
	// FindOperatorSessionForUpdate locks the session row for the caller's
	// transaction; refresh serializes on it before rotating.
	FindOperatorSessionForUpdate(ctx context.Context, token string) (OperatorSession, error)
	LatestOperatorSessionInFamily(ctx context.Context, familyID string) (OperatorSession, error)
}

// OperatorSessionCommand mints, rotates and revokes refresh sessions. The
// revocations return the deleted sessions so the caller can record its audit
// evidence in the same transaction.
type OperatorSessionCommand interface {
	CreateOperatorSession(ctx context.Context, session OperatorSession) (OperatorSession, error)
	// MarkOperatorSessionRotated records the hand-off exactly once and returns
	// ErrOperatorSessionRotated when the session was rotated before.
	MarkOperatorSessionRotated(ctx context.Context, id int64, replacementToken string, recoveryProofHash []byte, rotatedAt time.Time) error
	// DeleteExpiredRotatedOperatorSessions drops rotated predecessors of the
	// family whose refresh JWTs expired before now; earlier ones stay as
	// replay evidence.
	DeleteExpiredRotatedOperatorSessions(ctx context.Context, familyID string, now time.Time) error
	DeleteOperatorSession(ctx context.Context, id int64) error
	RevokeOperatorSessions(ctx context.Context, operatorID int64) ([]OperatorSession, error)
	RevokeOperatorSessionFamily(ctx context.Context, familyID string) ([]OperatorSession, error)
	DeleteExpiredOperatorSessions(ctx context.Context, now time.Time) (int, error)
}

// OperatorAccess is the capability operator login, refresh, MFA lockout and
// session revocation consume (#2720).
type OperatorAccess interface {
	OperatorQuery
	OperatorCommand
	OperatorSessionQuery
	OperatorSessionCommand
}

// AccountSession is one persisted, revocable refresh session of a platform
// account at one school. A family groups the generations one login produced
// through rotation; the hand-off (RotatedAt, ReplacementTokenHash,
// RecoveryProofHash) lets a lost rotation response be recovered within the
// rotation grace and a replay be detected. FamilyExpiryCap bounds every
// successor of a family a tenant switch retired.
//
// Token is the raw value the refresh JWT carries. Only the minting or
// presenting request knows it; the store keeps TokenHash, its hex SHA-256,
// and ReplacementTokenHash likewise names the successor by hash (#4902).
// Listings and revocations return Token empty.
type AccountSession struct {
	ID       int64
	TenantID int64
	// OrganizationID is the Träger of a Träger-Portal session (portal scope
	// org); such a session has no school (ADR 0084).
	OrganizationID int64
	AccountID      int64
	Token          string
	TokenHash      string
	Expiry         time.Time
	Mobile         bool
	Identifier     *string
	PortalScope    string
	MFAVerified    bool
	// MFAByDevice: only a remembered device proved the second factor
	// (#6955); a school that switched remembered devices off asks for the
	// code at an Absprung or a switch.
	MFAByDevice bool
	FamilyID    string
	// SwitchedFromFamilyID names the origin of the session's switch lineage:
	// the login family a school switch descends from, or the Träger family
	// of an Absprung. It outlives the origin's rows (#5422).
	SwitchedFromFamilyID string
	FamilyExpiryCap      *time.Time
	// LineageDeadline is the transition deadline of a lineage descending
	// from a family issued before #5422; switches and rotations keep it.
	LineageDeadline      *time.Time
	Generation           int
	RotatedAt            *time.Time
	ReplacementTokenHash *string
	RecoveryProofHash    []byte
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// AccountSessionLiveness narrows a listing by expiry.
type AccountSessionLiveness int

// AccountSessionFilter narrows ListAccountSessions. Zero values do not
// filter; the caller's tenant scope always applies.
type AccountSessionFilter struct {
	AccountID int64
	FamilyID  string
	Mobile    *bool
	Liveness  AccountSessionLiveness
}

// AccountSessionQuery resolves account refresh sessions. Reads that name no
// tenant apply the caller's tenant scope: a tenantless caller (login,
// refresh and logout run before a tenant is known) sees every school's rows.
// Lookups return ErrAccountSessionNotFound when nothing matches.
type AccountSessionQuery interface {
	FindAccountSession(ctx context.Context, token string) (AccountSession, error)
	// FindAccountSessionForUpdate locks the session row for the caller's
	// transaction; refresh serializes on it before rotating.
	FindAccountSessionForUpdate(ctx context.Context, token string) (AccountSession, error)
	LatestAccountSessionInFamily(ctx context.Context, familyID string) (AccountSession, error)
	ListAccountSessions(ctx context.Context, filter AccountSessionFilter) ([]AccountSession, error)
	// CountExpiredAccountSessions counts across every school; the cleanup
	// preview reports the whole sweep.
	CountExpiredAccountSessions(ctx context.Context) (int, error)
	// ListInactiveAccountIDsWithLiveSessions names deactivated accounts that
	// still hold an un-rotated, unexpired session, so a failed account-wide
	// wipe can be recovered.
	ListInactiveAccountIDsWithLiveSessions(ctx context.Context) ([]int64, error)
	HasLiveAccountSessionsCreatedAfter(ctx context.Context, accountID int64, since time.Time) (bool, error)
}

// AccountSessionCommand mints, rotates, caps and revokes account refresh
// sessions. Every write joins the caller's transaction; the revocations
// return the deleted sessions so the caller can record its audit evidence in
// the same transaction.
type AccountSessionCommand interface {
	// CreateAccountSession validates the session and pins it to the caller's
	// tenant when the session names none.
	CreateAccountSession(ctx context.Context, session AccountSession) (AccountSession, error)
	// MarkAccountSessionRotated records the hand-off exactly once and returns
	// ErrAccountSessionRotated when the session was rotated before.
	MarkAccountSessionRotated(ctx context.Context, id int64, replacementToken string, recoveryProofHash []byte, rotatedAt time.Time) error
	// DeleteExpiredRotatedAccountSessions drops rotated predecessors of the
	// account whose refresh JWTs expired before now; earlier ones stay as
	// replay evidence.
	DeleteExpiredRotatedAccountSessions(ctx context.Context, accountID int64, now time.Time) error
	// RetireAccountSessionFamily caps the expiry of the family's live sessions
	// so a tenant switch hands the replaced session a bounded grace.
	RetireAccountSessionFamily(ctx context.Context, accountID int64, familyID string, expiry time.Time) error
	// EnforceAccountSessionCap keeps at most keep live sessions in the portal
	// group of portalScope and returns the evicted sessions. The session with
	// the ID minted always stays; zero protects none.
	EnforceAccountSessionCap(ctx context.Context, accountID int64, portalScope string, keep int, minted int64) ([]AccountSession, error)
	DeleteAccountSession(ctx context.Context, id int64) error
	RevokeAccountSessionFamily(ctx context.Context, familyID string) ([]AccountSession, error)
	// RevokeAccountSessionsInTenant deletes the account's sessions at the
	// caller's school only and returns ErrTenantRequired without one.
	RevokeAccountSessionsInTenant(ctx context.Context, accountID int64) ([]AccountSession, error)
	// RevokeAllAccountSessions deletes the account's sessions at every school.
	// keepFamilyID, when set, names the one family that stays.
	RevokeAllAccountSessions(ctx context.Context, accountID int64, keepFamilyID string) ([]AccountSession, error)
	// RevokeAccountSessionsCreatedAtOrBefore deletes the sessions that already
	// existed at cutoff, including their later refresh successors, and keeps
	// logins that only started afterwards.
	RevokeAccountSessionsCreatedAtOrBefore(ctx context.Context, accountID int64, cutoff time.Time) ([]AccountSession, error)
	RevokeTenantAccountSessions(ctx context.Context, tenantID int64) ([]AccountSession, error)
	DeleteExpiredAccountSessions(ctx context.Context) (int, error)
}

// AccountSessionAccess is the capability tenant, parent and school login,
// refresh, switching, logout, session validation and revocation consume
// (#2720).
type AccountSessionAccess interface {
	AccountSessionQuery
	AccountSessionCommand
}

const (
	// AccountSessionsAny lists sessions regardless of expiry.
	AccountSessionsAny AccountSessionLiveness = iota
	// AccountSessionsLive lists sessions whose expiry is in the future.
	AccountSessionsLive
	// AccountSessionsExpired lists sessions whose expiry has passed.
	AccountSessionsExpired
)

// OperatorMFAQuery resolves operator MFA records. "Active" means unconsumed
// (codes) or unrevoked (devices) and not yet expired.
type OperatorMFAQuery interface {
	FindOperatorMFACredential(ctx context.Context, operatorID int64) (identityaccess.OperatorMFACredential, error)
	// FindActiveOperatorMFAChallenge returns the active code with the latest
	// expiry.
	FindActiveOperatorMFAChallenge(ctx context.Context, operatorID int64) (identityaccess.OperatorMFAChallenge, error)
	// CountOperatorMFAChallengesSince counts every code created at or after
	// since, redeemed or not; the send rate limit reads it.
	CountOperatorMFAChallengesSince(ctx context.Context, operatorID int64, since time.Time) (int, error)
	FindActiveOperatorTrustedDevice(ctx context.Context, operatorID int64, tokenHash string) (identityaccess.OperatorTrustedDevice, error)
	// ListActiveOperatorTrustedDevices orders by last use, most recent first,
	// then by creation.
	ListActiveOperatorTrustedDevices(ctx context.Context, operatorID int64) ([]identityaccess.OperatorTrustedDevice, error)
}

// OperatorMFACommand changes operator MFA records. The creates validate the
// record and return it with its identity and timestamps.
type OperatorMFACommand interface {
	CreateOperatorMFACredential(ctx context.Context, credential identityaccess.OperatorMFACredential) (identityaccess.OperatorMFACredential, error)
	TouchOperatorMFACredential(ctx context.Context, id int64, usedAt time.Time) error
	DeleteOperatorMFACredentials(ctx context.Context, operatorID int64) error
	CreateOperatorMFAChallenge(ctx context.Context, challenge identityaccess.OperatorMFAChallenge) (identityaccess.OperatorMFAChallenge, error)
	// ActivateOperatorMFAChallenge makes a delivered code redeemable.
	ActivateOperatorMFAChallenge(ctx context.Context, id int64) error
	// ConsumeOperatorMFAChallenge redeems a code exactly once.
	ConsumeOperatorMFAChallenge(ctx context.Context, id int64, consumedAt time.Time) error
	CreateOperatorTrustedDevice(ctx context.Context, device identityaccess.OperatorTrustedDevice) (identityaccess.OperatorTrustedDevice, error)
	TouchOperatorTrustedDevice(ctx context.Context, id int64, usedAt time.Time) error
	RevokeOperatorTrustedDevice(ctx context.Context, id int64, revokedAt time.Time) error
	RevokeOperatorTrustedDevices(ctx context.Context, operatorID int64, revokedAt time.Time) error
}

// OperatorMFARecords is the capability the operator MFA flows consume.
type OperatorMFARecords interface {
	OperatorMFAQuery
	OperatorMFACommand
}

// OperatorTokenQuery resolves operator invitation and e-mail change links.
type OperatorTokenQuery interface {
	// FindOperatorInvitation returns the invitation in any state.
	FindOperatorInvitation(ctx context.Context, id int64) (identityaccess.OperatorInvitation, error)
	FindRedeemableOperatorInvitation(ctx context.Context, token string) (identityaccess.OperatorInvitation, error)
	// ListRedeemableOperatorInvitations orders by creation, newest first.
	ListRedeemableOperatorInvitations(ctx context.Context) ([]identityaccess.OperatorInvitation, error)
	// CountOperatorInvitationsCreatedAfter counts every invitation the
	// operator created after since, spent or not; the invite rate limit
	// reads it.
	CountOperatorInvitationsCreatedAfter(ctx context.Context, createdBy int64, since time.Time) (int, error)
	// CountOperatorEmailChangesCreatedAfter counts every link created for
	// the operator after since, spent or not; the initiation rate limit
	// reads it.
	CountOperatorEmailChangesCreatedAfter(ctx context.Context, operatorID int64, since time.Time) (int, error)
}

// OperatorTokenCommand changes operator invitation and e-mail change links.
// The creates validate the link and return it with its identity and
// timestamps.
type OperatorTokenCommand interface {
	OperatorInvitationCommand
	OperatorEmailChangeCommand
}

// OperatorInvitationCommand issues, redeems and retires operator invitations.
type OperatorInvitationCommand interface {
	CreateOperatorInvitation(ctx context.Context, invitation identityaccess.OperatorInvitation) (identityaccess.OperatorInvitation, error)
	// RedeemOperatorInvitation spends a redeemable invitation exactly once.
	RedeemOperatorInvitation(ctx context.Context, token string) (identityaccess.OperatorInvitation, error)
	// RevokeOperatorInvitation spends an unused invitation.
	RevokeOperatorInvitation(ctx context.Context, id int64) error
	// RevokeOperatorInvitationsForEmail spends every unused invitation for
	// the address and returns how many it spent.
	RevokeOperatorInvitationsForEmail(ctx context.Context, email string) (int, error)
	// ExtendOperatorInvitation moves the expiry of a redeemable invitation
	// to the future expiresAt and replaces its link token. It returns the new
	// raw token, the only copy: the store keeps its hash (#4902).
	ExtendOperatorInvitation(ctx context.Context, id int64, expiresAt time.Time) (string, error)
	RecordOperatorInvitationDelivery(ctx context.Context, id int64, delivery identityaccess.TokenDelivery) error
	DeleteExpiredOperatorInvitations(ctx context.Context) (int, error)
}

// OperatorEmailChangeCommand issues and redeems operator e-mail changes.
type OperatorEmailChangeCommand interface {
	CreateOperatorEmailChange(ctx context.Context, change identityaccess.OperatorEmailChange) (identityaccess.OperatorEmailChange, error)
	// RedeemOperatorEmailChange spends a redeemable link exactly once.
	RedeemOperatorEmailChange(ctx context.Context, token string) (identityaccess.OperatorEmailChange, error)
	// RevokeOperatorEmailChanges spends every unused link of the operator.
	RevokeOperatorEmailChanges(ctx context.Context, operatorID int64) error
	RecordOperatorEmailChangeDelivery(ctx context.Context, id int64, delivery identityaccess.TokenDelivery) error
	// RevokeExpiredOperatorEmailChanges spends expired, unused links so the
	// operator can request a new one, and returns how many it spent.
	RevokeExpiredOperatorEmailChanges(ctx context.Context) (int, error)
	// DeleteStaleOperatorEmailChanges deletes expired or spent links older
	// than the initiation rate-limit window.
	DeleteStaleOperatorEmailChanges(ctx context.Context) (int, error)
}

// OperatorTokens is the capability the operator invitation and e-mail
// change flows consume.
type OperatorTokens interface {
	OperatorTokenQuery
	OperatorTokenCommand
}

// Records are all record ports the composed owner serves; the module's
// record-contract tests reach the stores through them.
type Records interface {
	OperatorAccess
	AccountSessionAccess
	OperatorMFARecords
	OperatorTokens
}
