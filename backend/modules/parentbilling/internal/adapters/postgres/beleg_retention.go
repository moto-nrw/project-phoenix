package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// adminDB returns the administrative transaction of the hourly job or the
// Operator portal. The retention of the Belege reads and deletes across
// Träger (ADR 0184); a Träger or school transaction is refused.
func adminDB(ctx context.Context) (bun.IDB, error) {
	if !tenant.IsAdminTx(ctx) {
		return nil, errors.New("parent billing postgres: the beleg retention requires the administrative transaction")
	}
	transaction, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return nil, errors.New("parent billing postgres: transaction is required")
	}
	tx, ok := transaction.(bun.Tx)
	if !ok {
		return nil, fmt.Errorf("parent billing postgres: unsupported transaction %T", transaction)
	}
	return tx, nil
}

// belegFactRow is one Beleg of parent-billing with the days of its events.
type belegFactRow struct {
	OrganizationID    int64          `bun:"organization_id"`
	BelegType         string         `bun:"beleg_type"`
	BelegID           int64          `bun:"beleg_id"`
	Issued            *calendar.Date `bun:"issued,type:date"`
	SignedOn          *calendar.Date `bun:"signed_on,type:date"`
	RevokedOn         *calendar.Date `bun:"revoked_on,type:date"`
	LastCollection    *calendar.Date `bun:"last_collection,type:date"`
	CollectionTrigger *calendar.Date `bun:"collection_trigger,type:date"`
}

// BelegFacts lists every Beleg of parent-billing of one Träger or, for
// organizationID 0, of every Träger, with the days of its events: the
// documents and invoices of released runs, an invoice with the later issue
// of itself and its cancellation, the released notices, every mandate with
// the submission of its last debit and the submitted debits. The function
// reads no bank data (migration 1.15.862).
func (s *Store) BelegFacts(ctx context.Context, organizationID int64) ([]parentbilling.BelegFact, error) {
	db, err := adminDB(ctx)
	if err != nil {
		return nil, err
	}
	var rows []belegFactRow
	if err := db.NewRaw(`SELECT * FROM parent_billing.beleg_retention_facts(?)`, organizationID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("parent billing postgres: list beleg facts: %w", err)
	}
	facts := make([]parentbilling.BelegFact, len(rows))
	for i, row := range rows {
		facts[i] = parentbilling.BelegFact{OrganizationID: row.OrganizationID, BelegType: row.BelegType, BelegID: row.BelegID,
			Issued: row.Issued, SignedOn: row.SignedOn, RevokedOn: row.RevokedOn, LastCollection: row.LastCollection,
			CollectionTrigger: row.CollectionTrigger}
	}
	return facts, nil
}

// BelegEvaluationDue reports whether today's evaluation is still open: no
// record yet, or a record from an earlier day. A Beleg issued after today's
// evaluation gets its record the next day.
func (s *Store) BelegEvaluationDue(ctx context.Context, today calendar.Date) (bool, error) {
	db, err := adminDB(ctx)
	if err != nil {
		return false, err
	}
	var due bool
	err = db.NewRaw(`SELECT NOT EXISTS (SELECT 1 FROM parent_billing.beleg_retentions)
		OR EXISTS (SELECT 1 FROM parent_billing.beleg_retentions WHERE evaluated_on < ?)`, today).Scan(ctx, &due)
	return due, err
}

type instructionRow struct {
	bun.BaseModel `bun:"table:parent_billing.beleg_retention_instructions"`

	ID                   int64         `bun:"id,pk,autoincrement"`
	OrganizationID       int64         `bun:"organization_id"`
	BelegType            string        `bun:"beleg_type"`
	TriggerEvent         string        `bun:"trigger_event"`
	Period               int           `bun:"period"`
	Basis                string        `bun:"basis"`
	LegalBasis           string        `bun:"legal_basis"`
	BasisJustification   *string       `bun:"basis_justification"`
	InstructedBy         string        `bun:"instructed_by"`
	InstructedOn         calendar.Date `bun:"instructed_on,type:date"`
	InstructionReference string        `bun:"instruction_reference"`
	Confirmed            bool          `bun:"statutes_not_applicable_confirmed"`
	ConfirmedByOperator  *int64        `bun:"confirmed_by_operator_id"`
	ConfirmedAt          *time.Time    `bun:"confirmed_at"`
	RecordedByOperatorID *int64        `bun:"recorded_by_operator_id"`
	RecordedAt           time.Time     `bun:"recorded_at,nullzero,default:current_timestamp"`
}

func (r instructionRow) instruction() parentbilling.RetentionInstruction {
	instruction := parentbilling.RetentionInstruction{
		ID: r.ID, OrganizationID: r.OrganizationID, BelegType: r.BelegType, TriggerEvent: r.TriggerEvent, Period: r.Period,
		Basis: r.Basis, LegalBasis: r.LegalBasis, InstructedBy: r.InstructedBy, InstructedOn: r.InstructedOn,
		InstructionReference: r.InstructionReference, StatutesNotApplicableConfirmed: r.Confirmed,
		ConfirmedAt: r.ConfirmedAt, RecordedAt: r.RecordedAt,
	}
	if r.BasisJustification != nil {
		instruction.BasisJustification = *r.BasisJustification
	}
	return instruction
}

type suspensionRow struct {
	bun.BaseModel `bun:"table:parent_billing.beleg_retention_suspensions"`

	ID                  int64         `bun:"id,pk,autoincrement"`
	OrganizationID      int64         `bun:"organization_id"`
	BelegType           string        `bun:"beleg_type"`
	BelegID             *int64        `bun:"beleg_id"`
	Reason              string        `bun:"reason"`
	Scope               string        `bun:"scope"`
	Responsible         string        `bun:"responsible"`
	ReviewOn            calendar.Date `bun:"review_on,type:date"`
	CreatedByOperatorID *int64        `bun:"created_by_operator_id"`
	CreatedAt           time.Time     `bun:"created_at,nullzero,default:current_timestamp"`
}

func (r suspensionRow) suspension() parentbilling.RetentionSuspension {
	suspension := parentbilling.RetentionSuspension{
		ID: r.ID, OrganizationID: r.OrganizationID, BelegType: r.BelegType, Reason: r.Reason, Scope: r.Scope,
		Responsible: r.Responsible, ReviewOn: r.ReviewOn, CreatedAt: r.CreatedAt,
	}
	if r.BelegID != nil {
		suspension.BelegID = *r.BelegID
	}
	return suspension
}

// ActiveInstructions lists the active instructions, of one Träger or, for
// organizationID 0, of every Träger.
func (s *Store) ActiveInstructions(ctx context.Context, organizationID int64) ([]parentbilling.RetentionInstruction, error) {
	db, err := adminDB(ctx)
	if err != nil {
		return nil, err
	}
	var rows []instructionRow
	query := db.NewSelect().Model(&rows).Where("replaced_at IS NULL").OrderExpr("organization_id, beleg_type")
	if organizationID > 0 {
		query = query.Where("organization_id = ?", organizationID)
	}
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("parent billing postgres: list retention instructions: %w", err)
	}
	instructions := make([]parentbilling.RetentionInstruction, len(rows))
	for i, row := range rows {
		instructions[i] = row.instruction()
	}
	return instructions, nil
}

// ActiveSuspensions lists the suspensions not lifted, of one Träger or, for
// organizationID 0, of every Träger.
func (s *Store) ActiveSuspensions(ctx context.Context, organizationID int64) ([]parentbilling.RetentionSuspension, error) {
	db, err := adminDB(ctx)
	if err != nil {
		return nil, err
	}
	var rows []suspensionRow
	query := db.NewSelect().Model(&rows).Where("lifted_at IS NULL").OrderExpr("organization_id, review_on, id")
	if organizationID > 0 {
		query = query.Where("organization_id = ?", organizationID)
	}
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("parent billing postgres: list retention suspensions: %w", err)
	}
	suspensions := make([]parentbilling.RetentionSuspension, len(rows))
	for i, row := range rows {
		suspensions[i] = row.suspension()
	}
	return suspensions, nil
}

// InsertInstruction replaces the active instruction of the type with a new
// one; the replaced one stays as history. The Träger row is locked so two
// operators cannot both leave an active instruction.
func (s *Store) InsertInstruction(ctx context.Context, organizationID, operatorID int64, input parentbilling.NewRetentionInstruction) (parentbilling.RetentionInstruction, error) {
	db, err := adminDB(ctx)
	if err != nil {
		return parentbilling.RetentionInstruction{}, err
	}
	if err := lockTraeger(ctx, db, organizationID); err != nil {
		return parentbilling.RetentionInstruction{}, err
	}
	if _, err := db.NewRaw(`UPDATE parent_billing.beleg_retention_instructions SET replaced_at = CURRENT_TIMESTAMP
		WHERE organization_id = ? AND beleg_type = ? AND replaced_at IS NULL`, organizationID, input.BelegType).Exec(ctx); err != nil {
		return parentbilling.RetentionInstruction{}, fmt.Errorf("parent billing postgres: replace retention instruction: %w", err)
	}
	row := &instructionRow{
		OrganizationID: organizationID, BelegType: input.BelegType, TriggerEvent: input.TriggerEvent, Period: input.Period,
		Basis: input.Basis, LegalBasis: input.LegalBasis, InstructedBy: input.InstructedBy, InstructedOn: input.InstructedOn,
		InstructionReference: input.InstructionReference, RecordedByOperatorID: optionalOperator(operatorID),
		Confirmed: input.StatutesNotApplicableConfirmed,
	}
	if input.BasisJustification != "" {
		row.BasisJustification = &input.BasisJustification
	}
	insert := db.NewInsert().Model(row)
	if row.Confirmed {
		// The confirmation names the operator of the session and the time
		// the database records it.
		row.ConfirmedByOperator = optionalOperator(operatorID)
		insert = insert.Value("confirmed_at", "CURRENT_TIMESTAMP")
	}
	if _, err := insert.Returning("id, recorded_at, confirmed_at").Exec(ctx); err != nil {
		return parentbilling.RetentionInstruction{}, fmt.Errorf("parent billing postgres: insert retention instruction: %w", writeError(err))
	}
	return row.instruction(), nil
}

// InsertSuspension stores a new suspension.
func (s *Store) InsertSuspension(ctx context.Context, organizationID, operatorID int64, input parentbilling.NewRetentionSuspension) (parentbilling.RetentionSuspension, error) {
	db, err := adminDB(ctx)
	if err != nil {
		return parentbilling.RetentionSuspension{}, err
	}
	if err := lockTraeger(ctx, db, organizationID); err != nil {
		return parentbilling.RetentionSuspension{}, err
	}
	row := &suspensionRow{
		OrganizationID: organizationID, BelegType: input.BelegType, Reason: input.Reason, Scope: input.Scope,
		Responsible: input.Responsible, ReviewOn: input.ReviewOn, CreatedByOperatorID: optionalOperator(operatorID),
	}
	if input.BelegID > 0 {
		row.BelegID = &input.BelegID
	}
	if _, err := db.NewInsert().Model(row).Returning("id, created_at").Exec(ctx); err != nil {
		return parentbilling.RetentionSuspension{}, fmt.Errorf("parent billing postgres: insert retention suspension: %w", writeError(err))
	}
	return row.suspension(), nil
}

// LiftSuspension ends an active suspension of the Träger. The records it
// covered move back to their state in the next evaluation, which runs at
// once: the caller evaluates the Träger again.
func (s *Store) LiftSuspension(ctx context.Context, organizationID, suspensionID, operatorID int64) error {
	db, err := adminDB(ctx)
	if err != nil {
		return err
	}
	if err := lockTraeger(ctx, db, organizationID); err != nil {
		return err
	}
	result, err := db.NewRaw(`UPDATE parent_billing.beleg_retention_suspensions
		SET lifted_at = CURRENT_TIMESTAMP, lifted_by_operator_id = ?
		WHERE organization_id = ? AND id = ? AND lifted_at IS NULL`, optionalOperator(operatorID), organizationID, suspensionID).Exec(ctx)
	if err != nil {
		return fmt.Errorf("parent billing postgres: lift retention suspension: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return parentbilling.ErrBelegSuspensionNotFound
	}
	return nil
}

// LockTraeger takes the retention lock of each Träger in ascending order
// (parent_billing.lock_beleg_retention, migration 1.15.862).
func (s *Store) LockTraeger(ctx context.Context, organizationIDs []int64) error {
	db, err := adminDB(ctx)
	if err != nil {
		return err
	}
	return lockTraeger(ctx, db, organizationIDs...)
}

// lockTraeger serializes the instructions, suspensions, reviews, the
// evaluation and the deletion of the Träger: two operators cannot both
// leave an active instruction of a type, and the hourly job decides on
// what an operator committed, never on what it is still recording.
func lockTraeger(ctx context.Context, db bun.IDB, organizationIDs ...int64) error {
	if len(organizationIDs) == 0 {
		return nil
	}
	if _, err := db.NewRaw(`SELECT parent_billing.lock_beleg_retention(?::bigint[])`, pgdialect.Array(organizationIDs)).Exec(ctx); err != nil {
		return fmt.Errorf("parent billing postgres: lock beleg retention: %w", err)
	}
	return nil
}

// citesStatuteConstraint is the citation heuristic of migration 1.15.862,
// the only place that reads the text of a basis.
const citesStatuteConstraint = "chk_beleg_retention_instructions_basis_cites_statute"

// writeError names a Träger that does not exist, which the foreign key of
// the new row refuses, a basis the database's citation heuristic refuses,
// and a row another check constraint refuses.
func writeError(err error) error {
	var pgErr pgdriver.Error
	if errors.As(err, &pgErr) {
		switch pgErr.Field('C') {
		case "23503":
			return parentbilling.ErrBelegTraegerNotFound
		case "23514":
			if pgErr.Field('n') == citesStatuteConstraint {
				return parentbilling.ErrBelegBasisCitesStatute
			}
			return fmt.Errorf("%w: %s", parentbilling.ErrBelegRetentionInvalid, pgErr.Field('n'))
		}
	}
	return err
}

func optionalOperator(operatorID int64) *int64 {
	if operatorID <= 0 {
		return nil
	}
	return &operatorID
}

// saveBelegRecordsSQL upserts the records of one batch. A Beleg that stays
// on the clarification list for the same reason keeps its listing day and
// review; a new or changed reason lists it today with a review 30 days on.
// The collection trigger of a mandate only moves forward.
const saveBelegRecordsSQL = `
INSERT INTO parent_billing.beleg_retentions (organization_id, beleg_type, beleg_id, state, reason, trigger_event,
	trigger_date, deletable_on, proposed_on, instruction_id, suspension_id, listed_on, review_on, responsible, collection_trigger_on, evaluated_on)
SELECT b.organization_id, b.beleg_type, b.beleg_id, b.state, NULLIF(b.reason, ''), NULLIF(b.trigger_event, ''),
	NULLIF(b.trigger_date, '')::date, NULLIF(b.deletable_on, '')::date, NULLIF(b.proposed_on, '')::date,
	NULLIF(b.instruction_id, 0), NULLIF(b.suspension_id, 0),
	CASE WHEN b.state <> 'clarification' THEN NULL
		WHEN old.state = 'clarification' AND COALESCE(old.reason, '') = b.reason THEN old.listed_on
		ELSE ?0::date END,
	CASE WHEN b.state <> 'clarification' THEN NULL
		WHEN old.state = 'clarification' AND COALESCE(old.reason, '') = b.reason THEN old.review_on
		ELSE ?0::date + ?1::int END,
	CASE WHEN b.state = 'clarification' THEN ?2 END,
	CASE WHEN b.beleg_type = 'sepa_mandate' THEN NULLIF(b.collection_trigger, '')::date END,
	?0::date
FROM unnest(?3::bigint[], ?4::text[], ?5::bigint[], ?6::text[], ?7::text[], ?8::text[], ?9::text[], ?10::text[], ?11::text[], ?12::bigint[], ?13::bigint[], ?14::text[])
	AS b(organization_id, beleg_type, beleg_id, state, reason, trigger_event, trigger_date, deletable_on, proposed_on, instruction_id, suspension_id, collection_trigger)
LEFT JOIN parent_billing.beleg_retentions AS old
	ON old.organization_id = b.organization_id AND old.beleg_type = b.beleg_type AND old.beleg_id = b.beleg_id
ON CONFLICT (organization_id, beleg_type, beleg_id) DO UPDATE SET
	state = EXCLUDED.state, reason = EXCLUDED.reason, trigger_event = EXCLUDED.trigger_event, trigger_date = EXCLUDED.trigger_date,
	deletable_on = EXCLUDED.deletable_on, proposed_on = EXCLUDED.proposed_on, instruction_id = EXCLUDED.instruction_id,
	suspension_id = EXCLUDED.suspension_id, listed_on = EXCLUDED.listed_on, review_on = EXCLUDED.review_on,
	responsible = EXCLUDED.responsible, evaluated_on = EXCLUDED.evaluated_on,
	collection_trigger_on = GREATEST(parent_billing.beleg_retentions.collection_trigger_on, EXCLUDED.collection_trigger_on)`

// belegRecordBatch bounds one upsert statement.
const belegRecordBatch = 5000

// SaveBelegRecords writes the records of today's evaluation of one Träger
// or, for organizationID 0, of every Träger, and drops the records of
// Belege that no longer exist.
func (s *Store) SaveBelegRecords(ctx context.Context, organizationID int64, today calendar.Date, records []parentbilling.BelegRecord) error {
	db, err := adminDB(ctx)
	if err != nil {
		return err
	}
	for start := 0; start < len(records); start += belegRecordBatch {
		if err := saveBelegRecordBatch(ctx, db, today, records[start:min(start+belegRecordBatch, len(records))]); err != nil {
			return err
		}
	}
	if _, err := db.NewRaw(`DELETE FROM parent_billing.beleg_retentions WHERE evaluated_on < ?0 AND (?1 = 0 OR organization_id = ?1)`,
		today, organizationID).Exec(ctx); err != nil {
		return fmt.Errorf("parent billing postgres: drop records of gone belege: %w", err)
	}
	return nil
}

func saveBelegRecordBatch(ctx context.Context, db bun.IDB, today calendar.Date, records []parentbilling.BelegRecord) error {
	n := len(records)
	organizations, types, ids := make([]int64, n), make([]string, n), make([]int64, n)
	states, reasons, triggers := make([]string, n), make([]string, n), make([]string, n)
	triggerDates, deletable, proposed := make([]string, n), make([]string, n), make([]string, n)
	instructions, suspensions, collections := make([]int64, n), make([]int64, n), make([]string, n)
	for i, record := range records {
		organizations[i], types[i], ids[i] = record.OrganizationID, record.BelegType, record.BelegID
		states[i], reasons[i], triggers[i] = record.State, record.Reason, record.TriggerEvent
		triggerDates[i], deletable[i], proposed[i] = dateText(record.TriggerDate), dateText(record.DeletableOn), dateText(record.ProposedOn)
		instructions[i], suspensions[i], collections[i] = record.InstructionID, record.SuspensionID, dateText(record.CollectionTrigger)
	}
	_, err := db.NewRaw(saveBelegRecordsSQL, today, parentbilling.ClarificationReviewDays, parentbilling.ClarificationOwner,
		pgdialect.Array(organizations), pgdialect.Array(types), pgdialect.Array(ids), pgdialect.Array(states),
		pgdialect.Array(reasons), pgdialect.Array(triggers), pgdialect.Array(triggerDates), pgdialect.Array(deletable),
		pgdialect.Array(proposed), pgdialect.Array(instructions), pgdialect.Array(suspensions), pgdialect.Array(collections)).Exec(ctx)
	if err != nil {
		return fmt.Errorf("parent billing postgres: save beleg records: %w", err)
	}
	return nil
}

func dateText(day *calendar.Date) string {
	if day == nil {
		return ""
	}
	return day.String()
}
