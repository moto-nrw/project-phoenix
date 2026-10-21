// Package staffpersonmerge repairs reviewed staff person duplicates without
// merging employment records. It owns no data and issues no SQL.
package staffpersonmerge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

var (
	ErrInvalid      = errors.New("select reviewed cases and confirm school consultation")
	ErrConflict     = errors.New("staff person merge preview is no longer current")
	ErrUnauthorized = errors.New("staff person merge requires an operator")
)

type Employment struct {
	ID        int64      `json:"id,string"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at"`
}
type PersonSnapshot struct {
	ID        int64     `json:"id,string"`
	TenantID  int64     `json:"tenant_id,string"`
	AccountID *int64    `json:"account_id,string,omitempty"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Birthday  string    `json:"birthday,omitempty"`
	TagID     *string   `json:"tag_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Side struct {
	Person     PersonSnapshot `json:"person"`
	Employment Employment     `json:"employment"`
}
type Candidate struct {
	Keeper   Side   `json:"keeper"`
	Donor    Side   `json:"donor"`
	Revision string `json:"revision"`
}
type Selection struct {
	DonorID  int64  `json:"donor_id,string"`
	KeeperID int64  `json:"keeper_id,string"`
	Revision string `json:"revision"`
}
type Request struct {
	SchoolConfirmed bool        `json:"school_confirmed"`
	Cases           []Selection `json:"cases"`
}

type Runtime interface {
	RequireAdmin(context.Context) error
	Lock(context.Context, int64) error
	MarkRollback(context.Context)
	OperatorID(context.Context) int64
}
type Audit interface {
	RecordStaffPersonMerge(context.Context, int64, int64, Candidate) error
}
type Workflow struct {
	people      peopledirectory.StaffPersonMerge
	memberships schoolmembership.StaffPersonMerge
	runtime     Runtime
	audit       Audit
}

func New(people peopledirectory.StaffPersonMerge, memberships schoolmembership.StaffPersonMerge, runtime Runtime, audit Audit) (*Workflow, error) {
	if people == nil || memberships == nil || runtime == nil || audit == nil {
		return nil, errors.New("staff person merge: all dependencies required")
	}
	return &Workflow{people: people, memberships: memberships, runtime: runtime, audit: audit}, nil
}
func (w *Workflow) Preview(ctx context.Context, schoolID int64) ([]Candidate, error) {
	if w.runtime.OperatorID(ctx) <= 0 {
		return nil, ErrUnauthorized
	}
	if schoolID <= 0 {
		return nil, ErrInvalid
	}
	if err := w.runtime.RequireAdmin(ctx); err != nil {
		return nil, err
	}
	return w.candidates(ctx, schoolID, false)
}

// Execute locks the school repair gate, all live people by ID, then all
// employments by ID. Every selected fingerprint is checked before any write.
// The caller must supply one administrative unit for the entire batch,
// including its audit entries, and roll it back on error. HTTP callers use
// the standing TenantTxMiddleware after the operator session gates.
func (w *Workflow) Execute(ctx context.Context, schoolID int64, request Request) error {
	actor := w.runtime.OperatorID(ctx)
	if actor <= 0 {
		return ErrUnauthorized
	}
	if schoolID <= 0 || !request.SchoolConfirmed || len(request.Cases) == 0 || len(request.Cases) > 100 {
		return ErrInvalid
	}
	if err := w.runtime.RequireAdmin(ctx); err != nil {
		return err
	}
	if err := w.runtime.Lock(ctx, schoolID); err != nil {
		return err
	}
	candidates, err := w.candidates(ctx, schoolID, true)
	if err != nil {
		return err
	}
	selected, err := selectCandidates(candidates, request.Cases)
	if err != nil {
		return err
	}
	if err := w.executeSelected(ctx, actor, schoolID, selected); err != nil {
		w.runtime.MarkRollback(ctx)
		return err
	}
	return nil
}

func (w *Workflow) executeSelected(ctx context.Context, actor, schoolID int64, selected []Candidate) error {
	for _, candidate := range selected {
		if err := w.memberships.ReassignStaffPerson(ctx, schoolmembership.StaffPersonReassignment{SchoolID: schoolID, StaffID: candidate.Donor.Employment.ID, DonorPersonID: candidate.Donor.Person.ID, KeeperPersonID: candidate.Keeper.Person.ID}); err != nil {
			return err
		}
		if err := w.people.RetireMergedStaffPerson(ctx, schoolID, candidate.Donor.Person.ID, candidate.Keeper.Person.ID); err != nil {
			if errors.Is(err, peopledirectory.ErrStaffPersonMergeNoLongerEligible) {
				return ErrConflict
			}
			return err
		}
		if err := w.audit.RecordStaffPersonMerge(ctx, actor, schoolID, candidate); err != nil {
			return err
		}
	}
	return nil
}

func selectCandidates(candidates []Candidate, selections []Selection) ([]Candidate, error) {
	byDonor := make(map[int64]Candidate, len(candidates))
	for _, candidate := range candidates {
		byDonor[candidate.Donor.Person.ID] = candidate
	}
	result := make([]Candidate, 0, len(selections))
	seen := make(map[int64]bool)
	for _, selection := range selections {
		candidate, ok := byDonor[selection.DonorID]
		if !ok || seen[selection.DonorID] || candidate.Keeper.Person.ID != selection.KeeperID || candidate.Revision != selection.Revision {
			return nil, ErrConflict
		}
		seen[selection.DonorID] = true
		result = append(result, candidate)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Donor.Person.ID < result[j].Donor.Person.ID })
	return result, nil
}
func (w *Workflow) candidates(ctx context.Context, schoolID int64, lock bool) ([]Candidate, error) {
	people, err := w.people.StaffMergePeople(ctx, schoolID, lock)
	if err != nil {
		return nil, err
	}
	memberships, err := w.memberships.StaffMergeMemberships(ctx, schoolID, lock)
	if err != nil {
		return nil, err
	}
	byPerson := make(map[int64][]schoolmembership.Staff)
	for _, staff := range memberships {
		byPerson[staff.PersonID] = append(byPerson[staff.PersonID], staff)
	}
	groups := make(map[string][]peopledirectory.StaffMergePerson)
	for _, person := range people {
		groups[person.NameKey] = append(groups[person.NameKey], person)
	}
	result := []Candidate{}
	for _, group := range groups {
		candidate, ok, err := candidateFor(group, byPerson)
		if err != nil {
			return nil, err
		}
		if ok {
			result = append(result, candidate)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Keeper.Person.ID < result[j].Keeper.Person.ID })
	return result, nil
}
func candidateFor(group []peopledirectory.StaffMergePerson, staff map[int64][]schoolmembership.Staff) (Candidate, bool, error) {
	if len(group) != 2 {
		return Candidate{}, false, nil
	}
	keeper, donor := group[0], group[1]
	if keeper.AccountID == nil {
		keeper, donor = donor, keeper
	}
	if keeper.AccountID == nil || donor.AccountID != nil || keeper.HasStudent || donor.HasStudent || donor.TagID != nil || (keeper.Birthday != "" && donor.Birthday != "" && keeper.Birthday != donor.Birthday) {
		return Candidate{}, false, nil
	}
	ks, ds := staff[keeper.ID], staff[donor.ID]
	if len(ks) != 1 || len(ds) != 1 || (ks[0].DeletedAt == nil) == (ds[0].DeletedAt == nil) || ks[0].PersonalDataErasedAt != nil || ds[0].PersonalDataErasedAt != nil {
		return Candidate{}, false, nil
	}
	side := func(p peopledirectory.Person, s schoolmembership.Staff) Side {
		return Side{Person: PersonSnapshot{ID: p.ID, TenantID: p.TenantID, AccountID: p.AccountID, FirstName: p.FirstName, LastName: p.LastName, Birthday: p.Birthday, TagID: p.TagID, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt}, Employment: Employment{ID: s.ID, CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt, DeletedAt: s.DeletedAt}}
	}
	candidate := Candidate{Keeper: side(keeper.Person, ks[0]), Donor: side(donor.Person, ds[0])}
	// All fields have deterministic JSON encodings; no maps or credentials.
	encoded, err := json.Marshal(candidate)
	if err != nil {
		return Candidate{}, false, err
	}
	sum := sha256.Sum256(encoded)
	candidate.Revision = hex.EncodeToString(sum[:])
	return candidate, true, nil
}
