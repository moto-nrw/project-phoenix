package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The review of a monthly run (#5916): the schools a run bills, its invoice
// date and the exclusions of single invoices. An exclusion is an entry of
// the run's trail (care_run_events), which the request role only appends
// to; the erasure of a demo school removes it with the contract it names.

// Trail actions of the review.
const (
	careRunEventExclude     = "exclude"
	careRunEventInclude     = "include"
	careRunEventInvoiceDate = "invoice_date"
)

type careRunSchoolRow struct {
	bun.BaseModel `bun:"table:parent_billing.care_run_schools"`

	OrganizationID int64         `bun:"organization_id,pk"`
	RunID          int64         `bun:"run_id,pk"`
	SchoolID       int64         `bun:"school_id,pk"`
	BillingMonth   calendar.Date `bun:"billing_month,type:date"`
	Open           bool          `bun:"open"`
}

// withRunSchools adds the schools of the run to a select of care_run rows,
// in the same statement.
func withRunSchools(query *bun.SelectQuery) *bun.SelectQuery {
	return query.ColumnExpr("care_run_row.*").ColumnExpr(`ARRAY(SELECT run_school.school_id
		FROM parent_billing.care_run_schools AS run_school
		WHERE run_school.organization_id = care_run_row.organization_id AND run_school.run_id = care_run_row.id
		ORDER BY run_school.school_id) AS school_ids`).ColumnExpr(`COALESCE((SELECT (event.details->>'free_of_charge')::integer
		FROM parent_billing.care_run_events AS event
		WHERE event.organization_id = care_run_row.organization_id AND event.run_id = care_run_row.id AND event.action = 'release'
		ORDER BY event.id DESC LIMIT 1), 0) AS free_of_charge`)
}

// insertCareRunSchools stores the schools of a new run as open. A school
// another open run of the month bills is ErrCareRunOpen.
func insertCareRunSchools(ctx context.Context, db bun.IDB, organizationID, runID int64, month calendar.Date, schoolIDs []int64) error {
	if len(schoolIDs) == 0 {
		return errors.New("insert care run schools: a run bills at least one school")
	}
	rows := make([]careRunSchoolRow, len(schoolIDs))
	for i, schoolID := range schoolIDs {
		rows[i] = careRunSchoolRow{OrganizationID: organizationID, RunID: runID, SchoolID: schoolID, BillingMonth: month, Open: true}
	}
	if _, err := db.NewInsert().Model(&rows).Exec(ctx); err != nil {
		if constraintOf(err) == openCareRunSchoolIndex {
			return parentbilling.ErrCareRunOpen
		}
		return fmt.Errorf("insert care run schools: %w", err)
	}
	return nil
}

// closeCareRunSchools closes the schools of a released run, so a later run
// of the month may bill them again, and names them.
func closeCareRunSchools(ctx context.Context, db bun.IDB, organizationID, runID int64) ([]int64, error) {
	var schools []int64
	if err := db.NewRaw(`UPDATE parent_billing.care_run_schools SET open = false
		WHERE organization_id = ? AND run_id = ? AND open
		RETURNING school_id`, organizationID, runID).Scan(ctx, &schools); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("close care run schools: %w", err)
	}
	return sortedIDs(schools), nil
}

func sortedIDs(ids []int64) []int64 {
	if ids == nil {
		return []int64{}
	}
	slices.Sort(ids)
	return ids
}

// OpenCareRunSchools names per school the open run of the month that bills
// it, so a new run can leave it out.
func (s *Store) OpenCareRunSchools(ctx context.Context, organizationID int64, month calendar.Date) (map[int64]int64, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []careRunSchoolRow
	if err := db.NewSelect().Model(&rows).Where("organization_id = ?", organizationID).
		Where("billing_month = ?", month).Where("open").Scan(ctx); err != nil {
		return nil, fmt.Errorf("read open care run schools: %w", err)
	}
	open := make(map[int64]int64, len(rows))
	for _, row := range rows {
		open[row.SchoolID] = row.RunID
	}
	return open, nil
}

// SetCareRunInvoiceDate sets the invoice date of a calculated run; nil
// takes the day of the release. A released run is ErrBillingRunReleased.
func (s *Store) SetCareRunInvoiceDate(ctx context.Context, organizationID, runID int64, invoiceDate *calendar.Date) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	result, err := db.NewUpdate().TableExpr("parent_billing.care_runs").Set("invoice_date = ?", invoiceDate).
		Where("organization_id = ?", organizationID).Where("id = ?", runID).Where("status = ?", parentbilling.RunCalculated).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("set care run invoice date: %w", err)
	}
	if changed, err := result.RowsAffected(); err != nil || changed == 0 {
		if err != nil {
			return err
		}
		return parentbilling.ErrBillingRunReleased
	}
	return nil
}

// CareRunExclusions reads the invoices of a run left out of its release:
// per invoice (contract and service, #6346) the last exclusion, unless a
// later return took it back. An entry names its services; one from before
// #6346 names both since migration 1.15.854, so it still covers both
// invoices of its contract. The trail is read by ID, allocated after the run
// lock. Transaction-start timestamps do not establish write order when
// transactions overlap.
func (s *Store) CareRunExclusions(ctx context.Context, organizationID, runID int64) (map[parentbilling.CareInvoiceKey]parentbilling.CareExclusion, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		Action         string         `bun:"action"`
		ActorAccountID *int64         `bun:"actor_account_id"`
		OccurredAt     time.Time      `bun:"occurred_at"`
		Details        map[string]any `bun:"details,type:jsonb"`
	}
	if err := db.NewRaw(`SELECT action, actor_account_id, occurred_at, details FROM parent_billing.care_run_events
		WHERE organization_id = ? AND run_id = ? AND action = ANY(?)
		ORDER BY id`, organizationID, runID,
		pgdialect.Array([]string{careRunEventExclude, careRunEventInclude})).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("read care run exclusions: %w", err)
	}
	exclusions := map[parentbilling.CareInvoiceKey]parentbilling.CareExclusion{}
	for _, row := range rows {
		contractID, ok := eventContract(row.Details)
		if !ok {
			continue
		}
		reason, _ := row.Details["reason"].(string)
		for _, service := range eventServices(row.Details) {
			key := parentbilling.CareInvoiceKey{ContractID: contractID, Service: service}
			if row.Action == careRunEventInclude {
				delete(exclusions, key)
				continue
			}
			exclusions[key] = parentbilling.CareExclusion{Reason: reason, AccountID: row.ActorAccountID, ExcludedAt: row.OccurredAt}
		}
	}
	return exclusions, nil
}

// eventServices are the services of the invoices an exclusion or return
// names. An entry without a known service names no invoice.
func eventServices(details map[string]any) []string {
	values, _ := details["services"].([]any)
	services := []string{}
	for _, value := range values {
		if service, ok := value.(string); ok && slices.Contains(parentbilling.ExcludableServices, service) && !slices.Contains(services, service) {
			services = append(services, service)
		}
	}
	return services
}

// eventContract is the one contract an exclusion or return names.
func eventContract(details map[string]any) (int64, bool) {
	ids, ok := details["contract_ids"].([]any)
	if !ok || len(ids) != 1 {
		return 0, false
	}
	id, ok := ids[0].(float64)
	return int64(id), ok && id > 0
}
