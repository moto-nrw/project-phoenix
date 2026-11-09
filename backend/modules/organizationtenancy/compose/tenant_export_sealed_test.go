package compose_test

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// openedSealedFields stands in for the owner that opens a sealed value. It
// answers a marker of table and column, never the sealed bytes, so the
// fixture's ciphertext marker must not reach the export.
type openedSealedFields struct{}

func (openedSealedFields) OpenExportField(table, column string, row map[string]json.RawMessage, sealed []byte) (string, error) {
	if len(sealed) == 0 || row["fixture_school"] == nil {
		return "", errors.New("sealed value or its row missing")
	}
	return "opened-" + table + "-" + column, nil
}

type refusingSealedFields struct{}

func (refusingSealedFields) OpenExportField(string, string, map[string]json.RawMessage, []byte) (string, error) {
	return "", errors.New("wrong key")
}

// A return without the key rings, with a key that does not open the value,
// or without the owner of one sealed table must not complete: neither
// ciphertext nor a gap goes out (#3838, #6328).
func TestTenantExportRefusesSealedValuesItCannotOpen(t *testing.T) {
	t.Parallel()
	db, catalog, a, b := exportFixture(t)
	uploads := fixtureUploads(t, db, catalog, a, b)
	bankOnly := organizationtenancy.TenantExportSealedFieldsByTable{
		"parent_billing.sepa_mandates": openedSealedFields{}, "parent_billing.sepa_debits": openedSealedFields{},
	}
	for name, tt := range map[string]struct {
		sealed organizationtenancy.TenantExportSealedFields
		names  string
	}{
		"without keys":         {nil, `(parent_billing\.sepa_|enrollment\.holiday_|users\.student_care_profiles)`},
		"wrong key":            {refusingSealedFields{}, `(parent_billing\.sepa_|enrollment\.holiday_|users\.student_care_profiles)`},
		"without health owner": {bankOnly, `cannot open a sealed value of (enrollment\.holiday_|users\.student_care_profiles)`},
	} {
		exporter, err := compose.NewTenantExporter(db, organizationtenancy.ExportFingerprints{Bytes: testpkg.ContentFingerprint, Reader: testpkg.ReaderContentFingerprint}, tt.sealed)
		require.NoError(t, err)
		out := filepath.Join(t.TempDir(), "export")
		err = exporter.Export(t.Context(), organizationtenancy.TenantExportOptions{SchoolID: a, Output: out, Uploads: uploads})
		require.Error(t, err, name)
		require.Regexp(t, tt.names, err.Error(), name)
		require.NoFileExists(t, filepath.Join(out, "COMPLETE"), name)
	}
}
