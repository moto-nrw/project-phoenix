package application

import (
	"context"
	"maps"
	"slices"
	"sync"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// callerEntry memoizes the identity chain of one (tenant, account) pair for
// the lifetime of one request (#2099, modeled on the settings request cache
// from #2065). Keying by tenant as well as account means a memo attached to
// a context that touches several tenants never serves one tenant's identity
// to another.
//
// Consistency contract:
//   - lifetime is one request; the memo dies with the request context
//   - only successful outcomes and clean not-found results (account not
//     linked to a person, person not staff, staff not a teacher) are
//     memoized; lookup errors and partially loaded group sets never are
//   - UpdateProfile and UpdateAvatar drop the caller's whole entry before
//     their trailing profile re-read, so a same-request read after a
//     self-write sees committed state
//   - deliberately exempt from eviction: group transfer/cancel writes a
//     substitution for a DIFFERENT staff member (self-transfer is rejected)
//     and renders without re-reading identity; admin substitution endpoints,
//     staff offboarding and invitation acceptance mutate another account's
//     chain or run for an account that is not authenticated in that request
//   - writes committed by other transactions become visible at the latest
//     with the next request
//   - entries are not date-keyed: the school's current day is stable within
//     a request span, and the only long-lived context (SSE) resolves
//     identity once at connection setup and never re-reads
//   - there is deliberately NO process-wide cache and NO TTL
//
// A nil entry is the "no memo" case (scheduler, CLI, device auth, contexts
// without the request memo or without an authenticated account): every read
// misses and every store is dropped.
//
// Each stage has its own loaded flag; a loaded stage with a zero value is a
// clean "not linked" outcome, not an error.
type callerEntry struct {
	mu            sync.Mutex
	accountLoaded bool
	account       domain.AccountMetadata
	personLoaded  bool
	person        *domain.CallerPerson
	staffLoaded   bool
	staffID       int64
	teacherLoaded bool
	teacherID     int64
	groupsLoaded  bool
	groups        []int64
	subsLoaded    bool
	subs          map[int64]bool
	classesLoaded bool
	classes       []string
}

// entry resolves the memo entry of the current caller, or nil when the
// request carries no memo or no authenticated account.
func (c *CallerContext) entry(ctx context.Context) *callerEntry {
	memo := c.deps.Memo(ctx)
	if memo == nil {
		return nil
	}
	caller := c.principal(ctx)
	if caller.AccountID <= 0 {
		return nil
	}
	value := memo.Entry(caller.TenantID, caller.AccountID, func() any { return &callerEntry{} })
	entry, _ := value.(*callerEntry)
	return entry
}

// invalidate drops every memoized stage of the current caller. The
// self-editing profile use cases call it before their trailing re-read.
func (c *CallerContext) invalidate(ctx context.Context) {
	memo := c.deps.Memo(ctx)
	if memo == nil {
		return
	}
	caller := c.principal(ctx)
	if caller.AccountID <= 0 {
		return
	}
	memo.Evict(caller.TenantID, caller.AccountID)
}

func (e *callerEntry) cachedAccount() (domain.AccountMetadata, bool) {
	if e == nil {
		return domain.AccountMetadata{}, false
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.account, e.accountLoaded
}

func (e *callerEntry) storeAccount(account domain.AccountMetadata) {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.account, e.accountLoaded = account, true
}

func (e *callerEntry) cachedPerson() (*domain.CallerPerson, bool) {
	if e == nil {
		return nil, false
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.personLoaded || e.person == nil {
		return nil, e.personLoaded
	}
	person := *e.person
	return &person, true
}

func (e *callerEntry) storePerson(person *domain.CallerPerson) {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if person != nil {
		copied := *person
		person = &copied
	}
	e.person, e.personLoaded = person, true
}

func (e *callerEntry) cachedStaff() (int64, bool) {
	if e == nil {
		return 0, false
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.staffID, e.staffLoaded
}

func (e *callerEntry) storeStaff(staffID int64) {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.staffID, e.staffLoaded = staffID, true
}

func (e *callerEntry) cachedTeacher() (int64, bool) {
	if e == nil {
		return 0, false
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.teacherID, e.teacherLoaded
}

func (e *callerEntry) storeTeacher(teacherID int64) {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.teacherID, e.teacherLoaded = teacherID, true
}

// cachedGroups returns a copy on every hit: a caller sorting its result in
// place must not reorder the memoized value.
func (e *callerEntry) cachedGroups() ([]int64, bool) {
	if e == nil {
		return nil, false
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.groupsLoaded {
		return nil, false
	}
	return slices.Clone(e.groups), true
}

func (e *callerEntry) storeGroups(groups []int64) {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.groups, e.groupsLoaded = slices.Clone(groups), true
}

// cachedSubstitutions returns a copy so the result is never a live alias of
// the memoized map.
func (e *callerEntry) cachedSubstitutions() (map[int64]bool, bool) {
	if e == nil {
		return nil, false
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.subsLoaded {
		return nil, false
	}
	return maps.Clone(e.subs), true
}

func (e *callerEntry) storeSubstitutions(subs map[int64]bool) {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.subs, e.subsLoaded = maps.Clone(subs), true
}

// cachedSchoolClasses copies for the same reason as cachedGroups.
func (e *callerEntry) cachedSchoolClasses() ([]string, bool) {
	if e == nil {
		return nil, false
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.classesLoaded {
		return nil, false
	}
	return slices.Clone(e.classes), true
}

func (e *callerEntry) storeSchoolClasses(classes []string) {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.classes, e.classesLoaded = slices.Clone(classes), true
}
