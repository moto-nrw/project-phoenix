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
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/portalauth"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	organizationTenancyCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	organizationTenancyTraeger "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/inbound/traeger"
	parentBillingCompose "github.com/moto-nrw/project-phoenix/modules/parentbilling/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	schoolMembershipCompose "github.com/moto-nrw/project-phoenix/modules/schoolmembership/compose"
	settingsHTTP "github.com/moto-nrw/project-phoenix/modules/settings/http"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	studentPresenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/traegerportal"
	"github.com/moto-nrw/project-phoenix/modules/traegerview"
	traegerViewCompose "github.com/moto-nrw/project-phoenix/modules/traegerview/compose"
	traegerViewHTTP "github.com/moto-nrw/project-phoenix/modules/traegerview/http"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// TraegerPortalIdentity is what the Träger-Portal serves of Identity &
// Access: the sign-in, the account second factor, the password reset, the
// Träger-Büro account management, the Träger-Büro members' addresses and the
// Absprung into a school.
type TraegerPortalIdentity interface {
	identityaccess.TraegerAccountLogin
	identityaccess.AccountMFAChallenges
	identityaccess.AccountMFAEnrollment
	identityaccess.TraegerAccountManagement
	identityaccess.TraegerRoles
	identityaccess.TraegerSchoolSessions
	identityaccess.PasswordResets
	// ListAccountEmails names the account addresses of a child's guardians
	// for the notice of a decided contract end (#4682).
	ListAccountEmails(ctx context.Context, accountIDs []int64) (map[int64]string, error)
}

// TraegerPersons is the People Directory read the access log names people
// with.
type TraegerPersons interface {
	ListPersonsByAccount(ctx context.Context, accountIDs []int64) ([]peopledirectory.Person, error)
	ListPersonsByID(ctx context.Context, personIDs []int64) ([]peopledirectory.Person, error)
	// The guardian relationships and profiles of a contract's accounts,
	// for the notice of a decided contract end (#4682).
	ListGuardianLinksByAccounts(ctx context.Context, accountIDs []int64) ([]peopledirectory.GuardianLink, error)
	ListGuardiansByAccount(ctx context.Context, accountIDs []int64) ([]peopledirectory.Guardian, error)
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
func mountTraegerPortalRoutes(root chi.Router, limiters authRateLimiters, identity TraegerPortalIdentity, organizations organizationtenancy.TraegerMembershipQuery, persons TraegerPersons, staff TraegerStaff, audit TraegerAudit, settings TraegerPortalSettings, healthData enrollment.HolidayHealthCipher, holidayCalendar enrollment.HolidaySchoolCalendar, holidayMail enrollment.TraegerMailOutbox, holidayTraeger enrollment.HolidayTraegerDirectory) error {
	portal, err := NewTraegerPortalHTTP(identity, organizations, persons, staff, audit, settings, healthData, holidayCalendar, holidayMail, holidayTraeger)
	if err != nil {
		return err
	}
	root.Mount("/traeger", portal.RouterWithAuthRateLimiter(limiters.authMiddleware(), ProtectedTraegerRoutes))
	return nil
}

// NewTraegerPortalHTTP binds the Träger sign-in, its second factor, the
// account management, the Träger projection, the holiday periods of
// Enrollment, the Träger settings and the master data of Organisation &
// Tenancy to the portal's route ports. healthData opens the health
// information of a holiday registration behind the right
// "Gesundheitsangaben sehen" (#4722). holidayCalendar names the host
// schools' days without care, which no holiday week may book (#4702).
// holidayMail tells the parents the places the Büro decided on by criteria
// (#4700), in the Träger transaction of the decision; holidayTraeger names
// the Träger and the host schools in that mail, like the mail of a booking
// that moved up (#6437).
func NewTraegerPortalHTTP(identity TraegerPortalIdentity, organizations organizationtenancy.TraegerMembershipQuery, persons TraegerPersons, staff TraegerStaff, audit TraegerAudit, settings TraegerPortalSettings, healthData enrollment.HolidayHealthCipher, holidayCalendar enrollment.HolidaySchoolCalendar, holidayMail enrollment.TraegerMailOutbox, holidayTraeger enrollment.HolidayTraegerDirectory) (*traegerportal.Resource, error) {
	if identity == nil || organizations == nil || persons == nil || staff == nil || audit.Trail == nil || audit.Log == nil || settings == nil || holidayCalendar == nil || holidayMail == nil || holidayTraeger == nil {
		return nil, errors.New("traeger portal: identity, organizations, persons, staff, the access trail, the access log, the settings, the host schools' calendar, the holiday mail and the Träger names are required")
	}
	overview, err := traegerViewCompose.New(traegerview.Ports{
		SchoolIDs: traegerSchoolIDs{organizations: organizations}, Trail: traegerAccessTrail{trail: audit.Trail},
		Recorder: traegerAccessRecorder{log: audit.Log}, Holiday: traegerHolidayTrail{trail: enrollmentCompose.NewHolidayAccessTrail()}, Persons: traegerPersonDirectory{persons: persons}, Staff: traegerStaffDirectory{staff: staff},
		Bureau:  traegerBureauAddresses{roles: identity},
		Rosters: schoolMembershipCompose.NewSchoolRosters(), Present: studentPresenceCompose.NewPresentStudents(),
		Sick: carePlanCompose.NewSickStudents(), OnSite: workforceCompose.NewOnSiteStaff(),
		KeyDates:    traegerKeyDateCounts{organizations: organizations},
		Enrollments: enrollmentCompose.NewOpenEnrollments(), Alerts: traegerAlertSettings{settings: settings},
		Occupancy: traegerOccupancyPorts(), Known: traegerKnownChildrenPorts(),
	})
	if err != nil {
		return nil, err
	}
	review, places, allocation, records := enrollmentCompose.NewHolidayRegistrationReview(healthData, holidayMail, holidayTraeger, holidayCalendar)
	_, allocationBilling := parentBillingCompose.NewHolidayContributions(parentBillingHolidayRegistrations{review: review}, parentBillingSettings{settings: settings})
	decide := newHolidayAllocation(allocationBilling, review, places, allocation)
	_, wishes := enrollmentCompose.NewHolidayWishes(healthData)
	holidayPeriods := enrollmentAPI.NewHolidayPeriodResource(enrollmentCompose.NewHolidayPeriods(nil, holidayCalendar), review, records, wishes, places, allocation, decide, holidaySchoolNames{overview: overview},
		holidayKnownChildren{overview: overview}, enrollmentCompose.NewHolidayKnownChildTrail(),
		common.RequiresPermission(identityaccess.TraegerRightEnrollmentManage), common.RequiresPermission(identityaccess.TraegerRightHealthRead))
	traegerSettings := settingsHTTP.NewTraegerSettingsResource(settings, identityaccess.TraegerRightOverviewRead,
		identityaccess.TraegerRightSettingsManage, identityaccess.TraegerRightStaffManage, identityaccess.TraegerRightBillingManage)
	masterData := organizationTenancyTraeger.NewMasterDataResource(organizationTenancyCompose.NewTraegerMasterData(), organizationTenancyCompose.NewTraegerDocumentTemplates(),
		common.RequiresPermission(identityaccess.TraegerRightOverviewRead), common.RequiresPermission(identityaccess.TraegerRightSettingsManage))
	overviewRoutes := traegerViewHTTP.NewResource(overview)
	return traegerportal.NewResource(TraegerPortalAuth(identity), TraegerPortalMFA(identity),
		portalauth.PasswordResetRoutes(identity, identityaccess.PasswordResetScopeTraeger), overviewRoutes, holidayPeriods,
		traegerViewHTTP.NewAccessLogResource(overview), traegerSettings, masterData, identity, overviewRoutes, identity), nil
}

// TraegerPortalSettings is the Träger level of the settings system as the
// portal serves it: the settings routes and the thresholds of the dashboard
// hints (#4675).
type TraegerPortalSettings interface {
	TraegerBrandingSettings
	settingsHTTP.TraegerOperations
}

// traegerAlertSettings binds the thresholds of the dashboard hints (#4675) to
// the Träger values of the settings system.
type traegerAlertSettings struct {
	settings TraegerPortalSettings
}

func (s traegerAlertSettings) AlertThresholds(ctx context.Context, organizationID int64) (traegerview.AlertThresholds, error) {
	values, err := s.settings.TraegerInts(ctx, organizationID, []string{
		configModels.KeyTraegerAlertChildrenPerStaff, configModels.KeyTraegerAlertSickPercent, configModels.KeyTraegerAlertOpenEnrollments,
	})
	if err != nil {
		return traegerview.AlertThresholds{}, err
	}
	return traegerview.AlertThresholds{
		ChildrenPerStaff: values[configModels.KeyTraegerAlertChildrenPerStaff],
		SickPercent:      values[configModels.KeyTraegerAlertSickPercent],
		OpenEnrollments:  values[configModels.KeyTraegerAlertOpenEnrollments],
	}, nil
}

// holidaySchoolNames binds the school names of the holiday registration
// export to the Träger projection's school list (#4663, ADR 0114).
type holidaySchoolNames struct {
	overview traegerview.Overview
}

func (n holidaySchoolNames) HolidaySchoolNames(ctx context.Context, organizationID int64) (map[int64]string, error) {
	schools, err := n.overview.Schools(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	names := make(map[int64]string, len(schools))
	for _, school := range schools {
		names[school.ID] = school.Name
	}
	return names, nil
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

// traegerKeyDateCounts binds the Träger projection's Stichtagszahlen to the
// rows of the operator billing report at Organisation & Tenancy (#4673): it
// translates an unavailable Träger and the calendar days into the
// projection's contract.
type traegerKeyDateCounts struct {
	organizations organizationtenancy.TraegerMembershipQuery
}

func (c traegerKeyDateCounts) ListTraegerKeyDateCounts(ctx context.Context, organizationID int64) ([]traegerview.SchoolKeyDateCount, error) {
	rows, err := c.organizations.ListTraegerBillingKeyDateCounts(ctx, organizationID)
	if errors.Is(err, organizationtenancy.ErrOrganizationInactive) || errors.Is(err, organizationtenancy.ErrOrganizationNotFound) {
		return nil, traegerview.ErrTraegerUnavailable
	}
	if err != nil {
		return nil, err
	}
	result := make([]traegerview.SchoolKeyDateCount, 0, len(rows))
	for _, row := range rows {
		period, err := calendar.ParseDate(row.Period)
		if err != nil {
			return nil, err
		}
		keyDate, err := calendar.ParseDate(row.KeyDate)
		if err != nil {
			return nil, err
		}
		result = append(result, traegerview.SchoolKeyDateCount{SchoolID: row.SchoolID, KeyDateCount: traegerview.KeyDateCount{
			Period: period, KeyDate: keyDate, ActiveStudents: row.ActiveStudents, ActiveTerminals: row.ActiveTerminals, RecordedAt: row.RecordedAt,
		}})
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

// traegerHolidayTrail binds Enrollment's trail of the holiday registrations
// to the Träger projection's port (#5326).
type traegerHolidayTrail struct {
	trail enrollment.HolidayAccessTrail
}

func (t traegerHolidayTrail) ListHolidayTrail(ctx context.Context, organizationID int64, query traegerview.HolidayTrailQuery) ([]traegerview.AccessTrailEntry, error) {
	rows, err := t.trail.ListHolidayAccess(ctx, organizationID, enrollment.HolidayAccessFilter(query))
	if err != nil {
		return nil, err
	}
	entries := make([]traegerview.AccessTrailEntry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, traegerview.AccessTrailEntry{ID: row.ID, OccurredAt: row.OccurredAt, AccountID: row.AccountID, Kind: row.Kind, Data: row.Data})
	}
	return entries, nil
}

func (t traegerHolidayTrail) ListHolidayTrailActors(ctx context.Context, organizationID int64, from, to time.Time) ([]traegerview.AccessTrailActor, error) {
	accounts, err := t.trail.ListHolidayAccessActors(ctx, organizationID, from, to)
	if err != nil {
		return nil, err
	}
	actors := make([]traegerview.AccessTrailActor, 0, len(accounts))
	for _, account := range accounts {
		actors = append(actors, traegerview.AccessTrailActor{AccountID: account})
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
		Login: sessions.LoginTraeger,
		IssueTokens: func(ctx context.Context, proof traegerportal.VerifiedChallenge, organizationID int64, ipAddress, userAgent string) (*identityaccess.TraegerLoginResult, error) {
			return sessions.IssueTraegerTokensForVerifiedMFA(ctx, identityaccess.VerifiedMFAChallenge{
				AccountID: proof.AccountID, Scope: identityaccess.MFAChallengeScopeOrg, MFAResetGeneration: proof.MFAResetGeneration,
			}, organizationID, ipAddress, userAgent)
		},
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
		StartChallenge: func(ctx context.Context, accountID int64, ip net.IP, credentialGeneration int64) (string, error) {
			return mfa.StartMFAChallenge(ctx, accountID, 0, identityaccess.MFAChallengeScopeOrg, ip, credentialGeneration)
		},
		Enroll: func(ctx context.Context, proof traegerportal.VerifiedChallenge) error {
			return mfa.EnrollMFA(ctx, identityaccess.VerifiedMFAChallenge{AccountID: proof.AccountID, Scope: identityaccess.MFAChallengeScopeOrg, MFAResetGeneration: proof.MFAResetGeneration})
		},
	}
}
