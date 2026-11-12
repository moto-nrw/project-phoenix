package application

import (
	"context"
	"fmt"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// The names of the people who exported or acted (#4693). The protocol, the
// runs' last exports and the history of an invoice mark the reader's own
// steps as "by you" and name a colleague with the name the colleague carries
// at the Träger of the reader's session, never with the address. The names of
// a page or a history come in one call; the reader's own steps need none, so
// a reader who did everything alone reads no name at all.

// nameExporters marks the reader's own entries and names the colleagues'.
func (f *FinanceExports) nameExporters(ctx context.Context, reviewer parentbilling.HolidayReviewer, entries []*parentbilling.ExportProtocolEntry) error {
	accounts := make([]*int64, 0, len(entries))
	for _, entry := range entries {
		accounts = append(accounts, entry.ExportedBy)
	}
	names, err := f.colleagueNames(ctx, reviewer, accounts)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		nameExporter(entry, reviewer, names)
	}
	return nil
}

// nameHistorySteps marks the reader's own steps of an invoice and names the
// colleagues', on the step and on its exported file.
func (f *FinanceExports) nameHistorySteps(ctx context.Context, reviewer parentbilling.HolidayReviewer, steps []parentbilling.InvoiceHistoryEntry) error {
	accounts := make([]*int64, 0, len(steps))
	for _, step := range steps {
		accounts = append(accounts, step.AccountID)
	}
	names, err := f.colleagueNames(ctx, reviewer, accounts)
	if err != nil {
		return err
	}
	for i := range steps {
		steps[i].ByYou = byAccount(steps[i].AccountID, reviewer.AccountID)
		if !steps[i].ByYou && steps[i].AccountID != nil {
			steps[i].AccountName = names[*steps[i].AccountID]
		}
		if steps[i].File != nil {
			nameExporter(steps[i].File, reviewer, names)
		}
	}
	return nil
}

func nameExporter(entry *parentbilling.ExportProtocolEntry, reviewer parentbilling.HolidayReviewer, names map[int64]string) {
	entry.ExportedByYou = byAccount(entry.ExportedBy, reviewer.AccountID)
	if !entry.ExportedByYou && entry.ExportedBy != nil {
		entry.ExportedByName = names[*entry.ExportedBy]
	}
}

// colleagueNames reads the names of the accounts other than the reader at
// the reader's Träger in one call, none for an empty set.
func (f *FinanceExports) colleagueNames(ctx context.Context, reviewer parentbilling.HolidayReviewer, accounts []*int64) (map[int64]string, error) {
	var ids []int64
	for _, account := range accounts {
		if account != nil && *account != reviewer.AccountID && !slices.Contains(ids, *account) {
			ids = append(ids, *account)
		}
	}
	if len(ids) == 0 {
		return map[int64]string{}, nil
	}
	names, err := f.names.TraegerActorNames(ctx, reviewer.OrganizationID, ids)
	if err != nil {
		return nil, fmt.Errorf("parent billing: read the names of the Träger-Büro: %w", err)
	}
	return names, nil
}
