package tenantexport

import (
	"encoding/json"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/stretchr/testify/require"
)

// Every column that points at a person is either a handling column by its
// name or listed here as a column about that person. A new person column
// fails until somebody decides which of the two it is: a subject column
// brings the row's dependent records into a person export, a handling
// column brings the one row only (#3838).
func TestPersonColumnsAreSubjectOrHandling(t *testing.T) {
	t.Parallel()
	subject := []string{
		// The parent account of a holiday registration is the family the
		// registration is about (ADR 0216).
		// The author of a letter, thread, notice or survey made a statement.
		"users.parent_announcements.created_by", "users.parent_group_threads.created_by_account_id",
		"users.staff_message_threads.created_by_account_id", "users.staff_notices.created_by", "users.surveys.created_by",
		"accepted_account_id", "account_id", "author_account_id", "created_student_id", "guardian_account_id",
		"guardian_profile_id", "last_sender_account_id", "matched_student_id", "membership_id", "organizer_account_id",
		"organizer_staff_id", "parent_account_id", "person_id", "regular_staff_id", "related_staff_id",
		"sender_account_id", "signer_account_id", "staff_id", "student_high_id", "student_id", "student_low_id",
		"student_profile_id", "subject_staff_id", "substitute_staff_id", "target_account_id", "target_staff_id", "user_id",
	}
	var catalog organizationtenancy.DataLifecycleCatalog
	data, err := organizationtenancy.DataLifecycleCatalogSource.ReadFile("data_lifecycle_catalog.json")
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &catalog))
	var register organizationtenancy.ReturnRegister
	data, err = organizationtenancy.ReturnRegisterSource.ReadFile(organizationtenancy.ReturnRegisterFile)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &register))

	found := map[string]bool{}
	handling := 0
	for _, table := range register.Tables {
		for _, rel := range personRelationships(table.ExportModelTable, catalog) {
			switch {
			case !personIdentity(rel.TargetTable):
			case personActorColumn(table.Name, rel.Column):
				handling++
			case personActorColumn("", rel.Column):
				found[table.Name+"."+rel.Column] = true
			default:
				found[rel.Column] = true
			}
		}
	}
	var got []string
	for column := range found {
		got = append(got, column)
	}
	require.ElementsMatch(t, subject, got)
	require.Positive(t, handling)
	for _, column := range []string{"created_by", "reviewed_by_account_id", "actor_account_id", "recorded_by_staff_id"} {
		require.True(t, personActorColumn("activities.groups", column), column)
	}
	require.False(t, personActorColumn("documents.files", "size_bytes"))
}
