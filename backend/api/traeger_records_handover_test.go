// Before the erasure order of a Träger is confirmed, the moto team points the
// Träger to its records subject to retention and records the customer's choice
// (#4720 Slice 3, AVV Ziff. 8.2 and 8.4): the Rückgabe was handed over on a
// day, or the customer chose lawful erasure. The choice never blocks the
// erasure indefinitely; the hint and the choice are in the order and in the
// operator audit, without personal data.
package api

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

func TestErasureOrderRequiresTheRecordsHintAndTheCustomersChoice(t *testing.T) {
	t.Parallel()
	f, _ := newTraegerLifecycleFixture(t, "traeger-rueckgabe")
	path := fmt.Sprintf("/organizations/%d/erasure-order", f.organizationID)
	require.Equal(t, http.StatusOK, f.setActive(t, false).Code)
	base := func(records map[string]any) map[string]any {
		body := map[string]any{"order_reference": "LOESCH-4720", "confirmed_on": "2026-01-02", "erase_not_before": "2099-12-31"}
		for key, value := range records {
			body[key] = value
		}
		return body
	}
	digest := strings.Repeat("ab", 32)

	for name, records := range map[string]map[string]any{
		"hint not shown":                 {"records_choice": "erasure_chosen"},
		"hint not acknowledged":          {"records_notice_acknowledged": false, "records_choice": "erasure_chosen"},
		"no choice":                      {"records_notice_acknowledged": true},
		"unknown choice":                 {"records_notice_acknowledged": true, "records_choice": "archive_at_moto"},
		"handover without a day":         {"records_notice_acknowledged": true, "records_choice": "handed_over"},
		"handover in the future":         {"records_notice_acknowledged": true, "records_choice": "handed_over", "records_handed_over_on": "2099-01-01"},
		"export checksum is no SHA-256":  {"records_notice_acknowledged": true, "records_choice": "handed_over", "records_handed_over_on": "2026-01-02", "records_export_sha256": "Max Mustermann"},
		"erasure carries a handover day": {"records_notice_acknowledged": true, "records_choice": "erasure_chosen", "records_handed_over_on": "2026-01-02"},
		"erasure carries an export":      {"records_notice_acknowledged": true, "records_choice": "erasure_chosen", "records_export_sha256": digest},
	} {
		rec := f.call(t, http.MethodPost, path, base(records))
		assert.Equal(t, http.StatusBadRequest, rec.Code, "%s: %s", name, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"code":"`, name)
	}
	assert.Nil(t, f.openOrder(t, path), "a refused order records nothing")
	assert.Zero(t, f.audited(t, "record_erasure_order"))

	// Handover of the Rückgabe export: day and the COMPLETE checksum of the
	// handed-over export are recorded with the operator.
	rec := f.call(t, http.MethodPost, path, base(map[string]any{
		"records_notice_acknowledged": true, "records_choice": "handed_over",
		"records_handed_over_on": "2026-01-02", "records_export_sha256": digest,
	}))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	recorded := decodeSchoolPortalData[organizationtenancy.TraegerErasureOrder](t, rec.Body.Bytes())
	assert.Equal(t, organizationtenancy.RecordsHandedOver, recorded.RecordsChoice)
	require.NotNil(t, recorded.RecordsHandedOverOn)
	assert.Equal(t, "2026-01-02", recorded.RecordsHandedOverOn.String())
	assert.Equal(t, digest, recorded.RecordsExportSHA256)
	require.NotNil(t, recorded.RecordsNoticeShownAt, "the hint is recorded as shown")
	open := f.openOrder(t, path)
	require.NotNil(t, open)
	assert.Equal(t, organizationtenancy.RecordsHandedOver, open.RecordsChoice, "the open order reports the choice")

	var changes struct {
		Choice     string `bun:"choice"`
		HandedOver string `bun:"handed_over_on"`
		Digest     string `bun:"digest"`
		Notice     bool   `bun:"notice"`
	}
	require.NoError(t, f.db.NewRaw(`SELECT changes->>'records_choice' AS choice, changes->>'records_handed_over_on' AS handed_over_on,
		changes->>'records_export_sha256' AS digest, (changes->>'records_notice_acknowledged')::boolean AS notice
		FROM platform.operator_audit_log WHERE operator_id = ? AND resource_type = 'organization' AND resource_id = ?
		AND action = 'record_erasure_order'`, f.operatorID, f.organizationID).Scan(f.ctx, &changes))
	assert.Equal(t, "handed_over", changes.Choice, "the operator audit records the choice")
	assert.Equal(t, "2026-01-02", changes.HandedOver)
	assert.Equal(t, digest, changes.Digest)
	assert.True(t, changes.Notice)

	// The customer's choice of lawful erasure needs no handover: the hint
	// never blocks the erasure indefinitely.
	require.Equal(t, http.StatusOK, f.call(t, http.MethodDelete, path, nil).Code)
	rec = f.call(t, http.MethodPost, path, base(map[string]any{"records_notice_acknowledged": true, "records_choice": "erasure_chosen"}))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	chosen := decodeSchoolPortalData[organizationtenancy.TraegerErasureOrder](t, rec.Body.Bytes())
	assert.Equal(t, organizationtenancy.RecordsErasureChosen, chosen.RecordsChoice)
	assert.Nil(t, chosen.RecordsHandedOverOn)
	assert.Empty(t, chosen.RecordsExportSHA256)
}

// An order started before the hint existed cannot be withdrawn, so the
// operator records the hint and the customer's choice on it afterwards: once,
// audited, with the same validation as a new order.
func TestStartedErasureOrderRecordsTheHintAndTheChoiceAfterwards(t *testing.T) {
	t.Parallel()
	f, _ := newTraegerLifecycleFixture(t, "traeger-rueckgabe-alt")
	path := fmt.Sprintf("/organizations/%d/erasure-order", f.organizationID)
	require.Equal(t, http.StatusOK, f.setActive(t, false).Code)
	_, err := f.db.ExecContext(f.ctx, `INSERT INTO platform.organization_erasure_orders (organization_id, order_reference, confirmed_on, erase_not_before, recorded_by, started_at)
		VALUES (?, 'ORDER-STARTED', '2026-01-02', '2026-01-03', ?, clock_timestamp())`, f.organizationID, f.operatorID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, f.call(t, http.MethodDelete, path, nil).Code, "a started order is never withdrawn")

	for name, records := range map[string]map[string]any{
		"hint not acknowledged":  {"records_choice": "erasure_chosen"},
		"no choice":              {"records_notice_acknowledged": true},
		"handover without a day": {"records_notice_acknowledged": true, "records_choice": "handed_over"},
	} {
		rec := f.call(t, http.MethodPut, path+"/records", records)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "%s: %s", name, rec.Body.String())
	}
	assert.Zero(t, f.audited(t, "record_erasure_order_records"))

	rec := f.call(t, http.MethodPut, path+"/records", map[string]any{
		"records_notice_acknowledged": true, "records_choice": "handed_over", "records_handed_over_on": "2026-01-02"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	recorded := decodeSchoolPortalData[organizationtenancy.TraegerErasureOrder](t, rec.Body.Bytes())
	assert.Equal(t, organizationtenancy.RecordsHandedOver, recorded.RecordsChoice)
	require.NotNil(t, recorded.RecordsNoticeShownAt)
	require.NotNil(t, recorded.StartedAt, "the order stays started")
	assert.Equal(t, organizationtenancy.RecordsHandedOver, f.openOrder(t, path).RecordsChoice)
	assert.Equal(t, 1, f.audited(t, "record_erasure_order_records"), "the operator audit records it")

	rec = f.call(t, http.MethodPut, path+"/records", map[string]any{"records_notice_acknowledged": true, "records_choice": "erasure_chosen"})
	assert.Equal(t, http.StatusConflict, rec.Code, "a recorded choice is not overwritten: %s", rec.Body.String())
	assert.Equal(t, organizationtenancy.RecordsHandedOver, f.openOrder(t, path).RecordsChoice)
}
