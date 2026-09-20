package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
	"github.com/stretchr/testify/require"
)

// In-memory doubles of the ports the account lifecycle flows (#3225)
// consume. They extend the account-authentication doubles with the PIN
// columns, the parent accounts, the staff and guardian directories, the
// preview audit and the retained invitation storage and delivery.

var errLifecycleBoom = errors.New("boom")

const lifecycleTenant int64 = 77

// --- identity store --------------------------------------------------------

type lifecycleStore struct {
	*fakeStore

	pins        map[int64]domain.PINAccount
	pinAttempts map[int64]int
	pinResets   map[int64]int
	lastLockout struct {
		threshold   int
		lockedUntil time.Time
	}
	cards                     map[string]bool
	grants                    map[mappingKey][]domain.PermissionGrant
	parents                   map[int64]domain.ParentAccount
	nextID                    int64
	granted                   []int64
	pinError                  error
	findAccountsByEmailsCalls int
}

func newLifecycleStore() *lifecycleStore {
	return &lifecycleStore{
		fakeStore: newFakeStore(),
		pins:      map[int64]domain.PINAccount{}, pinAttempts: map[int64]int{}, pinResets: map[int64]int{},
		cards: map[string]bool{}, grants: map[mappingKey][]domain.PermissionGrant{}, parents: map[int64]domain.ParentAccount{},
	}
}

func (s *lifecycleStore) FindPINAccount(_ context.Context, id int64, _ bool) (domain.PINAccount, bool, domain.OperationStats, error) {
	if s.pinError != nil {
		return domain.PINAccount{}, false, stats(), s.pinError
	}
	account, ok := s.pins[id]
	return account, ok, stats(), nil
}

func (s *lifecycleStore) IncrementPINAttempts(_ context.Context, id int64, threshold int, lockedUntil time.Time) (domain.OperationStats, error) {
	s.pinAttempts[id]++
	s.lastLockout.threshold = threshold
	s.lastLockout.lockedUntil = lockedUntil
	if s.pinAttempts[id] >= threshold {
		account := s.pins[id]
		account.PINLockedUntil = &lockedUntil
		s.pins[id] = account
	}
	return stats(), nil
}

func (s *lifecycleStore) ResetPINAttempts(_ context.Context, id int64) (domain.OperationStats, error) {
	s.pinResets[id]++
	s.pinAttempts[id] = 0
	account := s.pins[id]
	account.PINLockedUntil = nil
	s.pins[id] = account
	return stats(), nil
}

func (s *lifecycleStore) UpdatePINHash(_ context.Context, id int64, hash string) (domain.OperationStats, error) {
	account := s.pins[id]
	account.PINHash = hash
	s.pins[id] = account
	return stats(), nil
}

func (s *lifecycleStore) ListTenantAccounts(_ context.Context, tenantID int64) ([]domain.TenantAccount, domain.OperationStats, error) {
	var result []domain.TenantAccount
	for _, mapping := range s.mappings {
		if mapping.tenantID != tenantID {
			continue
		}
		account := s.accounts[mapping.accountID]
		status := "active"
		if s.inactive[mapping] {
			status = "inactive"
		}
		names := domain.RoleNames(s.roles[mapping])
		sort.Strings(names)
		result = append(result, domain.TenantAccount{AccountID: mapping.accountID, Email: account.Email, Active: account.Active, Status: status, RoleNames: names})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].AccountID < result[j].AccountID })
	return result, stats(), nil
}

func (s *lifecycleStore) ListAccountPermissionGrants(_ context.Context, accountID, tenantID int64) ([]domain.PermissionGrant, domain.OperationStats, error) {
	return append([]domain.PermissionGrant(nil), s.grants[mappingKey{accountID, tenantID}]...), stats(), nil
}

func (s *lifecycleStore) DeleteAccountPermissionGrants(_ context.Context, accountID, tenantID int64) (int64, domain.OperationStats, error) {
	key := mappingKey{accountID, tenantID}
	deleted := int64(len(s.grants[key]))
	delete(s.grants, key)
	return deleted, stats(), nil
}

func (s *lifecycleStore) DeactivateTenantMapping(_ context.Context, accountID, tenantID int64) (domain.OperationStats, error) {
	s.inactive[mappingKey{accountID, tenantID}] = true
	return stats(), nil
}

func (s *lifecycleStore) ListAccountEmails(_ context.Context, accountIDs []int64) (map[int64]string, domain.OperationStats, error) {
	emails := map[int64]string{}
	for _, id := range accountIDs {
		if account, ok := s.accounts[id]; ok {
			emails[id] = account.Email
		}
	}
	return emails, stats(), nil
}

func (s *lifecycleStore) FindParentAccount(_ context.Context, id int64) (domain.ParentAccount, bool, domain.OperationStats, error) {
	account, ok := s.parents[id]
	return account, ok, stats(), nil
}

func (s *lifecycleStore) findParent(match func(domain.ParentAccount) bool) (domain.ParentAccount, bool, domain.OperationStats, error) {
	for _, account := range s.parents {
		if match(account) {
			return account, true, stats(), nil
		}
	}
	return domain.ParentAccount{}, false, stats(), nil
}

func (s *lifecycleStore) FindParentAccountByEmail(_ context.Context, email string) (domain.ParentAccount, bool, domain.OperationStats, error) {
	return s.findParent(func(a domain.ParentAccount) bool { return strings.EqualFold(a.Email, email) })
}

func (s *lifecycleStore) FindParentAccountByUsername(_ context.Context, username string) (domain.ParentAccount, bool, domain.OperationStats, error) {
	return s.findParent(func(a domain.ParentAccount) bool { return strings.EqualFold(a.Username, username) })
}

func (s *lifecycleStore) InsertParentAccount(_ context.Context, account domain.ParentAccount) (domain.ParentAccount, domain.OperationStats, error) {
	s.nextID++
	account.ID = s.nextID
	s.parents[account.ID] = account
	return account, stats(), nil
}

func (s *lifecycleStore) UpdateParentAccount(_ context.Context, account domain.ParentAccount) (bool, domain.OperationStats, error) {
	if _, ok := s.parents[account.ID]; !ok {
		return false, stats(), nil
	}
	s.parents[account.ID] = account
	return true, stats(), nil
}

func (s *lifecycleStore) ListParentAccounts(_ context.Context, filter domain.ParentAccountFilter) ([]domain.ParentAccount, domain.OperationStats, error) {
	var result []domain.ParentAccount
	for _, account := range s.parents {
		if filter.Email != "" && !strings.EqualFold(account.Email, filter.Email) {
			continue
		}
		if filter.Active != nil && account.Active != *filter.Active {
			continue
		}
		result = append(result, account)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, stats(), nil
}

// ports.Store overrides the relative access and school identity flows reach.

func (s *lifecycleStore) FindAccountByEmail(_ context.Context, email string) (domain.Account, bool, domain.OperationStats, error) {
	for _, account := range s.accounts {
		if strings.EqualFold(account.Email, email) {
			return domain.Account{ID: account.ID, Email: account.Email}, true, stats(), nil
		}
	}
	return domain.Account{}, false, stats(), nil
}

func (s *lifecycleStore) FindAccountsByEmails(_ context.Context, emails []string) (map[string]domain.Account, domain.OperationStats, error) {
	s.findAccountsByEmailsCalls++
	accounts := make(map[string]domain.Account)
	for _, email := range emails {
		for _, account := range s.accounts {
			if strings.EqualFold(account.Email, email) {
				accounts[strings.ToLower(email)] = domain.Account{ID: account.ID, Email: account.Email}
			}
		}
	}
	return accounts, stats(), nil
}

func (s *lifecycleStore) EnsureActiveTenantMapping(_ context.Context, accountID, tenantID int64) (domain.OperationStats, error) {
	key := mappingKey{accountID, tenantID}
	delete(s.inactive, key)
	for _, mapping := range s.mappings {
		if mapping == key {
			return stats(), nil
		}
	}
	s.mappings = append(s.mappings, key)
	return stats(), nil
}

func (s *lifecycleStore) FindRoleByName(context.Context, string, int64) (int64, bool, domain.OperationStats, error) {
	return 900, true, stats(), nil
}

func (s *lifecycleStore) AssignAccountRole(_ context.Context, accountID, _, _ int64) (bool, domain.OperationStats, error) {
	s.granted = append(s.granted, accountID)
	return true, stats(), nil
}

func (s *lifecycleStore) FindRFIDCard(_ context.Context, tag string, _ int64) (string, bool, domain.OperationStats, error) {
	return tag, s.cards[tag], stats(), nil
}

// --- staff directory -------------------------------------------------------

type lifecycleStaff struct {
	mu       sync.Mutex
	persons  map[int64]domain.PersonRecord
	staff    map[int64]domain.StaffMember
	teachers map[int64]domain.CaregiverProfile
	students map[int64]bool
	nextID   int64
	// positions records the caregiver role each created profile was given.
	positions []string

	findStaffErr     error
	createStaffErr   error
	createTeacherErr error
}

func newLifecycleStaff() *lifecycleStaff {
	return &lifecycleStaff{
		persons: map[int64]domain.PersonRecord{}, staff: map[int64]domain.StaffMember{},
		teachers: map[int64]domain.CaregiverProfile{}, students: map[int64]bool{}, nextID: 100,
	}
}

func (d *lifecycleStaff) id() int64 {
	d.nextID++
	return d.nextID
}

func (d *lifecycleStaff) addPerson(person domain.PersonRecord) domain.PersonRecord {
	d.mu.Lock()
	defer d.mu.Unlock()
	person.ID = d.id()
	if person.TenantID == 0 {
		person.TenantID = lifecycleTenant
	}
	d.persons[person.ID] = person
	return person
}

func (d *lifecycleStaff) addStaff(personID int64) domain.StaffMember {
	d.mu.Lock()
	defer d.mu.Unlock()
	member := domain.StaffMember{ID: d.id(), TenantID: lifecycleTenant, PersonID: personID}
	d.staff[member.ID] = member
	return member
}

func (d *lifecycleStaff) staffRows() []domain.StaffMember {
	d.mu.Lock()
	defer d.mu.Unlock()
	rows := make([]domain.StaffMember, 0, len(d.staff))
	for _, row := range d.staff {
		rows = append(rows, row)
	}
	return rows
}

func (d *lifecycleStaff) FindStaff(_ context.Context, staffID int64) (domain.StaffMember, bool, error) {
	if d.findStaffErr != nil {
		return domain.StaffMember{}, false, d.findStaffErr
	}
	member, ok := d.staff[staffID]
	return member, ok, nil
}

func (d *lifecycleStaff) FindPerson(_ context.Context, personID int64) (domain.PersonRecord, bool, error) {
	person, ok := d.persons[personID]
	return person, ok, nil
}

func (d *lifecycleStaff) FindPersonByAccount(_ context.Context, accountID int64) (domain.PersonRecord, bool, error) {
	for _, person := range d.persons {
		if person.AccountID != nil && *person.AccountID == accountID && !person.Deleted {
			return person, true, nil
		}
	}
	return domain.PersonRecord{}, false, nil
}

func (d *lifecycleStaff) FindPersonByTag(_ context.Context, tagID string) (domain.PersonRecord, bool, error) {
	for _, person := range d.persons {
		if person.TagID != nil && *person.TagID == tagID && !person.Deleted {
			return person, true, nil
		}
	}
	return domain.PersonRecord{}, false, nil
}

func (d *lifecycleStaff) FindPersonNames(_ context.Context, accountIDs []int64) (map[int64]domain.PersonName, error) {
	names := map[int64]domain.PersonName{}
	for _, id := range accountIDs {
		for _, person := range d.persons {
			if person.AccountID != nil && *person.AccountID == id {
				names[id] = domain.PersonName{FirstName: person.FirstName, LastName: person.LastName}
			}
		}
	}
	return names, nil
}

func (d *lifecycleStaff) CreatePerson(_ context.Context, person domain.PersonRecord) (int64, error) {
	return d.addPerson(person).ID, nil
}

func (d *lifecycleStaff) LinkPersonToAccount(_ context.Context, personID, accountID int64) error {
	person := d.persons[personID]
	person.AccountID = &accountID
	d.persons[personID] = person
	return nil
}

func (d *lifecycleStaff) LinkPersonToRFIDCard(_ context.Context, personID int64, tagID string) error {
	person := d.persons[personID]
	person.TagID = &tagID
	d.persons[personID] = person
	return nil
}

func (d *lifecycleStaff) IsStudentPerson(_ context.Context, personID int64) (bool, error) {
	return d.students[personID], nil
}

func (d *lifecycleStaff) FindStaffByPerson(_ context.Context, personID int64) (domain.StaffMember, bool, error) {
	for _, member := range d.staff {
		if member.PersonID == personID {
			return member, true, nil
		}
	}
	return domain.StaffMember{}, false, nil
}

func (d *lifecycleStaff) CreateStaff(_ context.Context, tenantID, personID int64) (int64, error) {
	if d.createStaffErr != nil {
		return 0, d.createStaffErr
	}
	member := d.addStaff(personID)
	member.TenantID = tenantID
	d.staff[member.ID] = member
	return member.ID, nil
}

func (d *lifecycleStaff) FindCaregiverProfile(_ context.Context, staffID int64) (domain.CaregiverProfile, bool, error) {
	for _, profile := range d.teachers {
		if profile.StaffID == staffID {
			return profile, true, nil
		}
	}
	return domain.CaregiverProfile{}, false, nil
}

func (d *lifecycleStaff) HasLiveCaregiverProfile(ctx context.Context, accountID int64) (bool, error) {
	person, found, err := d.FindPersonByAccount(ctx, accountID)
	if err != nil || !found {
		return false, err
	}
	member, found, err := d.FindStaffByPerson(ctx, person.ID)
	if err != nil || !found || member.Deleted {
		return false, err
	}
	profile, found, err := d.FindCaregiverProfile(ctx, member.ID)
	return found && !profile.Deleted, err
}

func (d *lifecycleStaff) CreateCaregiverProfile(_ context.Context, _, staffID int64, position string) (int64, error) {
	if d.createTeacherErr != nil {
		return 0, d.createTeacherErr
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	profile := domain.CaregiverProfile{ID: d.id(), StaffID: staffID}
	d.teachers[profile.ID] = profile
	d.positions = append(d.positions, position)
	return profile.ID, nil
}

// --- credentials, lockout, roles -----------------------------------------------

type lifecyclePINs struct{}

func (lifecyclePINs) HashPIN(pin string) (string, error) { return "pin:" + pin, nil }
func (lifecyclePINs) VerifyPIN(pin, hash string) bool    { return hash == "pin:"+pin }

type lifecycleLockout struct {
	threshold int
	duration  time.Duration
}

func (l lifecycleLockout) PINLockout(context.Context) (int, time.Duration) {
	return l.threshold, l.duration
}

// lifecycleRoles applies the classification the public package owns, over
// the domain facts: tier by base_role, system roles by name, Lehrkraft never
// a caregiver, the retired teacher role a caregiver.
type lifecycleRoles struct{}

func roleTier(role *domain.RoleFacts) string {
	if role.BaseRole != nil && strings.TrimSpace(*role.BaseRole) != "" {
		return strings.ToLower(strings.TrimSpace(*role.BaseRole))
	}
	if role.IsSystem {
		return strings.ToLower(strings.TrimSpace(role.Name))
	}
	return ""
}

func (lifecycleRoles) RoleNeedsStaffRecord(role *domain.RoleFacts) bool {
	return role != nil && roleTier(role) != "guardian"
}

func (lifecycleRoles) RoleNeedsCaregiverProfile(role *domain.RoleFacts) bool {
	if role == nil {
		return false
	}
	name := strings.ToLower(strings.TrimSpace(role.Name))
	if role.IsSystem && name == "lehrkraft" {
		return false
	}
	return roleTier(role) == "user" || (role.IsSystem && name == "teacher")
}

type lifecyclePasswords struct{}

func (lifecyclePasswords) ValidatePasswordStrength(password string) error {
	if len(password) < 8 {
		return errors.New("password doesn't meet complexity requirements")
	}
	return nil
}

func (lifecyclePasswords) HashPassword(password string) (string, error) {
	return "hash:" + password, nil
}

// --- preview audit and codec -----------------------------------------------

type lifecyclePreviewAudit struct {
	mu      sync.Mutex
	started []domain.StaffPreviewEvent
	ended   []domain.StaffPreviewEvent
	locks   []string
}

func (a *lifecyclePreviewAudit) RecordStaffPreviewStart(_ context.Context, event domain.StaffPreviewEvent) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.started = append(a.started, event)
	return nil
}

func (a *lifecyclePreviewAudit) RecordStaffPreviewEndOnce(_ context.Context, event domain.StaffPreviewEvent) (bool, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, ended := range a.ended {
		if ended.AdminAccountID == event.AdminAccountID && ended.PreviewID == event.PreviewID {
			return false, nil
		}
	}
	a.ended = append(a.ended, event)
	return true, nil
}

func (a *lifecyclePreviewAudit) LockStaffPreview(_ context.Context, adminAccountID int64, previewID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.locks = append(a.locks, fmt.Sprintf("%d:%s", adminAccountID, previewID))
	return nil
}

func (a *lifecyclePreviewAudit) StaffPreviewEnded(_ context.Context, adminAccountID int64, previewID string) (bool, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, ended := range a.ended {
		if ended.AdminAccountID == adminAccountID && ended.PreviewID == previewID {
			return true, nil
		}
	}
	return false, nil
}

// lifecycleCodec signs access-only tokens as prefixed JSON; the allow-expired
// parse ignores the expiry the way the real signer's variant does.
type lifecycleCodec struct{ fakeCodec }

func (c lifecycleCodec) IssueAccessToken(claims domain.SessionClaims) (string, error) {
	return c.encode("access:", claims), nil
}

func (lifecycleCodec) ParseAccessTokenAllowExpired(token string) (domain.SessionClaims, error) {
	var claims domain.SessionClaims
	if !strings.HasPrefix(token, "access:") {
		return claims, errors.New("not an access token")
	}
	err := json.Unmarshal([]byte(strings.TrimPrefix(token, "access:")), &claims)
	return claims, err
}

func (lifecycleCodec) AccessExpiry() time.Duration { return 15 * time.Minute }

func decodeAccessToken(t *testing.T, token string) domain.SessionClaims {
	t.Helper()
	claims, err := lifecycleCodec{}.ParseAccessTokenAllowExpired(token)
	require.NoError(t, err)
	return claims
}

// --- retained administration ------------------------------------------------

type lifecycleAdmin struct {
	store       *lifecycleStore
	removed     []int64
	deactivated []int64
}

func (a *lifecycleAdmin) RemoveRoleFromAccount(_ context.Context, accountID, roleID int64) error {
	a.removed = append(a.removed, roleID)
	for key, roles := range a.store.roles {
		if key.accountID != accountID {
			continue
		}
		kept := roles[:0]
		for _, role := range roles {
			if role.RoleID != roleID {
				kept = append(kept, role)
			}
		}
		a.store.roles[key] = kept
	}
	return nil
}

func (a *lifecycleAdmin) DeactivateAccount(_ context.Context, accountID int64) error {
	a.deactivated = append(a.deactivated, accountID)
	account := a.store.accounts[accountID]
	account.Active = false
	a.store.accounts[accountID] = account
	return nil
}

// --- guardian directory, invitations, delivery -----------------------------------

type lifecycleGuardians struct {
	profiles map[int64]domain.GuardianProfile
	links    map[int64]domain.StudentGuardianLink
	students map[int64]domain.Student
	names    map[int64]domain.PersonName
	locked   []int64
	nextID   int64

	findProfileErr   error
	createProfileErr error
	linkErr          error
	listByStudentErr error
	listByProfileErr error
	deleteProfileErr error
	findProfilesErr  error
	findStudentsErr  error
	promoted         []int64
	deletedLinks     []int64
	deletedProfiles  []int64
}

func newLifecycleGuardians() *lifecycleGuardians {
	return &lifecycleGuardians{
		profiles: map[int64]domain.GuardianProfile{}, links: map[int64]domain.StudentGuardianLink{},
		students: map[int64]domain.Student{}, names: map[int64]domain.PersonName{}, nextID: 500,
	}
}

func (g *lifecycleGuardians) addProfile(profile domain.GuardianProfile) domain.GuardianProfile {
	g.nextID++
	profile.ID = g.nextID
	g.profiles[profile.ID] = profile
	return profile
}

func (g *lifecycleGuardians) addLink(link domain.StudentGuardianLink) domain.StudentGuardianLink {
	g.nextID++
	link.ID = g.nextID
	g.links[link.ID] = link
	return link
}

func (g *lifecycleGuardians) linkOf(studentID, profileID int64) (domain.StudentGuardianLink, bool) {
	for _, link := range g.links {
		if link.StudentID == studentID && link.GuardianProfileID == profileID {
			return link, true
		}
	}
	return domain.StudentGuardianLink{}, false
}

func (g *lifecycleGuardians) FindGuardianProfileByEmail(_ context.Context, email string) (domain.GuardianProfile, bool, error) {
	if g.findProfileErr != nil {
		return domain.GuardianProfile{}, false, g.findProfileErr
	}
	for _, profile := range g.profiles {
		if strings.EqualFold(profile.Email, email) {
			return profile, true, nil
		}
	}
	return domain.GuardianProfile{}, false, nil
}

func (g *lifecycleGuardians) FindGuardianProfile(_ context.Context, id int64) (domain.GuardianProfile, bool, error) {
	if g.findProfileErr != nil {
		return domain.GuardianProfile{}, false, g.findProfileErr
	}
	profile, ok := g.profiles[id]
	return profile, ok, nil
}

func (g *lifecycleGuardians) FindGuardianProfiles(_ context.Context, ids []int64) (map[int64]domain.GuardianProfile, error) {
	if g.findProfilesErr != nil {
		return nil, g.findProfilesErr
	}
	result := map[int64]domain.GuardianProfile{}
	for _, id := range ids {
		if profile, ok := g.profiles[id]; ok {
			result[id] = profile
		}
	}
	return result, nil
}

func (g *lifecycleGuardians) CreateGuardianProfile(_ context.Context, profile domain.GuardianProfile) (int64, error) {
	if g.createProfileErr != nil {
		return 0, g.createProfileErr
	}
	return g.addProfile(profile).ID, nil
}

func (g *lifecycleGuardians) DeleteGuardianProfile(_ context.Context, id int64) error {
	if g.deleteProfileErr != nil {
		return g.deleteProfileErr
	}
	g.deletedProfiles = append(g.deletedProfiles, id)
	delete(g.profiles, id)
	return nil
}

func (g *lifecycleGuardians) LinkGuardianProfileToAccount(_ context.Context, profileID, accountID int64) error {
	profile := g.profiles[profileID]
	profile.AccountID = &accountID
	profile.HasAccount = true
	g.profiles[profileID] = profile
	return nil
}

func (g *lifecycleGuardians) FindStudentGuardianLinkForUpdate(_ context.Context, studentID, guardianProfileID int64) (domain.StudentGuardianLink, bool, error) {
	link, ok := g.linkOf(studentID, guardianProfileID)
	return link, ok, nil
}

func (g *lifecycleGuardians) LinkStudentGuardianIfAbsent(_ context.Context, link domain.StudentGuardianLink) (bool, error) {
	if g.linkErr != nil {
		return false, g.linkErr
	}
	if _, ok := g.linkOf(link.StudentID, link.GuardianProfileID); ok {
		return false, nil
	}
	link.GuardianRole = "legal_guardian"
	g.addLink(link)
	return true, nil
}

func (g *lifecycleGuardians) PromoteStudentGuardianLink(_ context.Context, linkID int64) error {
	g.promoted = append(g.promoted, linkID)
	link := g.links[linkID]
	link.GuardianRole = "legal_guardian"
	g.links[linkID] = link
	return nil
}

func (g *lifecycleGuardians) sortedLinks(match func(domain.StudentGuardianLink) bool) []domain.StudentGuardianLink {
	var result []domain.StudentGuardianLink
	for _, link := range g.links {
		if match(link) {
			result = append(result, link)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func (g *lifecycleGuardians) ListStudentGuardianLinksByStudents(_ context.Context, studentIDs []int64) ([]domain.StudentGuardianLink, error) {
	if g.listByStudentErr != nil {
		return nil, g.listByStudentErr
	}
	return g.sortedLinks(func(link domain.StudentGuardianLink) bool { return slices.Contains(studentIDs, link.StudentID) }), nil
}

func (g *lifecycleGuardians) ListStudentGuardianLinksByProfile(_ context.Context, profileID int64) ([]domain.StudentGuardianLink, error) {
	if g.listByProfileErr != nil {
		return nil, g.listByProfileErr
	}
	return g.sortedLinks(func(link domain.StudentGuardianLink) bool { return link.GuardianProfileID == profileID }), nil
}

func (g *lifecycleGuardians) DeleteStudentGuardianLink(_ context.Context, linkID int64) error {
	g.deletedLinks = append(g.deletedLinks, linkID)
	delete(g.links, linkID)
	return nil
}

func (g *lifecycleGuardians) GuardianRoleClass(role string) domain.GuardianRoleClass {
	switch role {
	case "primary_guardian", "legal_guardian", "co_guardian":
		return domain.GuardianRoleFull
	case "social_worker":
		return domain.GuardianRoleSocialWorker
	default:
		return domain.GuardianRoleRestricted
	}
}

func (g *lifecycleGuardians) FindStudents(_ context.Context, ids []int64) (map[int64]domain.Student, error) {
	if g.findStudentsErr != nil {
		return nil, g.findStudentsErr
	}
	result := map[int64]domain.Student{}
	for _, id := range ids {
		if student, ok := g.students[id]; ok {
			result[id] = student
		}
	}
	return result, nil
}

func (g *lifecycleGuardians) LockStudent(_ context.Context, studentID int64) error {
	g.locked = append(g.locked, studentID)
	return nil
}

func (g *lifecycleGuardians) FindPersonNamesByIDs(_ context.Context, ids []int64) (map[int64]domain.PersonName, error) {
	result := map[int64]domain.PersonName{}
	for _, id := range ids {
		if name, ok := g.names[id]; ok {
			result[id] = name
		}
	}
	return result, nil
}

type lifecycleInvitations struct {
	rows       map[int64]domain.GuardianInvitation
	accounts   []domain.LoginAccount
	nextID     int64
	findErr    error
	listErr    error
	insertErr  error
	updateErr  error
	pendingErr error
}

func newLifecycleInvitations() *lifecycleInvitations {
	return &lifecycleInvitations{rows: map[int64]domain.GuardianInvitation{}, nextID: 900}
}

func (s *lifecycleInvitations) add(invitation domain.GuardianInvitation) domain.GuardianInvitation {
	s.nextID++
	invitation.ID = s.nextID
	s.rows[invitation.ID] = invitation
	return invitation
}

func (s *lifecycleInvitations) FindGuardianInvitation(_ context.Context, id int64) (domain.GuardianInvitation, bool, error) {
	if s.findErr != nil {
		return domain.GuardianInvitation{}, false, s.findErr
	}
	invitation, ok := s.rows[id]
	return invitation, ok, nil
}

func (s *lifecycleInvitations) sorted(match func(domain.GuardianInvitation) bool) []domain.GuardianInvitation {
	var result []domain.GuardianInvitation
	for _, invitation := range s.rows {
		if match(invitation) {
			result = append(result, invitation)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func (s *lifecycleInvitations) ListGuardianInvitationsByProfile(_ context.Context, profileID int64) ([]domain.GuardianInvitation, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.sorted(func(invitation domain.GuardianInvitation) bool { return invitation.GuardianProfileID == profileID }), nil
}

func (s *lifecycleInvitations) ListPendingGuardianApprovals(context.Context) ([]domain.GuardianInvitation, error) {
	if s.pendingErr != nil {
		return nil, s.pendingErr
	}
	return s.sorted(func(invitation domain.GuardianInvitation) bool { return invitation.IsPendingApproval() }), nil
}

func (s *lifecycleInvitations) InsertGuardianInvitation(_ context.Context, invitation domain.GuardianInvitation) (domain.GuardianInvitation, error) {
	if s.insertErr != nil {
		return domain.GuardianInvitation{}, s.insertErr
	}
	return s.add(invitation), nil
}

func (s *lifecycleInvitations) UpdateGuardianInvitation(_ context.Context, invitation domain.GuardianInvitation) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	s.rows[invitation.ID] = invitation
	return nil
}

func (s *lifecycleInvitations) FindGuardianInvitationByToken(_ context.Context, token string) (domain.GuardianInvitation, bool, error) {
	if s.findErr != nil {
		return domain.GuardianInvitation{}, false, s.findErr
	}
	for _, invitation := range s.sorted(func(row domain.GuardianInvitation) bool { return row.Token == token }) {
		return invitation, true, nil
	}
	return domain.GuardianInvitation{}, false, nil
}

func (s *lifecycleInvitations) ListOpenGuardianInvitations(_ context.Context, profileIDs []int64, now time.Time) ([]domain.GuardianInvitation, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	wanted := map[int64]bool{}
	for _, id := range profileIDs {
		wanted[id] = true
	}
	return s.sorted(func(invitation domain.GuardianInvitation) bool {
		return wanted[invitation.GuardianProfileID] && guardianInvitationNonFinal(invitation, now)
	}), nil
}

func (s *lifecycleInvitations) ListRedeemableGuardianInvitations(_ context.Context, now time.Time) ([]domain.GuardianInvitation, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.sorted(func(invitation domain.GuardianInvitation) bool {
		return guardianInvitationNonFinal(invitation, now) && !invitation.IsPendingApproval()
	}), nil
}

func (s *lifecycleInvitations) AcceptGuardianInvitation(_ context.Context, id int64, acceptedAt time.Time) (bool, error) {
	if s.updateErr != nil {
		return false, s.updateErr
	}
	invitation, ok := s.rows[id]
	if !ok || invitation.AcceptedAt != nil {
		return false, nil
	}
	invitation.AcceptedAt = &acceptedAt
	s.rows[id] = invitation
	return true, nil
}

// InsertAccount mirrors the store's account provisioning; the fake accounts
// live in the shared session store.
func (s *lifecycleInvitations) InsertAccount(_ context.Context, email, passwordHash string) (domain.LoginAccount, domain.OperationStats, error) {
	if s.insertErr != nil {
		return domain.LoginAccount{}, domain.OperationStats{}, s.insertErr
	}
	s.nextID++
	account := domain.LoginAccount{ID: s.nextID, Email: email, PasswordHash: passwordHash, Active: true}
	s.accounts = append(s.accounts, account)
	return account, domain.OperationStats{Queries: 1, Rows: 1}, nil
}

// lifecycleEnrollments records the claims an acceptance runs.
type lifecycleEnrollments struct {
	claimed map[int64]string
	err     error
}

func newLifecycleEnrollments() *lifecycleEnrollments {
	return &lifecycleEnrollments{claimed: map[int64]string{}}
}

func (e *lifecycleEnrollments) ClaimGuardianEnrollments(_ context.Context, accountID int64, email string) (int, error) {
	if e.err != nil {
		return 0, e.err
	}
	e.claimed[accountID] = email
	return 1, nil
}

type lifecycleDelivery struct {
	emails       []domain.GuardianInvitation
	accessEmails []domain.GuardianProfile
}

func (lifecycleDelivery) InvitationExpiry(context.Context) time.Duration { return 48 * time.Hour }
func (lifecycleDelivery) SchoolName(context.Context, int64) string       { return "OGS Musterschule" }
func (d *lifecycleDelivery) EnqueueInvitationEmail(_ context.Context, invitation domain.GuardianInvitation, _ domain.GuardianProfile, _ string) {
	d.emails = append(d.emails, invitation)
}
func (d *lifecycleDelivery) EnqueueExistingAccountEmail(_ context.Context, profile domain.GuardianProfile, _ string) {
	d.accessEmails = append(d.accessEmails, profile)
}

type lifecycleFinancial struct {
	removed []int64
	err     error
}

func (f *lifecycleFinancial) RecordPayerRemoved(_ context.Context, guardianProfileID, _, _ int64) error {
	if f.err != nil {
		return f.err
	}
	f.removed = append(f.removed, guardianProfileID)
	return nil
}

// --- fixture ------------------------------------------------------------------

type lifecycleFixture struct {
	lifecycle   *AccountLifecycle
	store       *lifecycleStore
	staff       *lifecycleStaff
	lockout     *lifecycleLockout
	audit       *lifecyclePreviewAudit
	admin       *lifecycleAdmin
	guardians   *lifecycleGuardians
	invitations *lifecycleInvitations
	delivery    *lifecycleDelivery
	enrollments *lifecycleEnrollments
	schools     *fakeSchools
	financial   *lifecycleFinancial
	runtime     *fakeRuntime
	sessions    *fakeStore
	persons     *fakePersons
}

func newLifecycleFixture(t *testing.T) *lifecycleFixture {
	t.Helper()
	store := newLifecycleStore()
	runtime := &fakeRuntime{}
	sessions := New(store, store, store, fakeTransaction{}, runtime.TenantID, func(ports.Observation) {})
	logger := slog.New(slog.NewTextHandler(testWriter{t}, nil))
	schools := newFakeSchools()
	schools.add(lifecycleTenant, lifecycleTenant*10, "lifecycle", true, false)
	persons := newFakePersons(runtime)
	auth, err := NewAccountAuthentication(sessions, AccountAuthenticationDependencies{
		Store: store, Schools: schools, Persons: persons, Passwords: fakePasswords{}, Codec: fakeCodec{},
		MFA: &fakeMFA{}, MFALock: &fakeMFALock{store: store.fakeStore}, Audit: &fakeAudit{}, Push: &fakePush{},
		Runtime: runtime, Rotation: fakeRotation{}, Logger: logger,
	})
	require.NoError(t, err)
	f := &lifecycleFixture{
		store: store, staff: newLifecycleStaff(), lockout: &lifecycleLockout{}, audit: &lifecyclePreviewAudit{},
		guardians: newLifecycleGuardians(), invitations: newLifecycleInvitations(), delivery: &lifecycleDelivery{},
		enrollments: newLifecycleEnrollments(), schools: schools,
		financial: &lifecycleFinancial{}, runtime: runtime, sessions: store.fakeStore, persons: persons,
	}
	f.admin = &lifecycleAdmin{store: store}
	f.lifecycle, err = NewAccountLifecycle(sessions, auth, AccountLifecycleDependencies{
		Store: store, Logins: store, RFID: store, Staff: f.staff, Profiles: f.staff, Roles: lifecycleRoles{}, PINs: lifecyclePINs{},
		Lockout: f.lockout, Audit: f.audit, Codec: lifecycleCodec{}, Admin: f.admin, Passwords: lifecyclePasswords{},
		Guardians: f.guardians, Invitations: f.invitations, Delivery: f.delivery, Enrollments: f.enrollments,
		Schools: f.schools, Financial: f.financial,
		Runtime: runtime, Logger: logger,
	})
	require.NoError(t, err)
	return f
}

func tenantContext() context.Context {
	return (&fakeRuntime{}).WithTenantID(context.Background(), lifecycleTenant)
}

// seedPINStaff seeds an active account with a PIN, its person and staff row
// at the lifecycle tenant, and returns the staff id.
func (f *lifecycleFixture) seedPINStaff(accountID int64, pin string) int64 {
	f.store.addAccount(accountID, fmt.Sprintf("staff%d@example.com", accountID), "hash:secret", true)
	f.store.addMapping(accountID, lifecycleTenant)
	f.store.pins[accountID] = domain.PINAccount{ID: accountID, Active: true, PINHash: "pin:" + pin, UpdatedAt: time.Now()}
	person := f.staff.addPerson(domain.PersonRecord{FirstName: "Kiosk", LastName: "Kraft", AccountID: &accountID})
	return f.staff.addStaff(person.ID).ID
}
