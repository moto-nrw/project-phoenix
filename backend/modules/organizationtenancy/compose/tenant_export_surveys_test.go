package compose_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// The school export holds the surveys and their questions, but no
// participation, no answer sheet and no single answer (#4781): those rows
// would group a person's answers, and two exports compared would point to the
// answers of whoever took part in between.
func TestTenantExportLeavesSurveyAnswersOut(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	school := testpkg.Tenant(t)
	account := testpkg.CreateTestAccount(t, db, "survey-export")
	ctx := t.Context()
	var surveyID, questionID int64
	require.NoError(t, db.NewRaw(`INSERT INTO users.surveys (tenant_id, title, audience, anonymous, closes_at, published_at)
		VALUES (?, 'Exportierte Umfrage', 'staff', TRUE, now() + interval '1 day', now()) RETURNING id`, school).Scan(ctx, &surveyID))
	require.NoError(t, db.NewRaw(`INSERT INTO users.survey_questions (tenant_id, survey_id, position, kind, prompt)
		VALUES (?, ?, 0, 'text', 'Exportierte Frage') RETURNING id`, school, surveyID).Scan(ctx, &questionID))
	_, err := db.ExecContext(ctx, `INSERT INTO users.survey_participations (tenant_id, survey_id, account_id) VALUES (?, ?, ?)`, school, surveyID, account.ID)
	require.NoError(t, err)
	var responseID string
	require.NoError(t, db.NewRaw(`INSERT INTO users.survey_responses (tenant_id, survey_id) VALUES (?, ?) RETURNING id::text`, school, surveyID).Scan(ctx, &responseID))
	_, err = db.ExecContext(ctx, `INSERT INTO users.survey_answers (tenant_id, response_id, question_id, text_value) VALUES (?, ?::uuid, ?, 'Geheime Antwort')`, school, responseID, questionID)
	require.NoError(t, err)

	exporter, err := compose.NewTenantExporter(db, organizationtenancy.ExportFingerprints{Bytes: testpkg.ContentFingerprint, Reader: testpkg.ReaderContentFingerprint})
	require.NoError(t, err)
	out := filepath.Join(t.TempDir(), "export")
	require.NoError(t, exporter.Export(ctx, organizationtenancy.TenantExportOptions{SchoolID: school, Output: out, Uploads: t.TempDir()}))
	manifest := readExportManifest(t, out)
	assertExportArtifacts(t, out, manifest)

	surveys, err := os.ReadFile(filepath.Join(out, "users.surveys.jsonl"))
	require.NoError(t, err)
	require.Contains(t, string(surveys), "Exportierte Umfrage")
	questions, err := os.ReadFile(filepath.Join(out, "users.survey_questions.jsonl"))
	require.NoError(t, err)
	require.Contains(t, string(questions), "Exportierte Frage")

	for _, table := range []string{"users.survey_participations", "users.survey_responses", "users.survey_answers"} {
		_, err := os.Stat(filepath.Join(out, table+".jsonl"))
		require.ErrorIs(t, err, os.ErrNotExist, table)
	}
	// Neither the answer nor the answer sheet appears anywhere in the export.
	entries, err := os.ReadDir(out)
	require.NoError(t, err)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(out, entry.Name()))
		require.NoError(t, err)
		require.False(t, strings.Contains(string(data), "Geheime Antwort"), entry.Name())
		require.False(t, strings.Contains(string(data), responseID), entry.Name())
	}
}
