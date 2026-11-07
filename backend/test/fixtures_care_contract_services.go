package test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// CareContractService is one booked service of a test contract: on Weekday
// ("mon" to "sun") the module ends at EndsAt ("HH:MM", empty for none).
type CareContractService struct {
	Weekday string
	Module  string
	EndsAt  string
}

// CreateTestCareContract stores a care contract of the Träger for the child
// at the school, with one version from effectiveFrom and its services
// (#4683). status is the contract's status; a draft has no sent_at. The rows
// carry no tenant: the caller cleans them up with OwnTraegerCareContracts.
func CreateTestCareContract(tb testing.TB, db *bun.DB, organizationID, schoolID, studentID int64, status string,
	effectiveFrom calendar.Date, services ...CareContractService) int64 {
	tb.Helper()
	ctx := context.Background()
	var contractID, versionID int64
	sentAt := "NOW()"
	if status == "entwurf" {
		sentAt = "NULL"
	}
	require.NoError(tb, db.NewRaw(`INSERT INTO care_contract.contracts (organization_id, school_id, request_child_id,
			student_id, status, sent_at) VALUES (?, ?, ?, ?, ?, `+sentAt+`) RETURNING id`,
		organizationID, schoolID, UniqueTestTenantID(tb), studentID, status).Scan(ctx, &contractID))
	require.NoError(tb, db.NewRaw(`INSERT INTO care_contract.contract_versions (organization_id, contract_id, version_number,
			effective_from, child_first_name, child_last_name, school_name, term_kind, notice_months, notice_to,
			special_termination, special_termination_note, statute_required, statute_title, statute_url, contract_partner)
		VALUES (?, ?, 1, ?, 'Kind', 'Vertrag', 'Schule', 'open_ended', 1, 'month_end', false, '', false, '', '', '')
		RETURNING id`, organizationID, contractID, effectiveFrom).Scan(ctx, &versionID))
	for i, service := range services {
		endsAt := any(nil)
		if service.EndsAt != "" {
			endsAt = service.EndsAt
		}
		_, err := db.ExecContext(ctx, `INSERT INTO care_contract.contract_version_services (organization_id,
				contract_version_id, weekday, care_offering_id, module_name, ends_at) VALUES (?, ?, ?, ?, ?, ?::time)`,
			organizationID, versionID, service.Weekday, int64(i+1), service.Module, endsAt)
		require.NoError(tb, err, fmt.Sprintf("insert care contract service %d", i))
	}
	return contractID
}
