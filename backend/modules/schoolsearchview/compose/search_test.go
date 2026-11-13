package compose_test

import (
	"context"
	"strings"
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/modules/schoolsearchview"
	"github.com/moto-ogs/project-phoenix/modules/schoolsearchview/compose"
	"github.com/moto-ogs/project-phoenix/modules/transactionruntime/protocol"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// answerSearch is Enrollment's answer search as the root binds it; it
// records whether the projection asked.
type answerSearch struct {
	matches []enrollment.StudentAnswerMatch
	asked   int
}

func (a *answerSearch) SearchStudentAnswers(context.Context, string, int) ([]enrollment.StudentAnswerMatch, error) {
	a.asked++
	return a.matches, nil
}

var fullScope = schoolsearchview.Scope{Children: true, ChildDetails: true, Contacts: true, Staff: true, Rooms: true, Groups: true}

func search(t *testing.T, db *bun.DB, tenantID int64, answers *answerSearch, text string, scope schoolsearchview.Scope) schoolsearchview.Result {
	t.Helper()
	if answers == nil {
		answers = &answerSearch{}
	}
	searcher, err := compose.New(answers)
	require.NoError(t, err)
	var result schoolsearchview.Result
	require.NoError(t, testpkg.WithTenantTx(t, context.Background(), db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		result, err = searcher.SearchSchool(ctx, schoolsearchview.Query{Text: text, Scope: scope})
		return err
	}))
	return result
}

func addPhone(t *testing.T, db *bun.DB, tenantID, guardianID int64, number string) {
	t.Helper()
	_, err := db.ExecContext(testpkg.Ctx(t), `INSERT INTO users.guardian_phone_numbers (tenant_id, guardian_profile_id, phone_number, phone_type, is_primary)
		VALUES (?, ?, ?, 'mobile', true)`, tenantID, guardianID, number)
	require.NoError(t, err)
}

func TestPhoneQueryFindsTheFamilyInEveryFormat(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	child := testpkg.CreateTestStudent(t, db, "Mia", "Sonnenberg", "3a")
	mother := testpkg.CreateTestGuardianProfileNamed(t, db, "Sabine", "Sonnenberg", "sabine")
	testpkg.CreateTestStudentGuardianLink(t, db, child.ID, mother.ID, "primary_guardian")
	addPhone(t, db, tenantID, mother.ID, "+49 170 1234567")

	for _, query := range []string{"01701234567", "0170 123 45 67", "0170/1234567", "+491701234567", "0049 170-1234567", "4567"} {
		result := search(t, db, tenantID, nil, query, fullScope)
		require.Len(t, result.Contacts, 1, query)
		contact := result.Contacts[0]
		require.Equal(t, mother.ID, contact.ID, query)
		require.Equal(t, schoolsearchview.MatchPhone, contact.Match.Field)
		require.Equal(t, "+49 170 1234567", contact.Match.Text)
		require.Len(t, contact.Children, 1, "a phone hit shows the child")
		require.Equal(t, child.ID, contact.Children[0].ID)
		require.Equal(t, "3a", contact.Children[0].SchoolClass)
		require.Empty(t, result.Children, "a child has no number of its own")
		require.Empty(t, result.Rooms)
	}
	require.Empty(t, search(t, db, tenantID, nil, "0171 1234567", fullScope).Contacts, "another number finds nobody")
}

func TestWithoutContactAccessTheSearchFindsNoContactData(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	child := testpkg.CreateTestStudent(t, db, "Ole", "Brückner", "2b")
	father := testpkg.CreateTestGuardianProfileNamed(t, db, "Jens", "Brückner", "jens.brueckner")
	testpkg.CreateTestStudentGuardianLink(t, db, child.ID, father.ID, "primary_guardian")
	addPhone(t, db, tenantID, father.ID, "0221 555123")
	answers := &answerSearch{matches: []enrollment.StudentAnswerMatch{{StudentID: child.ID, Label: "Seepferdchen", Answer: "Ja"}}}
	guest := schoolsearchview.Scope{Children: true}

	byPhone := search(t, db, tenantID, answers, "0221 555123", guest)
	require.Equal(t, schoolsearchview.Result{
		Children: []schoolsearchview.Child{}, Contacts: []schoolsearchview.Contact{},
		Staff: []schoolsearchview.Staff{}, Rooms: []schoolsearchview.Room{}, Groups: []schoolsearchview.Group{},
	}, byPhone, "the same empty shape as a number nobody has")
	byEmail := search(t, db, tenantID, answers, "jens.brueckner", guest)
	require.Empty(t, byEmail.Contacts)
	require.Empty(t, byEmail.Children)
	byName := search(t, db, tenantID, answers, "Brückner", guest)
	require.Empty(t, byName.Contacts, "contact names stay behind users:read")
	require.Len(t, byName.Children, 1, "the child's own name is open to students:read")
	require.Empty(t, search(t, db, tenantID, answers, "Seepferdchen", guest).Children, "registration answers stay behind users:read")
	require.Zero(t, answers.asked, "Enrollment is not even asked")
}

func TestChildDetailsFindParentNotesAndRegistrationAnswers(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	noted := testpkg.CreateTestStudent(t, db, "Lina", "Achterberg", "1c")
	swimmer := testpkg.CreateTestStudent(t, db, "Paul", "Zander", "4a")
	_, err := db.ExecContext(testpkg.Ctx(t), `UPDATE users.student_profiles SET extra_info = ? WHERE id = ?`, "Wird freitags von der Oma abgeholt", noted.ID)
	require.NoError(t, err)
	answers := &answerSearch{matches: []enrollment.StudentAnswerMatch{{StudentID: swimmer.ID, Label: "Seepferdchen vorhanden", Answer: "Ja"}}}

	notes := search(t, db, tenantID, answers, "oma", fullScope)
	require.Len(t, notes.Children, 2)
	require.Equal(t, noted.ID, notes.Children[0].ID)
	require.Equal(t, schoolsearchview.Match{Field: schoolsearchview.MatchParentNote, Text: "Wird freitags von der Oma abgeholt"}, notes.Children[0].Match)
	require.Equal(t, swimmer.ID, notes.Children[1].ID, "the answer hit follows the direct hits")
	require.Equal(t, schoolsearchview.Match{Field: schoolsearchview.MatchEnrollmentAnswer, Label: "Seepferdchen vorhanden", Text: "Ja"}, notes.Children[1].Match)

	restricted := search(t, db, tenantID, answers, "oma", schoolsearchview.Scope{Children: true})
	require.Empty(t, restricted.Children, "parent notes stay behind users:read")
}

func TestSearchStaysInsideTheSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	otherTenant, _ := testpkg.CreateTestTenant(t, db)
	own := testpkg.CreateTestStudent(t, db, "Emil", "Quellental", "2a")
	foreign := testpkg.CreateTestStudentForTenant(t, db, otherTenant, "Emma", "Quellental", "2a")
	foreignGuardian := testpkg.CreateTestGuardianProfileForTenant(t, db, otherTenant, "Uwe", "Quellental", "uwe")
	testpkg.CreateTestStudentGuardianLinkForTenant(t, db, otherTenant, foreign.ID, foreignGuardian.ID, "primary_guardian")
	addPhone(t, db, otherTenant, foreignGuardian.ID, "0175 9988776")
	testpkg.CreateTestStaffForTenant(t, db, otherTenant, "Ute", "Quellental")
	testpkg.CreateTestRoomForTenant(t, db, otherTenant, "Quellental-Raum")
	testpkg.CreateTestEducationGroupForTenant(t, db, otherTenant, "Quellental-Gruppe")

	result := search(t, db, tenantID, nil, "Quellental", fullScope)
	require.Len(t, result.Children, 1)
	require.Equal(t, own.ID, result.Children[0].ID)
	require.Empty(t, result.Contacts)
	require.Empty(t, result.Staff)
	require.Empty(t, result.Rooms)
	require.Empty(t, result.Groups)
	require.Empty(t, search(t, db, tenantID, nil, "0175 9988776", fullScope).Contacts)
}

func TestStaffRoomsAndGroupsFollowTheirScope(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	staff := testpkg.CreateTestStaff(t, db, "Greta", "Wiesenthal")
	room := testpkg.CreateTestRoom(t, db, "Wiesenthal-Atelier")
	group := testpkg.CreateTestEducationGroup(t, db, "Wiesenthal-Füchse")

	result := search(t, db, tenantID, nil, "wiesenthal", fullScope)
	require.Equal(t, []schoolsearchview.Staff{{ID: staff.ID, FirstName: "Greta", LastName: "Wiesenthal"}}, result.Staff)
	require.Len(t, result.Rooms, 1)
	require.Equal(t, room.ID, result.Rooms[0].ID)
	require.Len(t, result.Groups, 1)
	require.Equal(t, group.ID, result.Groups[0].ID)
	reversed := search(t, db, tenantID, nil, "Wiesenthal Greta", fullScope)
	require.Len(t, reversed.Staff, 1, "a name matches in either order")
	require.Equal(t, staff.ID, reversed.Staff[0].ID)

	none := search(t, db, tenantID, nil, "wiesenthal", schoolsearchview.Scope{Children: true})
	require.Empty(t, none.Staff)
	require.Empty(t, none.Rooms)
	require.Empty(t, none.Groups)
}

// TestSearchQueryBudget keeps one search flat in the size of the school:
// one statement per kind, one for the children of all contact hits and one
// for the children found through registration answers.
func TestSearchQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	tenantID := testpkg.Tenant(t)
	answers := &answerSearch{}
	addSchool := func(n int) {
		for range n {
			child := testpkg.CreateTestStudent(t, db, "Budget", "Kind", "1a")
			guardian := testpkg.CreateTestGuardianProfileNamed(t, db, "Budget", "Eltern", "budget")
			testpkg.CreateTestStudentGuardianLink(t, db, child.ID, guardian.ID, "primary_guardian")
			testpkg.CreateTestStaff(t, db, "Budget", "Team")
			testpkg.CreateTestRoom(t, db, "Budget-Raum")
			testpkg.CreateTestEducationGroup(t, db, "Budget-Gruppe")
			other := testpkg.CreateTestStudent(t, db, "Antwort", "Kind", "2b")
			answers.matches = append(answers.matches, enrollment.StudentAnswerMatch{StudentID: other.ID, Label: "Budget-AG", Answer: "Ja"})
		}
	}
	counter := testpkg.CaptureQueries(t, db)
	reads := func() []string {
		return counter.Matching(func(sql string) bool {
			return strings.Contains(sql, "users.") || strings.Contains(sql, "facilities.") || strings.Contains(sql, "education.")
		})
	}
	run := func() int {
		t.Helper()
		counter.Reset()
		result := search(t, db, tenantID, answers, "budget", fullScope)
		require.NotEmpty(t, result.Children)
		require.NotEmpty(t, result.Contacts)
		require.NotEmpty(t, result.Contacts[0].Children)
		require.NotEmpty(t, result.Staff)
		require.NotEmpty(t, result.Rooms)
		require.NotEmpty(t, result.Groups)
		return len(reads())
	}
	addSchool(1)
	small := run()
	addSchool(6)
	large := run()
	require.Equal(t, small, large, "the statement count must not grow with the school")
	querybudgets.AssertQueryBudget(t, "modules.schoolsearchview.search", counter.Measurement(reads()))
}

// TestEveryStatementWithTheTypedTextIsMarkedForTheLog keeps names and phone
// numbers out of the slow-query log: bun writes bound values into the logged
// SQL, so each statement that carries the text must carry the marker that
// makes the log drop its literals (ADR 0227).
func TestEveryStatementWithTheTypedTextIsMarkedForTheLog(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	tenantID := testpkg.Tenant(t)
	counter := testpkg.CaptureQueries(t, db)
	for _, text := range []string{"Zyxwortel", "0170 9876543"} {
		counter.Reset()
		search(t, db, tenantID, nil, text, fullScope)
		typed := strings.ToLower(strings.Fields(text)[0])
		carrying := counter.Matching(func(sql string) bool {
			return strings.Contains(strings.ToLower(sql), typed) || strings.Contains(sql, "1709876543")
		})
		require.NotEmpty(t, carrying, text)
		for _, statement := range carrying {
			require.True(t, strings.HasPrefix(statement, protocol.RedactLiteralsInLog), "unmarked statement: %.120s", statement)
		}
	}
}
