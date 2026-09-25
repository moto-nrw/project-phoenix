package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

func TestRecordCareUsageExportAuditIncludesLayout(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		compact bool
		want    string
	}{
		{name: "detailed", compact: false, want: "detailed"},
		{name: "compact", compact: true, want: "compact"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeExportAccessLog{}
			svc := NewReports(ReportDependencies{
				AccessLog: repo,
				Phases:    &fakeClassRosterPhaseRepo{},
			})
			report := &enrollment.CareUsageReport{Phase: enrollment.CareUsagePhase{ID: 55}}

			err := svc.recordCareUsageExportAudit(context.Background(), report, 42, "admin", "pdf", tc.compact)

			require.NoError(t, err)
			require.Len(t, repo.entries, 1)
			assert.Equal(t, tc.want, repo.entries[0].Metadata["layout"])
		})
	}
}
