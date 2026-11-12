package importpkg

import (
	"context"
	"fmt"
	"strings"

	importModels "github.com/moto-ogs/project-phoenix/modules/dataimport"
	"github.com/moto-ogs/project-phoenix/modules/dataimport/ports"
)

// BatchWriteConfig locks the staff rows a chunk updates, ascending, before
// its first row (#6702). Staff rows are one ascending class of the unit of
// work's lock order and come before every caregiver profile, master data and
// qualification lock. Locked row by row, the second update of a chunk would
// ask for its staff row after the first one's caregiver profile and be
// refused. An upsert takes the staff person merge gate first, which comes
// before the staff rows.
//
// The locked rows replace each update's own read of its staff row, and the
// e-mail matches and misses are resolved in sets and kept for the chunk.
func (c *StaffImportConfig) BatchWriteConfig(ctx context.Context, rows []importModels.StaffImportRow) (ports.ImportConfig[importModels.StaffImportRow], error) {
	c.lockedStaff, c.emailMatches, c.emailAccounts = nil, nil, nil
	mode := importModeFromContext(ctx)
	if c.Membership == nil || (mode != importModels.ImportModeUpdate && mode != importModels.ImportModeUpsert) {
		return c, nil
	}
	c.emailMatches = make(map[string]staffEmailMatch)
	if err := c.resolveBatchEmails(ctx, rows); err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		// A match error is the row's own failure; applyBatchRow reports it.
		if id, err := c.FindExisting(ctx, row); err == nil && id != nil {
			ids = append(ids, *id)
		}
	}
	if len(ids) == 0 {
		return c, nil
	}
	if err := c.lockStaffPersonMerge(ctx); err != nil {
		return nil, err
	}
	staff, err := c.Membership.ListStaff(ctx, ports.StaffFilter{IDs: ids, ForMutation: true})
	if err != nil {
		return nil, copyErrorf("Mitarbeiter sperren: %w", err)
	}
	c.lockedStaff = make(map[int64]ports.Staff, len(staff))
	// Missing rows are the row's own update failure, as before.
	for _, member := range staff {
		c.lockedStaff[member.ID] = member
	}
	return c, nil
}

// lockedStaffRow hands an update the staff row its chunk locked, once: a
// second row of the same person reads the row its predecessor wrote.
func (c *StaffImportConfig) lockedStaffRow(ctx context.Context, staffID int64) (ports.Staff, error) {
	if staff, ok := c.lockedStaff[staffID]; ok {
		delete(c.lockedStaff, staffID)
		return staff, nil
	}
	return c.Membership.FindStaff(ctx, staffID)
}

// staffEmailMatch keeps both clean misses and per-address matching failures.
type staffEmailMatch struct {
	id  int64
	err error
}

// matchedEmail reuses the chunk's email resolution, including clean misses.
func (c *StaffImportConfig) matchedEmail(ctx context.Context, rawEmail string) (*int64, error) {
	email, err := normalizeStaffEmail(rawEmail)
	if err != nil || email == "" {
		return nil, nil
	}
	if match, ok := c.emailMatches[email]; ok {
		if match.id == 0 {
			return nil, match.err
		}
		return &match.id, match.err
	}
	id, err := c.findStaffByLoginEmail(ctx, email)
	if err == nil && c.emailMatches != nil {
		match := staffEmailMatch{}
		if id != nil {
			match.id = *id
		}
		c.emailMatches[email] = match
	}
	return id, err
}

// resolveBatchEmails resolves account, person and live membership sets before
// taking staff rows. Personnel-number hits need no email lookup.
func (c *StaffImportConfig) resolveBatchEmails(ctx context.Context, rows []importModels.StaffImportRow) error {
	if c.FindSchoolEmailIdentities == nil || c.Persons == nil {
		return nil
	}
	emails := c.batchEmailCandidates(rows)
	if len(emails) == 0 {
		return nil
	}
	accounts, invited, err := c.FindSchoolEmailIdentities(ctx, emails)
	if err != nil {
		return err
	}
	c.emailAccounts = accounts
	accountIDs := make([]int64, 0, len(accounts))
	for _, id := range accounts {
		accountIDs = append(accountIDs, id)
	}
	people, err := c.Persons.ListPersonsByAccount(ctx, accountIDs)
	if err != nil {
		return err
	}
	peopleByEmail, personIDs := staffEmailPeople(emails, accounts, invited, people)
	if len(personIDs) == 0 {
		return nil
	}
	staff, err := c.Membership.ListStaff(ctx, ports.StaffFilter{PersonIDs: personIDs, MembershipOnly: true})
	if err != nil {
		return err
	}
	staffByPerson := make(map[int64]int64, len(staff))
	for _, member := range staff {
		if staffByPerson[member.PersonID] == 0 {
			staffByPerson[member.PersonID] = member.ID
		}
	}
	for email, ids := range peopleByEmail {
		c.emailMatches[email] = matchInvitedStaff(email, ids, staffByPerson)
	}
	return nil
}

func (c *StaffImportConfig) batchEmailCandidates(rows []importModels.StaffImportRow) []string {
	emails := make([]string, 0, len(rows))
	for _, row := range rows {
		if _, found := c.staffByPersonnelNumber[strings.ToLower(strings.TrimSpace(row.PersonnelNumber))]; found {
			continue
		}
		email, err := normalizeStaffEmail(row.Email)
		if err != nil || email == "" {
			continue
		}
		if _, found := c.emailMatches[email]; !found {
			emails = append(emails, email)
			c.emailMatches[email] = staffEmailMatch{}
		}
	}
	return emails
}

// staffEmailPeople prefers the same first person as FindPersonByAccount:
// ListPersonsByAccount orders by account, latest update and then lowest ID.
func staffEmailPeople(emails []string, accounts map[string]int64, invited map[string][]int64, people []ports.Person) (map[string][]int64, []int64) {
	personByAccount := make(map[int64]int64, len(people))
	for _, person := range people {
		if person.AccountID != nil && personByAccount[*person.AccountID] == 0 {
			personByAccount[*person.AccountID] = person.ID
		}
	}
	peopleByEmail := make(map[string][]int64, len(emails))
	personIDs := make([]int64, 0, len(people))
	for _, email := range emails {
		ids := invited[email]
		if accountID, found := accounts[email]; found {
			ids = nil // An active account takes precedence even without staff.
			if id := personByAccount[accountID]; id > 0 {
				ids = []int64{id}
			}
		}
		peopleByEmail[email] = ids
		personIDs = append(personIDs, ids...)
	}
	return peopleByEmail, personIDs
}

func matchInvitedStaff(email string, personIDs []int64, staffByPerson map[int64]int64) staffEmailMatch {
	var match staffEmailMatch
	for _, personID := range personIDs {
		id := staffByPerson[personID]
		if id == 0 {
			continue
		}
		if match.id != 0 && match.id != id {
			return staffEmailMatch{err: fmt.Errorf("mehrere offene Einladungen für E-Mail-Adresse '%s' verweisen auf unterschiedliche Personen", email)}
		}
		match.id = id
	}
	return match
}
