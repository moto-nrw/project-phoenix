package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Every read of the finance exports names the Träger in each statement and
// subquery, also below row level security (brief rule 25, review of
// 9d0709ae69: a Träger filter of the invoice read could go without a test
// turning red). The other Träger holds rows that point at this Träger's
// runs, invoices and files, as only a broken write could leave them
// (foreign keys off). Removing one Träger filter lets such a row into a
// count, a step, an actor, a label or a file and turns an assertion red.
//
// Twelve filters come in six pairs joined on organization_id: invoice and
// run of an issue step (monthly and holiday), mailing and letter of a
// monthly mailing, mailing and Storno of a holiday Storno mailing, and
// cancellation and invoice of a monthly Storno (overview and content).
// Removing one of a pair cannot change a result, the other and the join
// still bind the row to the Träger; both stay as a second line.
func TestFinanceExportReadsKeepTheTraegerBelowRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	own := newMandateScope(t, db, "Träger Lesen A")
	other := newMandateScope(t, db, "Träger Lesen B")
	school := testpkg.CreateTestSchoolInOrganization(t, db, own.organization, "OGS Lesen A")
	store := NewStore()
	rollback := errors.New("roll the reads back")
	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, own.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, own.organization)
		require.NoError(t, err)
		care := seedDebitScope(t, ctx, tx, own, school, "DE98ZZZ09999999999", "SJR-LA-1")
		var invoice int64
		require.NoError(t, tx.NewRaw(`SELECT care_invoice_id FROM parent_billing.sepa_debits WHERE id = ?`, care.debit).Scan(ctx, &invoice))
		holiday, document := seedHolidayExportRun(t, ctx, tx, own.organization, "Herbst A", 1000, true)
		var cancellation int64
		var cancellationNumber string
		require.NoError(t, tx.NewRaw(`SELECT id, number FROM parent_billing.documents WHERE reference_document_id = ?`, document).
			Scan(ctx, &cancellation, &cancellationNumber))
		require.NoError(t, store.StartExportFileNumbers(ctx, own.organization, 100))
		ownFile := recordSollstellung(t, ctx, store, own.organization, parentbilling.DebitRunCare, care.run, 1, 7750,
			[]int64{invoice}, []int64{document})

		_, err = tx.ExecContext(ctx, `SET LOCAL session_replication_role = replica`)
		require.NoError(t, err)
		var otherCareRun, otherHolidayRun, foreignFile, strayFile, careLabelFile, holidayLabelFile int64
		exec := func(query string, args ...any) {
			t.Helper()
			_, err := tx.NewRaw(query, args...).Exec(ctx)
			require.NoError(t, err, query)
		}
		scan := func(dest *int64, query string, args ...any) {
			t.Helper()
			require.NoError(t, tx.NewRaw(query, args...).Scan(ctx, dest), query)
		}
		// This Träger's own release and Storno, with their actors.
		exec(`INSERT INTO parent_billing.care_run_events (organization_id, run_id, action, actor_account_id, occurred_at)
			VALUES (?, ?, 'release', 111, NOW() - INTERVAL '1 hour')`, own.organization, care.run)
		exec(`INSERT INTO parent_billing.run_events (organization_id, run_id, action, actor_account_id, occurred_at)
			VALUES (?, ?, 'release', 111, NOW() - INTERVAL '1 hour')`, own.organization, holiday)
		exec(`INSERT INTO parent_billing.run_events (organization_id, run_id, action, actor_account_id, occurred_at, details)
			VALUES (?, ?, 'cancel', 112, NOW() - INTERVAL '1 hour', jsonb_build_object('document_id', ?::bigint))`,
			own.organization, holiday, cancellation)
		// The other Träger's runs, one holding an invoice of this Träger.
		scan(&otherCareRun, `INSERT INTO parent_billing.care_runs (organization_id, billing_month, status, released_at, issued_on,
			document_count, total_cents, month_rule) VALUES (?, DATE '2029-07-01', 'released', NOW(), DATE '2029-06-20', 1, 1, 'pro_rata')
			RETURNING id`, other.organization)
		scan(&otherHolidayRun, `INSERT INTO parent_billing.runs (organization_id, service, holiday_period_id, period_name, service_from,
			service_until, status, released_at, issued_on, document_count, total_cents)
			VALUES (?, 'holiday', 1, 'Fremd', DATE '2029-10-01', DATE '2029-10-05', 'released', NOW(), DATE '2029-09-20', 1, 1)
			RETURNING id`, other.organization)
		copyInvoice := `INSERT INTO parent_billing.care_invoices (organization_id, run_id, number, number_year, number_sequence, issued_on,
			billing_month, care_contract_id, school_id, school_name, child_first_name, child_last_name, recipient_first_name,
			recipient_last_name, recipient_email, month_rule, total_cents)
			SELECT ?, ?, ?, number_year, number_sequence + 500, issued_on, billing_month, care_contract_id + 100000, school_id, school_name,
				'Fremd', 'Kind', recipient_first_name, recipient_last_name, recipient_email, month_rule, 1111
			FROM parent_billing.care_invoices WHERE id = ?`
		exec(copyInvoice, own.organization, otherCareRun, "RE-IN-FREMD", invoice)
		exec(copyInvoice, other.organization, care.run, "RE-FREMD", invoice)
		// The other Träger's rows on this Träger's monthly invoice.
		exec(`INSERT INTO parent_billing.care_run_events (organization_id, run_id, action, actor_account_id, occurred_at)
			VALUES (?, ?, 'release', 999, NOW() + INTERVAL '1 hour')`, other.organization, care.run)
		exec(`INSERT INTO parent_billing.care_invoice_positions (organization_id, invoice_id, position, care_offering_id, module_name,
			contract_version_number, service_from, service_until, weekdays, amount_cents, net_cents)
			VALUES (?, ?, 1, 1, 'Fremd', 1, DATE '2020-01-01', DATE '2031-12-31', '[{"weekday":"mon"}]'::jsonb, 1, 1)`,
			other.organization, invoice)
		exec(`INSERT INTO parent_billing.care_meal_positions (organization_id, invoice_id, position, school_id, student_id, meal_month,
			mode, quantity, unit_cents, amount_cents, net_cents)
			VALUES (?, ?, 1, ?, 1, DATE '2019-01-01', 'per_meal', 1, 1, 1, 1)`, other.organization, invoice, school)
		exec(`INSERT INTO parent_billing.care_invoice_cancellations (organization_id, run_id, invoice_id, number, number_year,
			number_sequence, issued_on, reason, total_cents) VALUES (?, ?, ?, 'ST-FREMD', 2029, 900, DATE '2029-06-20', 'Fremd', -1)`,
			other.organization, care.run, invoice)
		copyDebit := `INSERT INTO parent_billing.sepa_debits (organization_id, mandate_id, care_contract_id, run_kind, run_id,
			care_invoice_id, document_id, invoice_number, amount_cents, due_on, sequence_type, creditor_id, mandate_reference,
			mandate_signed_on, account_holder_sealed, iban_sealed, status, message_id, submitted_at, submitted_amount_cents, collect_on)
			SELECT ?, mandate_id, care_contract_id, ?, ?, ?, ?, invoice_number, amount_cents, due_on, sequence_type, creditor_id,
				mandate_reference, mandate_signed_on, account_holder_sealed, iban_sealed, 'submitted', 'MSG-FREMD', NOW(), amount_cents, due_on
			FROM parent_billing.sepa_debits WHERE id = ?`
		exec(copyDebit, other.organization, parentbilling.DebitRunCare, care.run, invoice, nil, care.debit)
		// The other Träger's rows on this Träger's holiday invoice.
		exec(`INSERT INTO parent_billing.run_events (organization_id, run_id, action, actor_account_id, occurred_at, details)
			VALUES (?0, ?1, 'release', 999, NOW() + INTERVAL '1 hour', '{}'),
				(?0, ?1, 'cancel', 999, NOW() + INTERVAL '1 hour', jsonb_build_object('document_id', ?2::bigint))`,
			other.organization, holiday, cancellation)
		exec(`INSERT INTO parent_billing.document_positions (organization_id, document_id, position, holiday_registration_child_id,
			child_first_name, child_last_name, kind, amount_cents, net_cents) VALUES (?, ?, 1, 1, 'Fremd', 'Kind', 'base', 1, 1)`,
			other.organization, document)
		exec(`INSERT INTO parent_billing.document_mailings (organization_id, run_id, document_id, delivery_id, recipient_email)
			VALUES (?, ?, ?, 4242, 'fremd@example.org')`, other.organization, holiday, document)
		exec(`INSERT INTO parent_billing.documents (organization_id, run_id, service, number, number_year, number_sequence, issued_on,
			holiday_period_id, holiday_registration_id, recipient_first_name, recipient_last_name, recipient_email, service_from,
			service_until, total_cents, kind, reference_document_id, reason)
			SELECT ?, run_id, service, 'FS-FREMD', number_year, number_sequence + 900, issued_on, holiday_period_id,
				holiday_registration_id + 900, recipient_first_name, recipient_last_name, recipient_email, service_from, service_until,
				-1, 'credit', id, 'Fremd'
			FROM parent_billing.documents WHERE id = ?`, other.organization, document)
		exec(`UPDATE parent_billing.documents SET kind = 'cancellation', total_cents = -1
			WHERE organization_id = ? AND number = 'FS-FREMD'`, other.organization)
		exec(copyDebit, other.organization, parentbilling.DebitRunHoliday, holiday, nil, document, care.debit)
		// Files: one of the other Träger on this Träger's run and invoices,
		// earlier with the same content and later; one of this Träger linked
		// only by the other Träger; two of this Träger on the other Träger's
		// runs, whose labels are the other Träger's.
		scan(&foreignFile, `INSERT INTO parent_billing.export_files (organization_id, file_number, format, run_kind, run_id, file_name,
			content_sha256, invoice_count, total_cents) VALUES (?, 5, 'sollstellung', 'care', ?, 'fremd-frueh.csv', ?, 1, 1) RETURNING id`,
			other.organization, care.run, ownFile.Fingerprint)
		exec(`INSERT INTO parent_billing.export_files (organization_id, file_number, format, run_kind, run_id, file_name,
			content_sha256, invoice_count, total_cents) VALUES (?, 900, 'sollstellung', 'care', ?, 'fremd-spaet.csv', repeat('c', 64), 1, 1)`,
			other.organization, care.run)
		scan(&strayFile, `INSERT INTO parent_billing.export_files (organization_id, file_number, format, run_kind, run_id, file_name,
			content_sha256, invoice_count, total_cents) VALUES (?, 801, 'sollstellung', 'holiday', ?, 'eigen-fremd-verknuepft.csv', repeat('d', 64),
			1, 1) RETURNING id`, own.organization, holiday)
		scan(&careLabelFile, `INSERT INTO parent_billing.export_files (organization_id, file_number, format, run_kind, run_id, file_name,
			content_sha256, invoice_count, total_cents) VALUES (?, 802, 'sollstellung', 'care', ?, 'eigen-monat.csv', repeat('e', 64), 1, 1)
			RETURNING id`, own.organization, otherCareRun)
		scan(&holidayLabelFile, `INSERT INTO parent_billing.export_files (organization_id, file_number, format, run_kind, run_id,
			file_name, content_sha256, invoice_count, total_cents) VALUES (?, 803, 'sollstellung', 'holiday', ?, 'eigen-ferien.csv',
			repeat('f', 64), 1, 1) RETURNING id`, own.organization, otherHolidayRun)
		exec(`INSERT INTO parent_billing.export_file_invoices (organization_id, export_file_id, care_invoice_id, document_id) VALUES
			(?0, ?2, ?4, NULL), (?0, ?2, NULL, ?5),
			(?1, ?3, ?4, NULL), (?1, ?3, NULL, ?5),
			(?0, ?6, ?4, NULL), (?0, ?6, NULL, ?5), (?0, ?7, ?4, NULL), (?0, ?7, NULL, ?5)`,
			own.organization, other.organization, foreignFile, strayFile, invoice, document, careLabelFile, holidayLabelFile)
		exec(`INSERT INTO parent_billing.export_mappings (organization_id, format, settings) VALUES (?, 'sollstellung', '{"fremd":true}')`,
			other.organization)
		_, err = tx.ExecContext(ctx, `SET LOCAL session_replication_role = origin`)
		require.NoError(t, err)

		assertOwnRuns(t, ctx, store, own.organization, care.run, holiday, ownFile)
		assertOwnContent(t, ctx, store, own.organization, care.run, holiday, invoice, document, cancellation)
		for _, history := range []struct {
			kind string
			id   int64
		}{{parentbilling.HistoryCareInvoice, invoice}, {parentbilling.HistoryHolidayDocument, document}} {
			assertOwnHistory(t, ctx, store, own.organization, history.kind, history.id, cancellationNumber,
				ownFile.ID, []int64{careLabelFile, holidayLabelFile})
		}

		mapping, _, err := store.ExportMapping(ctx, own.organization, parentbilling.ExportFormatSollstellung)
		require.NoError(t, err)
		assert.Nil(t, mapping, "ExportMapping reads no mapping of the other Träger")
		require.NoError(t, store.DeleteExportMapping(ctx, own.organization, parentbilling.ExportFormatSollstellung))
		var kept int
		require.NoError(t, tx.NewRaw(`SELECT COUNT(*) FROM parent_billing.export_mappings WHERE organization_id = ?`, other.organization).
			Scan(ctx, &kept))
		assert.Equal(t, 1, kept, "DeleteExportMapping leaves the other Träger's mapping")
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}

// assertOwnRuns checks the overview: only the own runs, with the own
// documents and the own last file.
func assertOwnRuns(t *testing.T, ctx context.Context, store *Store, organization, careRun, holidayRun int64,
	ownFile parentbilling.ExportProtocolEntry,
) {
	t.Helper()
	page, err := store.ExportRuns(ctx, organization, 0, parentbilling.ExportPageSize)
	require.NoError(t, err)
	runs := map[string]parentbilling.ExportRun{}
	for _, run := range page.Runs {
		runs[run.RunKind] = run
	}
	assert.Len(t, runs, 2, "only the own monthly and holiday run, not the other Träger's run with an own invoice")
	assert.Equal(t, 2, page.Total)
	monthly := runs[parentbilling.DebitRunCare]
	assert.Equal(t, careRun, monthly.RunID)
	assert.Equal(t, holidayRun, runs[parentbilling.DebitRunHoliday].RunID)
	assert.Equal(t, 1, monthly.InvoiceCount, "the other Träger's invoice and Storno in the run are not counted")
	assert.EqualValues(t, 7750, monthly.TotalCents)
	require.NotNil(t, monthly.LastSollstellung)
	assert.Equal(t, ownFile.ID, monthly.LastSollstellung.ID, "the last Sollstellung is the own one, not the other Träger's later file")
	assert.Equal(t, 1, monthly.Exports, "the own Sollstellung, not the other Träger's files of the run")
	assert.Equal(t, 2, runs[parentbilling.DebitRunHoliday].InvoiceCount, "the own invoice and Storno, not the other Träger's Storno")
	assert.Zero(t, runs[parentbilling.DebitRunHoliday].TotalCents)
}

// assertOwnContent checks what a Sollstellung writes: the own documents
// with their own positions.
func assertOwnContent(t *testing.T, ctx context.Context, store *Store, organization, careRun, holidayRun, invoice, document, cancellation int64) {
	t.Helper()
	content, err := store.ExportRunContent(ctx, organization, parentbilling.DebitRunCare, careRun)
	require.NoError(t, err)
	require.Len(t, content.Invoices, 1, "no invoice or Storno of the other Träger in the own run")
	assert.Equal(t, invoice, content.Invoices[0].CareInvoiceID)
	assert.Equal(t, "2029-06-01", content.Invoices[0].ServiceFrom.String(), "the other Träger's positions do not widen the period")
	assert.Equal(t, "2029-06-30", content.Invoices[0].ServiceUntil.String())
	content, err = store.ExportRunContent(ctx, organization, parentbilling.DebitRunHoliday, holidayRun)
	require.NoError(t, err)
	ids := []int64{}
	for _, line := range content.Invoices {
		ids = append(ids, line.DocumentID)
		assert.Empty(t, line.ChildName, "no child of the other Träger's positions")
	}
	assert.ElementsMatch(t, []int64{document, cancellation}, ids)
}

// assertOwnHistory checks an invoice's history: the own actors and steps,
// the own files with the own labels.
func assertOwnHistory(t *testing.T, ctx context.Context, store *Store, organization int64, kind string, id int64,
	cancellationNumber string, ownFile int64, foreignRunFiles []int64,
) {
	t.Helper()
	_, steps, exports, err := store.InvoiceHistory(ctx, organization, kind, id)
	require.NoError(t, err, kind)
	for _, step := range steps {
		switch step.Kind {
		case parentbilling.HistoryIssued:
			require.NotNil(t, step.AccountID, kind)
			assert.EqualValues(t, 111, *step.AccountID, "%s: the own release, not the other Träger's later one", kind)
			assert.Zero(t, step.Positions, "%s: no position of the other Träger", kind)
		case parentbilling.HistoryCancelled:
			require.Equal(t, parentbilling.HistoryHolidayDocument, kind, "the monthly invoice has no own Storno")
			assert.Equal(t, cancellationNumber, step.Note)
			require.NotNil(t, step.AccountID)
			assert.EqualValues(t, 112, *step.AccountID, "the own cancel event, not the other Träger's later one")
		default:
			t.Errorf("%s: step %q of the other Träger", kind, step.Kind)
		}
	}
	labels := map[int64]string{}
	for _, file := range exports {
		labels[file.ID] = file.RunLabel
		assert.Zero(t, file.RepeatOf, "%s: the other Träger's earlier file with the same content is no earlier file", kind)
	}
	ids := []int64{}
	for file := range labels {
		ids = append(ids, file)
	}
	assert.ElementsMatch(t, append([]int64{ownFile}, foreignRunFiles...), ids, "%s: only the own files linked by the own Träger", kind)
	assert.NotEmpty(t, labels[ownFile], kind)
	for _, file := range foreignRunFiles {
		assert.NotContains(t, labels[file], "Juli 2029", "%s: no month of the other Träger's run", kind)
		assert.NotContains(t, labels[file], "Fremd", "%s: no period of the other Träger's run", kind)
	}
}
