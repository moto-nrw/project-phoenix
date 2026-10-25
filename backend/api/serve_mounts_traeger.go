package api

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/auditlog"
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/traegerportal"
	"github.com/moto-nrw/project-phoenix/modules/traegerview"
	traegerViewCompose "github.com/moto-nrw/project-phoenix/modules/traegerview/compose"
	traegerViewHTTP "github.com/moto-nrw/project-phoenix/modules/traegerview/http"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// TraegerPortalIdentity is what the Träger-Portal serves of Identity &
// Access: the sign-in, the account second factor, the Träger-Büro account
// management, the Träger-Büro members' addresses and the Absprung into a
// school.
type TraegerPortalIdentity interface {
	identityaccess.TraegerAccountLogin
	identityaccess.AccountMFAChallenges
	identityaccess.AccountMFAEnrollment
	identityaccess.TraegerAccountManagement
	identityaccess.TraegerRoles
	identityaccess.TraegerSchoolSessions
}

// TraegerPersons is the People Directory read the access log names people
// with.
type TraegerPersons interface {
	ListPersonsByAccount(ctx context.Context, accountIDs []int64) ([]peopledirectory.Person, error)
	ListPersonsByID(ctx context.Context, personIDs []int64) ([]peopledirectory.Person, error)
}

// TraegerStaff is the School Membership read that ties a staff ID the
// access log meets to its person.
type TraegerStaff interface {
	ListStaff(ctx context.Context, filter schoolmembership.StaffFilter) ([]schoolmembership.Staff, error)
}

// TraegerAudit is what the Träger-Portal reads from and writes to at the
// Audit Platform: the access trail of the Träger's schools and the access
// log of one school.
type TraegerAudit struct {
	Trail auditlog.AccessTrailQuery
	Log   owners.DataAccessAppender
}

// mountTraegerPortalRoutes serves the Träger-Portal ("moto träger", ADR 0084,
// #4649) at /traeger: the public sign-in and the Träger route group, which
// accepts only Träger sessions. Token refresh and logout go through the
// shared scope-preserving /auth/refresh and /auth/logout.
func mountTraegerPortalRoutes(root chi.Router, limiters authRateLimiters, identity TraegerPortalIdentity, organizations organizationtenancy.TraegerMembershipQuery, persons TraegerPersons, staff TraegerStaff, audit TraegerAudit) error {
	portal, err := NewTraegerPortalHTTP(identity, organizations, persons, staff, audit)
	if err != nil {
		return err
	}
	root.Mount("/traeger", portal.RouterWithAuthRateLimiter(limiters.authMiddleware(), ProtectedTraegerRoutes))
	return nil
}

// NewTraegerPortalHTTP binds the Träger sign-in, its second factor, the
// account management, the Träger projection and the holiday periods of
// Enrollment to the portal's route ports.
func NewTraegerPortalHTTP(identity TraegerPortalIdentity, organizations organizationtenancy.TraegerMembershipQuery, persons TraegerPersons, staff TraegerStaff, audit TraegerAudit) (*traegerportal.Resource, error) {
	if identity == nil || organizations == nil || persons == nil || staff == nil || audit.Trail == nil || audit.Log == nil {
		return nil, errors.New("traeger portal: identity, organizations, persons, staff, the access trail and the access log are required")
	}
	overview, err := traegerViewCompose.New(traegerview.Ports{
		SchoolIDs: traegerSchoolIDs{organizations: organizations}, Trail: traegerAccessTrail{trail: audit.Trail},
		Recorder: traegerAccessRecorder{log: audit.Log}, Persons: traegerPersonDirectory{persons: persons}, Staff: traegerStaffDirectory{staff: staff},
		Bureau: traegerBureauAddresses{roles: identity},
	})
	if err != nil {
		return nil, err
	}
	holidayPeriods := enrollmentAPI.NewHolidayPeriodResource(enrollmentCompose.NewHolidayPeriods(),
		common.RequiresPermission(identityaccess.TraegerRightEnrollmentManage))
	overviewRoutes := traegerViewHTTP.NewResource(overview)
	return traegerportal.NewResource(TraegerPortalAuth(identity), TraegerPortalMFA(identity), overviewRoutes, holidayPeriods,
		traegerViewHTTP.NewAccessLogResource(overview), identity, overviewRoutes, identity), nil
}

// traegerSchoolIDs binds the Träger projection's school set to Organisation &
// Tenancy and translates a deactivated, deleted or unknown Träger into the
// projection's contract (#4657).
type traegerSchoolIDs struct {
	organizations organizationtenancy.TraegerMembershipQuery
}

func (s traegerSchoolIDs) ListTraegerSchoolIDs(ctx context.Context, organizationID int64) ([]int64, error) {
	ids, err := s.organizations.ListTraegerSchoolIDs(ctx, organizationID)
	if errors.Is(err, organizationtenancy.ErrOrganizationInactive) || errors.Is(err, organizationtenancy.ErrOrganizationNotFound) {
		return nil, traegerview.ErrTraegerUnavailable
	}
	return ids, err
}

func (s traegerSchoolIDs) ListAccountTraeger(ctx context.Context, accountID int64) ([]traegerview.Traeger, error) {
	organizations, err := s.organizations.ListAccountTraegerOrganizations(ctx, accountID)
	if err != nil {
		return nil, err
	}
	result := make([]traegerview.Traeger, 0, len(organizations))
	for _, organization := range organizations {
		result = append(result, traegerview.Traeger(organization))
	}
	return result, nil
}

// traegerAccessTrail binds the Audit Platform's access trail to the Träger
// projection's port.
type traegerAccessTrail struct {
	trail auditlog.AccessTrailQuery
}

func (t traegerAccessTrail) ListAccessTrail(ctx context.Context, query traegerview.AccessTrailQuery) ([]traegerview.AccessTrailEntry, error) {
	rows, err := t.trail.ListAccessTrail(ctx, auditlog.AccessTrailFilter(query))
	if err != nil {
		return nil, err
	}
	entries := make([]traegerview.AccessTrailEntry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, traegerview.AccessTrailEntry(row))
	}
	return entries, nil
}

func (t traegerAccessTrail) ListAccessTrailActors(ctx context.Context, schools []int64, from, to time.Time) ([]traegerview.AccessTrailActor, error) {
	rows, err := t.trail.ListAccessTrailActors(ctx, schools, from, to)
	if err != nil {
		return nil, err
	}
	actors := make([]traegerview.AccessTrailActor, 0, len(rows))
	for _, row := range rows {
		actors = append(actors, traegerview.AccessTrailActor(row))
	}
	return actors, nil
}

// traegerAccessActorRole is the actor role of a Träger-Portal read in the
// access log of a school.
const traegerAccessActorRole = "traeger"

// traegerAccessRecorder appends a Träger-Portal read to the access log of one
// school in that school's tenant transaction (ADR 0084 section 7).
type traegerAccessRecorder struct {
	log owners.DataAccessAppender
}

func (r traegerAccessRecorder) RecordTraegerAccess(ctx context.Context, access traegerview.TraegerAccess) error {
	school, err := tenant.NewTenantID(access.SchoolID)
	if err != nil {
		return err
	}
	return tenant.WithinTenant(ctx, school, func(txCtx context.Context) error {
		metadata := map[string]any{"channel": "traeger_portal", "organization_id": access.OrganizationID, "view": access.View, "rows": access.Rows}
		if access.Page > 0 {
			metadata["page"] = access.Page
		}
		return r.log.Create(txCtx, &auditModels.DataAccessLog{
			ActorAccountID: access.AccountID, ActorRole: traegerAccessActorRole, ResourceType: access.ResourceType,
			RangeStart: access.RangeStart, RangeEnd: access.RangeEnd, Metadata: metadata,
		})
	})
}

// traegerPersonDirectory reads the persons of People Directory for the
// access log.
type traegerPersonDirectory struct {
	persons TraegerPersons
}

func (d traegerPersonDirectory) PersonsByAccount(ctx context.Context, accountIDs []int64) ([]traegerview.Person, error) {
	persons, err := d.persons.ListPersonsByAccount(ctx, accountIDs)
	if err != nil {
		return nil, err
	}
	latest := peopledirectory.LatestPersonsByAccountSchool(persons)
	result := make([]traegerview.Person, 0, len(latest))
	for _, person := range latest {
		result = append(result, traegerAccessPerson(person))
	}
	return result, nil
}

func (d traegerPersonDirectory) PersonsByID(ctx context.Context, personIDs []int64) ([]traegerview.Person, error) {
	persons, err := d.persons.ListPersonsByID(ctx, personIDs)
	if err != nil {
		return nil, err
	}
	result := make([]traegerview.Person, 0, len(persons))
	for _, person := range persons {
		result = append(result, traegerAccessPerson(person))
	}
	return result, nil
}

func traegerAccessPerson(person peopledirectory.Person) traegerview.Person {
	return traegerview.Person{
		PersonID: person.ID, SchoolID: person.TenantID, AccountID: person.AccountID,
		Name: peopledirectory.PersonDisplayName(person.FirstName, person.LastName),
	}
}

// traegerStaffDirectory reads the staff members of School Membership for
// the access log, former ones included: a ledger keeps naming who changed
// data after they left.
type traegerStaffDirectory struct {
	staff TraegerStaff
}

func (d traegerStaffDirectory) StaffByID(ctx context.Context, schools, staffIDs []int64) ([]traegerview.StaffMember, error) {
	return d.list(ctx, schoolmembership.StaffFilter{IDs: staffIDs, TenantIDs: schools, IncludeDeleted: true, MembershipOnly: true})
}

func (d traegerStaffDirectory) StaffOfPersons(ctx context.Context, schools, personIDs []int64) ([]traegerview.StaffMember, error) {
	return d.list(ctx, schoolmembership.StaffFilter{PersonIDs: personIDs, TenantIDs: schools, IncludeDeleted: true, MembershipOnly: true})
}

func (d traegerStaffDirectory) list(ctx context.Context, filter schoolmembership.StaffFilter) ([]traegerview.StaffMember, error) {
	staff, err := d.staff.ListStaff(ctx, filter)
	if err != nil {
		return nil, err
	}
	result := make([]traegerview.StaffMember, 0, len(staff))
	for _, member := range staff {
		result = append(result, traegerview.StaffMember{StaffID: member.ID, SchoolID: member.TenantID, PersonID: member.PersonID})
	}
	return result, nil
}

// traegerBureauAddresses names the members of the Träger-Büro by address.
// Accounts without a Träger role at the Träger stay unnamed.
type traegerBureauAddresses struct {
	roles identityaccess.TraegerRoles
}

func (b traegerBureauAddresses) BureauAddresses(ctx context.Context, organizationID int64, accountIDs []int64) ([]traegerview.AccountName, error) {
	accounts, err := b.roles.ListTraegerAccounts(ctx, organizationID, accountIDs)
	if err != nil {
		return nil, err
	}
	names := make([]traegerview.AccountName, 0, len(accounts))
	for _, account := range accounts {
		if len(account.Roles) > 0 {
			names = append(names, traegerview.AccountName{AccountID: account.AccountID, Name: account.Email})
		}
	}
	return names, nil
}

// TraegerPortalAuth binds the Träger-Portal's session port to Identity &
// Access.
func TraegerPortalAuth(sessions identityaccess.TraegerAccountLogin) traegerportal.AuthRuntime {
	if sessions == nil {
		return traegerportal.AuthRuntime{}
	}
	return traegerportal.AuthRuntime{
		Login: func(ctx context.Context, email, password, ipAddress, userAgent string, organizationID int64) (*traegerportal.LoginResult, error) {
			result, err := sessions.LoginTraeger(ctx, email, password, ipAddress, userAgent, organizationID)
			if err != nil || result == nil {
				return nil, err
			}
			return traegerPortalLoginResult(result), nil
		},
		IssueTokens: func(ctx context.Context, proof traegerportal.VerifiedChallenge, organizationID int64, ipAddress, userAgent string) (*traegerportal.LoginResult, error) {
			result, err := sessions.IssueTraegerTokensForVerifiedMFA(ctx, identityaccess.VerifiedMFAChallenge{
				AccountID: proof.AccountID, Scope: identityaccess.MFAChallengeScopeOrg, MFAResetGeneration: proof.MFAResetGeneration,
			}, organizationID, ipAddress, userAgent)
			if err != nil || result == nil {
				return nil, err
			}
			return traegerPortalLoginResult(result), nil
		},
		InvalidCredentials: identityInvalidCredentials,
		AccountInactive:    identityIs(identityaccess.ErrAccountInactive),
		NoMembership:       identityIs(identityaccess.ErrNoTraegerMembership),
		MFABlocked:         identityMFABlocked,
		MFAUnavailable:     identityIs(identityaccess.ErrMFAStatusUnavailable),
		InvalidSession:     identityIs(identityaccess.ErrInvalidToken),
	}
}

func traegerPortalLoginResult(result *identityaccess.TraegerLoginResult) *traegerportal.LoginResult {
	organizations := make([]traegerportal.Organization, 0, len(result.Organizations))
	for _, organization := range result.Organizations {
		organizations = append(organizations, traegerportal.Organization(organization))
	}
	return &traegerportal.LoginResult{
		Status: traegerportal.LoginStatus(result.Status), AccessToken: result.AccessToken,
		RefreshToken: result.RefreshToken, ChallengeToken: result.ChallengeToken, MaskedEmail: result.MaskedEmail,
		OrganizationID: result.OrganizationID, Organizations: organizations,
	}
}

// TraegerPortalMFA binds the Träger second factor. The org challenge scope is
// applied here, so a challenge of another portal is refused before any code
// is compared. A Träger challenge names no school.
func TraegerPortalMFA(mfa interface {
	identityaccess.AccountMFAChallenges
	identityaccess.AccountMFAEnrollment
}) traegerportal.MFARuntime {
	if mfa == nil {
		return traegerportal.MFARuntime{}
	}
	verified := func(proof identityaccess.VerifiedMFAChallenge) traegerportal.VerifiedChallenge {
		return traegerportal.VerifiedChallenge{AccountID: proof.AccountID, MFAResetGeneration: proof.MFAResetGeneration}
	}
	return traegerportal.MFARuntime{
		VerifyChallenge: func(ctx context.Context, challengeToken, code string) (traegerportal.VerifiedChallenge, error) {
			proof, err := mfa.VerifyMFAChallengeForScope(ctx, challengeToken, code, identityaccess.MFAChallengeScopeOrg)
			return verified(proof), err
		},
		VerifyChallengeForOwner: func(ctx context.Context, challengeToken, code string, accountID int64) (traegerportal.VerifiedChallenge, error) {
			proof, err := mfa.VerifyMFAChallengeForOwner(ctx, challengeToken, code, identityaccess.MFAChallengeScopeOrg, accountID, 0)
			return verified(proof), err
		},
		ResendChallenge: func(ctx context.Context, challengeToken string, ip net.IP) (string, error) {
			return mfa.ResendMFAChallengeForScope(ctx, challengeToken, ip, identityaccess.MFAChallengeScopeOrg)
		},
		StartChallenge: func(ctx context.Context, accountID int64, ip net.IP) (string, error) {
			return mfa.StartMFAChallenge(ctx, accountID, 0, identityaccess.MFAChallengeScopeOrg, ip)
		},
		Enroll:            mfa.EnrollMFA,
		ChallengeUnusable: identityChallengeUnusable,
		Blocked:           identityMFABlocked,
		Unavailable:       identityIs(identityaccess.ErrMFAStatusUnavailable),
		AlreadyEnrolled:   identityIs(identityaccess.ErrMFAAlreadyEnrolled),
	}
}
