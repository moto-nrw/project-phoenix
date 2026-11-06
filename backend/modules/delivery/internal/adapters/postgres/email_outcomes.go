package postgres

import (
	"context"
	"fmt"
	"time"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	"github.com/uptrace/bun"
)

// What became of a school's mails, as the school reads it back (#5934): in the
// caller's tenant transaction, without payload. The address of a mail is compared
// in the list's form, which idx_email_outbox_school_kind_address serves.
const emailOutcomeColumns = `outbox.id, outbox.kind, outbox.related_entity_type, outbox.related_entity_id,
		outbox.status, outbox.delivery_state, COALESCE(outbox.delivery_state_at, outbox.sent_at) AS outcome_at,
		outbox.attempts, outbox.last_error, LOWER(TRIM(outbox.recipient->>'address')) AS address,
		suppression.reason AS address_mark
	FROM platform.email_outbox AS outbox
	LEFT JOIN platform.email_suppressions AS suppression
		ON suppression.tenant_id = outbox.tenant_id
		AND suppression.address = LOWER(TRIM(outbox.recipient->>'address'))`

type emailOutcomeRow struct {
	ID            int64      `bun:"id"`
	Kind          string     `bun:"kind"`
	RelatedType   *string    `bun:"related_entity_type"`
	RelatedID     *int64     `bun:"related_entity_id"`
	Status        string     `bun:"status"`
	DeliveryState *string    `bun:"delivery_state"`
	OutcomeAt     *time.Time `bun:"outcome_at"`
	Attempts      int        `bun:"attempts"`
	LastError     *string    `bun:"last_error"`
	Address       string     `bun:"address"`
	AddressMark   *string    `bun:"address_mark"`
}

func (row emailOutcomeRow) report() deliveryContract.EmailOutcomeReport {
	report := deliveryContract.EmailOutcomeReport{
		MailID: row.ID, Kind: row.Kind, Address: row.Address, OutcomeAt: row.OutcomeAt, Attempts: row.Attempts,
	}
	if row.RelatedType != nil {
		report.RelatedType = *row.RelatedType
	}
	if row.RelatedID != nil {
		report.RelatedID = *row.RelatedID
	}
	if row.AddressMark != nil {
		report.Mark = deliveryContract.EmailAddressMark(*row.AddressMark)
	}
	report.Outcome = deliveryContract.EmailOutcomeOf(row.Status, row.DeliveryState, row.LastError, report.Mark)
	return report
}

// EmailOutcomes reports the mails of the related entities, oldest first.
func (s *Store) EmailOutcomes(ctx context.Context, tenantID int64, query deliveryContract.EmailOutcomeQuery) ([]deliveryContract.EmailOutcomeReport, error) {
	if len(query.RelatedIDs) == 0 {
		return nil, nil
	}
	db, err := s.tenantDB(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	var rows []emailOutcomeRow
	raw := db.NewRaw(`SELECT `+emailOutcomeColumns+`
		WHERE outbox.tenant_id = ? AND outbox.related_entity_type = ? AND outbox.related_entity_id IN (?)
		ORDER BY outbox.id`, tenantID, query.RelatedType, bun.List(query.RelatedIDs))
	if len(query.Kinds) > 0 {
		raw = db.NewRaw(`SELECT `+emailOutcomeColumns+`
			WHERE outbox.tenant_id = ? AND outbox.related_entity_type = ? AND outbox.related_entity_id IN (?)
				AND outbox.kind IN (?)
			ORDER BY outbox.id`, tenantID, query.RelatedType, bun.List(query.RelatedIDs), bun.List(query.Kinds))
	}
	if err := raw.Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("delivery postgres: list email outcomes: %w", err)
	}
	reports := make([]deliveryContract.EmailOutcomeReport, 0, len(rows))
	for _, row := range rows {
		reports = append(reports, row.report())
	}
	return reports, nil
}

// LatestEmailOutcomes reports the newest mail of the kinds to each address.
func (s *Store) LatestEmailOutcomes(ctx context.Context, tenantID int64, kinds, addresses []string) (map[string]deliveryContract.EmailOutcomeReport, error) {
	normalized := normalizedAddresses(addresses)
	if len(normalized) == 0 || len(kinds) == 0 {
		return map[string]deliveryContract.EmailOutcomeReport{}, nil
	}
	db, err := s.tenantDB(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	var rows []emailOutcomeRow
	err = db.NewRaw(`SELECT DISTINCT ON (LOWER(TRIM(outbox.recipient->>'address'))) `+emailOutcomeColumns+`
		WHERE outbox.tenant_id = ? AND outbox.kind IN (?)
			AND LOWER(TRIM(outbox.recipient->>'address')) IN (?)
		ORDER BY LOWER(TRIM(outbox.recipient->>'address')), outbox.id DESC`,
		tenantID, bun.List(kinds), bun.List(normalized)).Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("delivery postgres: read latest email outcomes: %w", err)
	}
	reports := make(map[string]deliveryContract.EmailOutcomeReport, len(rows))
	for _, row := range rows {
		reports[row.Address] = row.report()
	}
	return reports, nil
}

// EmailAddressMarks reports the marked addresses among the given ones.
func (s *Store) EmailAddressMarks(ctx context.Context, tenantID int64, addresses []string) (map[string]deliveryContract.EmailAddressMark, error) {
	normalized := normalizedAddresses(addresses)
	if len(normalized) == 0 {
		return map[string]deliveryContract.EmailAddressMark{}, nil
	}
	db, err := s.tenantDB(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		Address string `bun:"address"`
		Reason  string `bun:"reason"`
	}
	if err := db.NewRaw(`SELECT address, reason FROM platform.email_suppressions WHERE tenant_id = ? AND address IN (?)`,
		tenantID, bun.List(normalized)).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("delivery postgres: read email address marks: %w", err)
	}
	marks := make(map[string]deliveryContract.EmailAddressMark, len(rows))
	for _, row := range rows {
		marks[row.Address] = deliveryContract.EmailAddressMark(row.Reason)
	}
	return marks, nil
}

func normalizedAddresses(addresses []string) []string {
	seen := make(map[string]bool, len(addresses))
	result := make([]string, 0, len(addresses))
	for _, address := range addresses {
		normalized := deliveryContract.NormalizedEmailAddress(address)
		if normalized != "" && !seen[normalized] {
			seen[normalized] = true
			result = append(result, normalized)
		}
	}
	return result
}
