package application

import (
	"context"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/traegerview"
)

// Who acted in an access log entry (#4655). A ledger records the acting
// account, or for staff master data and personnel numbers the acting staff
// member; the access log shows both as the person and account behind them.

type accountSchool struct{ account, school int64 }

// actorRef is who acted in one entry as the trail records it: an account or
// a staff member (or neither) at a school, 0 for the Träger's own data, and
// the name a ledger stored.
type actorRef struct {
	school   int64
	account  *int64
	staff    *int64
	snapshot string
}

// actor is the account and the shown name of who acted in one entry.
type actor struct {
	account *int64
	name    string
}

// actorIndex holds the persons of accounts at schools, the persons behind
// staff IDs and the addresses of the Träger-Büro.
type actorIndex struct {
	persons map[accountSchool]string
	staff   map[int64]traegerview.Person
	bureau  map[int64]string
}

// actorIndex resolves the actors of the entries in the projection's
// administrative unit. It reads the Träger-Büro addresses only for accounts
// that neither a person at the entry's school nor a stored name names.
func (o *Overview) actorIndex(adminCtx context.Context, organizationID int64, schools []int64, refs []actorRef) (actorIndex, error) {
	index := actorIndex{persons: map[accountSchool]string{}, staff: map[int64]traegerview.Person{}, bureau: map[int64]string{}}
	accounts, staffIDs := refIDs(refs)
	if len(staffIDs) > 0 {
		staffPersons, err := o.staffPersons(adminCtx, schools, staffIDs)
		if err != nil {
			return index, err
		}
		accounts = append(accounts, index.addStaff(staffPersons)...)
	}
	if accounts = distinctIDs(accounts); len(accounts) == 0 {
		return index, nil
	}
	persons, err := o.persons.PersonsByAccount(adminCtx, accounts)
	if err != nil {
		return index, err
	}
	index.addPersons(persons)
	unnamed := index.unnamed(refs)
	if len(unnamed) == 0 {
		return index, nil
	}
	addresses, err := o.bureau.BureauAddresses(adminCtx, organizationID, unnamed)
	if err != nil {
		return index, err
	}
	for _, address := range addresses {
		index.bureau[address.AccountID] = address.Name
	}
	return index, nil
}

// refIDs lists the accounts and the distinct staff IDs of the entries.
func refIDs(refs []actorRef) (accounts, staffIDs []int64) {
	for _, ref := range refs {
		if ref.account != nil {
			accounts = append(accounts, *ref.account)
		}
		if ref.staff != nil {
			staffIDs = append(staffIDs, *ref.staff)
		}
	}
	return accounts, distinctIDs(staffIDs)
}

// addStaff keeps the persons behind staff IDs and returns their accounts.
func (i actorIndex) addStaff(staffPersons map[int64]traegerview.Person) []int64 {
	accounts := []int64{}
	for staffID, person := range staffPersons {
		i.staff[staffID] = person
		if person.AccountID != nil {
			accounts = append(accounts, *person.AccountID)
		}
	}
	return accounts
}

func (i actorIndex) addPersons(persons []traegerview.Person) {
	for _, person := range persons {
		if person.AccountID != nil {
			i.persons[accountSchool{account: *person.AccountID, school: person.SchoolID}] = person.Name
		}
	}
}

// unnamed lists the accounts that neither a person at the entry's school nor
// a stored name names.
func (i actorIndex) unnamed(refs []actorRef) []int64 {
	accounts := []int64{}
	for _, ref := range refs {
		if ref.account != nil && ref.snapshot == "" && i.persons[accountSchool{account: *ref.account, school: ref.school}] == "" {
			accounts = append(accounts, *ref.account)
		}
	}
	return distinctIDs(accounts)
}

// staffPersons maps staff IDs of the Träger's schools to their persons at
// the same school.
func (o *Overview) staffPersons(adminCtx context.Context, schools, staffIDs []int64) (map[int64]traegerview.Person, error) {
	members, err := o.staff.StaffByID(adminCtx, schools, staffIDs)
	if err != nil || len(members) == 0 {
		return map[int64]traegerview.Person{}, err
	}
	personIDs := make([]int64, 0, len(members))
	for _, member := range members {
		personIDs = append(personIDs, member.PersonID)
	}
	persons, err := o.persons.PersonsByID(adminCtx, distinctIDs(personIDs))
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]traegerview.Person, len(persons))
	for _, person := range persons {
		byID[person.PersonID] = person
	}
	result := make(map[int64]traegerview.Person, len(members))
	for _, member := range members {
		if person, ok := byID[member.PersonID]; ok && person.SchoolID == member.SchoolID {
			result[member.StaffID] = person
		}
	}
	return result, nil
}

// staffOfAccount lists the staff IDs the account's persons hold at the
// Träger's schools, so a person filter also finds the ledgers that record
// the staff member.
func (o *Overview) staffOfAccount(adminCtx context.Context, schools []int64, accountID int64) ([]int64, error) {
	persons, err := o.persons.PersonsByAccount(adminCtx, []int64{accountID})
	if err != nil {
		return nil, err
	}
	personIDs := []int64{}
	for _, person := range persons {
		if slices.Contains(schools, person.SchoolID) {
			personIDs = append(personIDs, person.PersonID)
		}
	}
	if len(personIDs) == 0 {
		return nil, nil
	}
	members, err := o.staff.StaffOfPersons(adminCtx, schools, personIDs)
	if err != nil {
		return nil, err
	}
	staffIDs := make([]int64, 0, len(members))
	for _, member := range members {
		staffIDs = append(staffIDs, member.StaffID)
	}
	return staffIDs, nil
}

// resolve names the actor of an entry: a staff member by its person, an
// account by its person at the entry's school, else by the name the ledger
// stored, else by its address in the Träger-Büro.
func (i actorIndex) resolve(ref actorRef) actor {
	school, account, staff, snapshot := ref.school, ref.account, ref.staff, ref.snapshot
	if account == nil && staff != nil {
		if person, ok := i.staff[*staff]; ok && person.Name != "" {
			return actor{account: person.AccountID, name: person.Name}
		}
		return actor{name: knownOr(snapshot, personUnknown)}
	}
	if account == nil {
		return actor{name: knownOr(snapshot, personWithoutAccount)}
	}
	if name := i.persons[accountSchool{account: *account, school: school}]; name != "" {
		return actor{account: account, name: name}
	}
	if snapshot != "" {
		return actor{account: account, name: snapshot}
	}
	if address := i.bureau[*account]; address != "" {
		return actor{account: account, name: address + traegerBueroSuffix}
	}
	return actor{account: account, name: personUnknown}
}

func knownOr(name, fallback string) string {
	if name != "" {
		return name
	}
	return fallback
}

// trailActorRef is who acted as the trail's actor list records it.
func trailActorRef(trailActor traegerview.AccessTrailActor) actorRef {
	return actorRef{school: trailActor.SchoolID, account: positiveID(trailActor.AccountID),
		staff: positiveID(trailActor.StaffID), snapshot: trailActor.ActorNameSnapshot}
}

func positiveID(id int64) *int64 {
	if id <= 0 {
		return nil
	}
	return &id
}

func distinctIDs(ids []int64) []int64 {
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id > 0 {
			result = append(result, id)
		}
	}
	slices.Sort(result)
	return slices.Compact(result)
}
