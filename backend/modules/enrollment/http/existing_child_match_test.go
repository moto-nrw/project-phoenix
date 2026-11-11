package enrollmenthttp_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// An enrolled child submitted again in an open or linked_parents phase must
// not become a second child at approval (#6053). These tests drive the real
// submit, detail read and decision against the test database.

var existingChildBirthday = calendar.NewDate(2018, 4, 15)

// existingChildEnv is the decision env with a request service that matches
// enrolled children: the People Directory's students and the guardian
// relationships, as the production root binds them.
type existingChildEnv struct {
	*decisionTestEnv
	matching enrollmentAPI.RequestService
}

func setupExistingChildTest(t *testing.T, audience string) *existingChildEnv {
	t.Helper()
	env := setupDecisionTest(t)
	matching := newTestRequestService(owners.EnrollmentIntakeSources{
		Bookings:           requestTestBookingCommands(),
		Requests:           env.repos.Enrollment(),
		Children:           env.repos.Enrollment(),
		CareOfferingRepo:   enrollmentCompose.NewCareOfferingRecords(env.reposCare.CarePlan),
		Catalog:            env.repos.Enrollment(),
		SchoolRepo:         factorySchools{repos: env.reposSchools},
		RateLimitRepo:      env.repos.Enrollment(),
		LateInviteRepo:     env.repos.Enrollment(),
		StudentRepo:        env.repos.Student,
		GuardianAuthorizer: env.repos.StudentGuardian,
		GuardianHold:       testGuardianHold(env.db, env.repos),
		OutboxEnqueuer:     env.outbox,
		Settings:           env.settings,
		FrontendURL:        "http://localhost:3000",
		Logger:             slog.Default(),
	})
	_, err := env.db.NewUpdate().TableExpr(`enrollment.phases`).
		Set("audience = ?", audience).
		Where("id = ?", env.sourcePhase.ID).
		Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	return &existingChildEnv{decisionTestEnv: env, matching: matching}
}

// enrolledChild creates an active child of the test's school born on
// birthday.
func enrolledChild(t *testing.T, db *bun.DB, firstName, lastName string, birthday calendar.Date) *testpkg.Student {
	t.Helper()
	student := testpkg.CreateTestStudent(t, db, firstName, lastName, "1a")
	setPersonBirthday(t, db, student.PersonID, birthday)
	return student
}

func setPersonBirthday(t *testing.T, db *bun.DB, personID int64, birthday calendar.Date) {
	t.Helper()
	_, err := db.NewUpdate().TableExpr(`users.persons`).
		Set("birthday = ?", birthday).
		Where("id = ?", personID).
		Exec(testpkg.Ctx(t))
	require.NoError(t, err)
}

// parentAccountLinkedTo creates a parent account whose guardian profile is
// linked to the student with the given role preset.
func parentAccountLinkedTo(t *testing.T, db *bun.DB, studentID int64, role string) int64 {
	t.Helper()
	account := testpkg.CreateTestAccount(t, db, "existing-child-parent")
	profile := testpkg.CreateTestGuardianProfileNamed(t, db, "Eva", "Bestand", "existing-child-parent")
	_, err := db.NewUpdate().TableExpr(`users.guardian_profiles`).
		Set("account_id = ?", account.ID).Set("has_account = TRUE").
		Where("id = ?", profile.ID).
		Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	testpkg.CreateTestStudentGuardianLink(t, db, studentID, profile.ID, role)
	return account.ID
}

type existingChildSubmission struct {
	accountID *int64
	email     string
	children  []enrollmentAPI.SubmitChild
}

func submitChild(firstName, lastName string, birthday calendar.Date) enrollmentAPI.SubmitChild {
	grade := int16(2)
	return enrollmentAPI.SubmitChild{FirstName: firstName, LastName: lastName, DateOfBirth: birthday, TargetGradeLevel: &grade}
}

func (env *existingChildEnv) submit(t *testing.T, in existingChildSubmission) *enrollmentAPI.SubmitResult {
	t.Helper()
	email := in.email
	if email == "" {
		email = "existing-child@example.com"
	}
	res, err := env.matching.Submit(testpkg.Ctx(t), enrollmentAPI.SubmitRequest{
		TenantID:               testpkg.Tenant(t),
		PhaseID:                env.sourcePhase.ID,
		GuardianFirstName:      "Eva",
		GuardianLastName:       "Bestand",
		GuardianEmail:          email,
		GuardianAccountID:      in.accountID,
		GuardianSubmitEligible: in.accountID != nil,
		ConsentFlags:           testFormValues(rolloverConsentFlags()),
		Children:               in.children,
	})
	require.NoError(t, err)
	require.Len(t, res.Children, len(in.children))
	return res
}

func (env *existingChildEnv) decide(t *testing.T, input enrollmentAPI.DecideInput) error {
	t.Helper()
	if input.Status == "" {
		input.Status = enrollmentNative.DecisionApproved
	}
	if input.ReviewedBy == 0 {
		input.ReviewedBy = env.creatorID
	}
	return testpkg.WithTenantTx(t, testpkg.Ctx(t), env.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		_, err := testpkg.TenantQuery1(t, txCtx, env.db, env.decision.Decide, input)
		return err
	})
}

func (env *existingChildEnv) decideOutcome(t *testing.T, input enrollmentAPI.DecideInput) *enrollmentAPI.DecideOutcome {
	t.Helper()
	input.Status, input.ReviewedBy = enrollmentNative.DecisionApproved, env.creatorID
	var outcome *enrollmentAPI.DecideOutcome
	require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), env.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		out, err := testpkg.TenantQuery1(t, txCtx, env.db, env.decision.Decide, input)
		outcome = out
		return err
	}))
	return outcome
}

// guardianLinks lists the guardian links of the student.
func (env *existingChildEnv) guardianLinks(t *testing.T, studentID int64) []int64 {
	t.Helper()
	links, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), env.db, env.repos.StudentGuardian.FindByStudentID, studentID)
	require.NoError(t, err)
	ids := make([]int64, 0, len(links))
	for _, link := range links {
		ids = append(ids, link.GuardianProfileID)
	}
	return ids
}

func (env *existingChildEnv) child(t *testing.T, childID int64) *enrollmentNative.RequestChild {
	t.Helper()
	child, err := enrollmentAPI.ReadOwnerChildForTest(testpkg.Ctx(t), env.repos.Enrollment(), childID)
	require.NoError(t, err)
	return child
}

func (env *existingChildEnv) candidates(t *testing.T, requestID, childID int64) []enrollmentNative.ExistingChildCandidate {
	t.Helper()
	summary, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), env.db, env.decision.Get, requestID)
	require.NoError(t, err)
	return summary.ExistingChildCandidates[childID]
}

// studentsNamed counts the school's children with the name.
func studentsNamed(t *testing.T, db *bun.DB, firstName, lastName string) int {
	t.Helper()
	count, err := db.NewSelect().TableExpr(`users.student_profiles AS sp`).
		Join(`JOIN users.persons AS p ON p.id = sp.person_id`).
		Where("sp.tenant_id = ?", testpkg.Tenant(t)).
		Where("p.first_name = ? AND p.last_name = ?", firstName, lastName).
		Count(testpkg.Ctx(t))
	require.NoError(t, err)
	return int(count)
}

func TestExistingChild_OpenPhaseRenewsLinkedChildAndKeepsSiblingNew(t *testing.T) {
	t.Parallel()
	env := setupExistingChildTest(t, enrollmentNative.PhaseAudienceOpen)
	existing := enrolledChild(t, env.db, "Lina", "Bestand", existingChildBirthday)
	accountID := parentAccountLinkedTo(t, env.db, existing.ID, securityruntime.GuardianRolePrimaryGuardian)

	res := env.submit(t, existingChildSubmission{accountID: &accountID, children: []enrollmentAPI.SubmitChild{
		submitChild("Lina", "Bestand", existingChildBirthday),
		submitChild("Tom", "Bestand", calendar.NewDate(2020, 2, 3)),
	}})
	lina, tom := res.Children[0], res.Children[1]
	require.NotNil(t, lina.MatchedStudentID, "an unambiguous enrolled child of the submitting parent is pinned")
	assert.Equal(t, existing.ID, *lina.MatchedStudentID)
	assert.Nil(t, tom.MatchedStudentID, "a sibling stays a new child")
	assert.Empty(t, env.candidates(t, res.Request.ID, tom.ID))

	require.NoError(t, env.decide(t, enrollmentAPI.DecideInput{RequestID: res.Request.ID, ChildID: lina.ID}))
	require.NoError(t, env.decide(t, enrollmentAPI.DecideInput{RequestID: res.Request.ID, ChildID: tom.ID}))

	renewed := env.child(t, lina.ID)
	require.NotNil(t, renewed.CreatedStudentID)
	assert.Equal(t, existing.ID, *renewed.CreatedStudentID, "the approval renews the enrolled child")
	assert.False(t, renewed.MatchedByOffice)
	assert.Equal(t, 1, studentsNamed(t, env.db, "Lina", "Bestand"), "no second child is created")
	created := env.child(t, tom.ID)
	require.NotNil(t, created.CreatedStudentID)
	assert.NotEqual(t, existing.ID, *created.CreatedStudentID)
	assert.Equal(t, 1, studentsNamed(t, env.db, "Tom", "Bestand"), "the sibling is created as a new child")
}

func TestExistingChild_LinkedParentsPhaseMatchesThroughTheLink(t *testing.T) {
	t.Parallel()
	env := setupExistingChildTest(t, enrollmentNative.PhaseAudienceLinkedParents)
	existing := enrolledChild(t, env.db, "Mila", "Verknuepft", existingChildBirthday)
	accountID := parentAccountLinkedTo(t, env.db, existing.ID, securityruntime.GuardianRolePrimaryGuardian)

	res := env.submit(t, existingChildSubmission{accountID: &accountID, children: []enrollmentAPI.SubmitChild{
		submitChild("Mila", "Verknuepft", existingChildBirthday),
	}})
	require.NotNil(t, res.Children[0].MatchedStudentID)
	assert.Equal(t, existing.ID, *res.Children[0].MatchedStudentID)

	require.NoError(t, env.decide(t, enrollmentAPI.DecideInput{RequestID: res.Request.ID, ChildID: res.Children[0].ID}))
	assert.Equal(t, 1, studentsNamed(t, env.db, "Mila", "Verknuepft"))
}

// A link without the re-enrollment permission is not enough to renew the
// child on the parent's word: the office decides.
func TestExistingChild_LinkWithoutEnrollmentPermissionAsksTheOffice(t *testing.T) {
	t.Parallel()
	env := setupExistingChildTest(t, enrollmentNative.PhaseAudienceLinkedParents)
	existing := enrolledChild(t, env.db, "Ida", "Abholung", existingChildBirthday)
	accountID := parentAccountLinkedTo(t, env.db, existing.ID, securityruntime.GuardianRolePickupOnly)

	res := env.submit(t, existingChildSubmission{accountID: &accountID, children: []enrollmentAPI.SubmitChild{
		submitChild("Ida", "Abholung", existingChildBirthday),
	}})
	child := res.Children[0]
	assert.Nil(t, child.MatchedStudentID)
	candidates := env.candidates(t, res.Request.ID, child.ID)
	require.Len(t, candidates, 1)
	assert.Equal(t, existing.ID, candidates[0].StudentID)

	err := env.decide(t, enrollmentAPI.DecideInput{RequestID: res.Request.ID, ChildID: child.ID})
	require.ErrorIs(t, err, enrollmentNative.ErrChildPossiblyEnrolled)
}

// An anonymous public form proves nothing about the submitter, so even an
// exact match is left to the office, which can renew the child or create a
// new one.
func TestExistingChild_AnonymousExactMatchNeedsOfficeChoiceBothWays(t *testing.T) {
	t.Parallel()
	env := setupExistingChildTest(t, enrollmentNative.PhaseAudienceOpen)
	existing := enrolledChild(t, env.db, "Paul", "Offen", existingChildBirthday)

	renewal := env.submit(t, existingChildSubmission{email: "renewal@example.com", children: []enrollmentAPI.SubmitChild{
		submitChild("Paul", "Offen", existingChildBirthday),
	}})
	child := renewal.Children[0]
	assert.Nil(t, child.MatchedStudentID, "an anonymous submission pins nothing")
	candidates := env.candidates(t, renewal.Request.ID, child.ID)
	require.Len(t, candidates, 1)
	assert.Equal(t, existing.ID, candidates[0].StudentID)
	require.NotNil(t, candidates[0].DateOfBirth)
	assert.Equal(t, existingChildBirthday.String(), string(*candidates[0].DateOfBirth))

	err := env.decide(t, enrollmentAPI.DecideInput{RequestID: renewal.Request.ID, ChildID: child.ID})
	require.ErrorIs(t, err, enrollmentNative.ErrChildPossiblyEnrolled, "approval without the office's choice is refused")
	assert.Equal(t, enrollmentNative.ChildStatusSubmitted, env.child(t, child.ID).Status)

	unrelated := enrolledChild(t, env.db, "Paula", "Fremd", calendar.NewDate(2017, 1, 1))
	err = env.decide(t, enrollmentAPI.DecideInput{RequestID: renewal.Request.ID, ChildID: child.ID, ExistingStudentID: &unrelated.ID})
	require.ErrorIs(t, err, enrollmentNative.ErrExistingChildChoiceInvalid, "only a candidate can be chosen")

	outcome := env.decideOutcome(t, enrollmentAPI.DecideInput{RequestID: renewal.Request.ID, ChildID: child.ID, ExistingStudentID: &existing.ID})
	assert.Nil(t, outcome.PendingInvite, "the office's choice invites nobody")
	renewed := env.child(t, child.ID)
	require.NotNil(t, renewed.CreatedStudentID)
	assert.Equal(t, existing.ID, *renewed.CreatedStudentID)
	assert.True(t, renewed.MatchedByOffice)
	assert.Equal(t, 1, studentsNamed(t, env.db, "Paul", "Offen"))
	assert.Empty(t, env.guardianLinks(t, existing.ID), "the office's choice links none of the submitting family")

	// A confirmed edit of the request later still links nobody to the child.
	req, err := enrollmentAPI.ReadOwnerRequestForTest(testpkg.Ctx(t), env.repos.Enrollment(), renewal.Request.ID)
	require.NoError(t, err)
	req.GuardianEmail = "renewal-edited@example.com"
	require.NoError(t, enrollmentAPI.UpdateOwnerRequestGuardianForTest(testpkg.Ctx(t), env.repos.Enrollment(), req, true))
	_, err = changeRequestApplierForTest(t, env.decisionTestEnv).SyncApprovedChildData(testpkg.Ctx(t), enrollmentAPI.SyncApprovedChildDataInput{
		RequestID: renewal.Request.ID, ChildID: child.ID, ActorAccountID: env.creatorID, ReplaceTargetedData: true,
	})
	require.NoError(t, err)
	assert.Empty(t, env.guardianLinks(t, existing.ID), "a later sync links none of the submitting family either")

	// A different family with a namesake born the same day: the office
	// creates a new child on purpose.
	namesake := env.submit(t, existingChildSubmission{email: "namesake@example.com", children: []enrollmentAPI.SubmitChild{
		submitChild("Paul", "Offen", existingChildBirthday),
	}})
	require.NoError(t, env.decide(t, enrollmentAPI.DecideInput{RequestID: namesake.Request.ID, ChildID: namesake.Children[0].ID, CreateNewStudent: true}))
	created := env.child(t, namesake.Children[0].ID)
	require.NotNil(t, created.CreatedStudentID)
	assert.NotEqual(t, existing.ID, *created.CreatedStudentID)
	assert.Equal(t, 2, studentsNamed(t, env.db, "Paul", "Offen"))
}

// Several children with the name and birthday are shown to the office and
// never matched automatically, even when the parent is linked to one. A name
// with another birthday is no match: the child is created as usual.
func TestExistingChild_AmbiguousMatchAsksTheOfficeAndNameOnlyIsNew(t *testing.T) {
	t.Parallel()
	env := setupExistingChildTest(t, enrollmentNative.PhaseAudienceOpen)
	first := enrolledChild(t, env.db, "Ben", "Doppelt", existingChildBirthday)
	second := enrolledChild(t, env.db, "Ben", "Doppelt", existingChildBirthday)
	accountID := parentAccountLinkedTo(t, env.db, first.ID, securityruntime.GuardianRolePrimaryGuardian)
	nameOnly := enrolledChild(t, env.db, "Emma", "Datum", calendar.NewDate(2018, 1, 1))
	// A twin of the namesakes is not one of their candidates.
	enrolledChild(t, env.db, "Lea", "Doppelt", existingChildBirthday)

	res := env.submit(t, existingChildSubmission{accountID: &accountID, children: []enrollmentAPI.SubmitChild{
		submitChild("Ben", "Doppelt", existingChildBirthday),
		submitChild("Emma", "Datum", existingChildBirthday),
	}})
	ben, emma := res.Children[0], res.Children[1]
	assert.Nil(t, ben.MatchedStudentID, "two enrolled children of that name are ambiguous")
	assert.Nil(t, emma.MatchedStudentID)
	benCandidates := env.candidates(t, res.Request.ID, ben.ID)
	require.Len(t, benCandidates, 2)
	assert.ElementsMatch(t, []int64{first.ID, second.ID}, []int64{benCandidates[0].StudentID, benCandidates[1].StudentID})
	assert.Empty(t, env.candidates(t, res.Request.ID, emma.ID), "the same name with another birthday is no match")

	require.ErrorIs(t, env.decide(t, enrollmentAPI.DecideInput{RequestID: res.Request.ID, ChildID: ben.ID}), enrollmentNative.ErrChildPossiblyEnrolled)
	require.NoError(t, env.decide(t, enrollmentAPI.DecideInput{RequestID: res.Request.ID, ChildID: ben.ID, ExistingStudentID: &first.ID}))
	assert.Equal(t, 2, studentsNamed(t, env.db, "Ben", "Doppelt"))
	require.NoError(t, env.decide(t, enrollmentAPI.DecideInput{RequestID: res.Request.ID, ChildID: emma.ID}))
	created := env.child(t, emma.ID)
	require.NotNil(t, created.CreatedStudentID)
	assert.NotEqual(t, nameOnly.ID, *created.CreatedStudentID)
}

// A child of another school never matches, as candidate or pin.
func TestExistingChild_OtherSchoolsChildNeverMatches(t *testing.T) {
	t.Parallel()
	env := setupExistingChildTest(t, enrollmentNative.PhaseAudienceOpen)
	otherTenant, _ := testpkg.CreateTestTenant(t, env.db)
	foreign := testpkg.CreateTestStudentForTenant(t, env.db, otherTenant, "Noah", "Woanders", "1a")
	setPersonBirthday(t, env.db, foreign.PersonID, existingChildBirthday)
	accountID := parentAccountLinkedTo(t, env.db, enrolledChild(t, env.db, "Lotta", "Hier", existingChildBirthday).ID, securityruntime.GuardianRolePrimaryGuardian)

	res := env.submit(t, existingChildSubmission{accountID: &accountID, children: []enrollmentAPI.SubmitChild{
		submitChild("Noah", "Woanders", existingChildBirthday),
	}})
	child := res.Children[0]
	assert.Nil(t, child.MatchedStudentID)
	assert.Empty(t, env.candidates(t, res.Request.ID, child.ID))

	require.NoError(t, env.decide(t, enrollmentAPI.DecideInput{RequestID: res.Request.ID, ChildID: child.ID}))
	created := env.child(t, child.ID)
	require.NotNil(t, created.CreatedStudentID)
	assert.NotEqual(t, foreign.ID, *created.CreatedStudentID)
}

// new_students and existing_students keep their submit-time rules; their
// approvals take no choice.
func TestExistingChild_OtherAudiencesTakeNoChoice(t *testing.T) {
	t.Parallel()
	env := setupExistingChildTest(t, enrollmentNative.PhaseAudienceNewStudents)

	res := env.submit(t, existingChildSubmission{children: []enrollmentAPI.SubmitChild{
		submitChild("Jona", "Neu", existingChildBirthday),
	}})
	err := env.decide(t, enrollmentAPI.DecideInput{RequestID: res.Request.ID, ChildID: res.Children[0].ID, CreateNewStudent: true})
	require.ErrorIs(t, err, enrollmentNative.ErrExistingChildChoiceInvalid)
	require.NoError(t, env.decide(t, enrollmentAPI.DecideInput{RequestID: res.Request.ID, ChildID: res.Children[0].ID}))
}

// A student created by an earlier approval already has an active request,
// even though that request never needed a matched_student_id.
func TestExistingChild_CreatedStudentBlocksOfficeChoiceAndSubmitPin(t *testing.T) {
	t.Parallel()
	for _, linked := range []bool{false, true} {
		name := "office_choice"
		if linked {
			name = "parent_pin"
		}
		t.Run(name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			env := setupExistingChildTest(t, enrollmentNative.PhaseAudienceOpen)
			first := env.submit(t, existingChildSubmission{email: "first@example.com", children: []enrollmentAPI.SubmitChild{
				submitChild("Leni", "Neu", existingChildBirthday),
			}})
			require.NoError(t, env.decide(t, enrollmentAPI.DecideInput{RequestID: first.Request.ID, ChildID: first.Children[0].ID}))
			created := env.child(t, first.Children[0].ID)
			require.NotNil(t, created.CreatedStudentID)
			assert.Nil(t, created.MatchedStudentID)
			studentID := *created.CreatedStudentID
			in := existingChildSubmission{email: "second@example.com", children: []enrollmentAPI.SubmitChild{
				submitChild("Leni", "Neu", existingChildBirthday),
			}}
			if linked {
				accountID := parentAccountLinkedTo(t, env.db, studentID, securityruntime.GuardianRolePrimaryGuardian)
				in.accountID = &accountID
			}
			second := env.submit(t, in)
			child := second.Children[0]
			assert.Nil(t, child.MatchedStudentID, "a created student with an active request must not be pinned again")
			if linked {
				return
			}
			require.Len(t, env.candidates(t, second.Request.ID, child.ID), 1)
			err := env.decide(t, enrollmentAPI.DecideInput{RequestID: second.Request.ID, ChildID: child.ID, ExistingStudentID: &studentID})
			require.ErrorIs(t, err, enrollmentNative.ErrExistingStudentAlreadyRequested)
			assert.Equal(t, enrollmentNative.ChildStatusSubmitted, env.child(t, child.ID).Status)
		})
	}
}

// A later confirmed request edit must not undo the school's corrections to
// an office-matched child's master data, with or without field replacement.
func TestExistingChild_OfficeMatchedSyncPreservesStudentMasterData(t *testing.T) {
	t.Parallel()
	env := setupExistingChildTest(t, enrollmentNative.PhaseAudienceOpen)
	existing := enrolledChild(t, env.db, "Paul", "Offen", existingChildBirthday)
	submitted := env.submit(t, existingChildSubmission{children: []enrollmentAPI.SubmitChild{
		submitChild("Paul", "Offen", existingChildBirthday),
	}})
	child := submitted.Children[0]
	require.NoError(t, env.decide(t, enrollmentAPI.DecideInput{RequestID: submitted.Request.ID, ChildID: child.ID, ExistingStudentID: &existing.ID}))

	ctx := testpkg.Ctx(t)
	birthday := calendar.NewDate(2018, 4, 16)
	_, err := env.db.NewUpdate().TableExpr(`users.persons`).
		Set("first_name = ?", "Paula").Set("last_name = ?", "Korrigiert").Set("birthday = ?", birthday).
		Where("id = ?", existing.PersonID).Exec(ctx)
	require.NoError(t, err)
	student, err := testpkg.TenantQuery1(t, ctx, env.db, env.repos.Student.FindByID, any(existing.ID))
	require.NoError(t, err)
	student.SchoolClass = "3b"
	require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, env.repos.Student.Update, student))

	for _, replace := range []bool{false, true} {
		_, err = changeRequestApplierForTest(t, env.decisionTestEnv).SyncApprovedChildData(ctx, enrollmentAPI.SyncApprovedChildDataInput{
			RequestID: submitted.Request.ID, ChildID: child.ID, ActorAccountID: env.creatorID, ReplaceTargetedData: replace,
		})
		require.NoError(t, err)
		student, err := testpkg.TenantQuery1(t, ctx, env.db, env.repos.Student.FindByID, any(existing.ID))
		require.NoError(t, err)
		person, err := testpkg.TenantQuery1(t, ctx, env.db, env.repos.Person.FindByID, any(student.PersonID))
		require.NoError(t, err)
		assert.Equal(t, "Paula", person.FirstName)
		assert.Equal(t, "Korrigiert", person.LastName)
		require.NotNil(t, person.Birthday)
		assert.Equal(t, birthday, *person.Birthday)
		assert.Equal(t, "3b", student.SchoolClass)
		assert.Empty(t, env.guardianLinks(t, existing.ID))
	}
}

func TestExistingChild_OfficeApprovalPreservesClass(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{enrollmentNative.PhaseKindCustom, enrollmentNative.PhaseKindSchoolYear} {
		t.Run(kind, func(t *testing.T) {
			testpkg.OwnTenant(t)
			env := setupExistingChildTest(t, enrollmentNative.PhaseAudienceOpen)
			ctx := testpkg.Ctx(t)
			env.sourcePhase.Kind = kind
			require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, env.repos.Enrollment().UpdatePhase, enrollmentAPI.OwnerPhaseForTest(env.sourcePhase)))
			existing := enrolledChild(t, env.db, "Paul", "Offen", existingChildBirthday)
			submitted := env.submit(t, existingChildSubmission{children: []enrollmentAPI.SubmitChild{
				submitChild("Paul", "Offen", existingChildBirthday),
			}})
			child := submitted.Children[0]
			outcome := env.decideOutcome(t, enrollmentAPI.DecideInput{RequestID: submitted.Request.ID, ChildID: child.ID, ExistingStudentID: &existing.ID})
			assert.Nil(t, outcome.PendingInvite)
			student, err := testpkg.TenantQuery1(t, ctx, env.db, env.repos.Student.FindByID, any(existing.ID))
			require.NoError(t, err)
			assert.Equal(t, "1a", student.SchoolClass, "an office match must preserve the running class")
			planned, err := testpkg.TenantQuery1(t, ctx, env.db, env.repos.Enrollment().ChildClassSwitch, child.ID)
			require.NoError(t, err)
			assert.Nil(t, planned, "an office match must not plan a class from the unrelated submission")
			require.NotNil(t, student.EnrolledUntil)
			assert.Equal(t, calendar.Date(env.sourcePhase.ServiceEndDate), *student.EnrolledUntil, "the enrollment window is still renewed")
			assert.Empty(t, env.guardianLinks(t, existing.ID))
		})
	}
}

func TestExistingChild_OfficeProtectionSurvivesRolloverDescendants(t *testing.T) {
	t.Parallel()
	env := setupExistingChildTest(t, enrollmentNative.PhaseAudienceOpen)
	ctx := testpkg.Ctx(t)
	publishLegacyDuplicateTargetSchema(t, env.decisionTestEnv, "office-rollover-health", []enrollmentNative.FormField{{
		Key: "health_info", Label: "Gesundheit", Type: enrollmentNative.FormFieldTextarea,
		AppliesToCh: true, Target: enrollmentNative.TargetStudentHealthInfo,
	}})
	existing := enrolledChild(t, env.db, "Paul", "Offen", existingChildBirthday)
	input := submitChild("Paul", "Offen", existingChildBirthday)
	input.CustomData = testFormValues(map[string]any{"health_info": "synthetic-unrelated-value"})
	submitted := env.submit(t, existingChildSubmission{children: []enrollmentAPI.SubmitChild{input}})
	require.NoError(t, env.decide(t, enrollmentAPI.DecideInput{RequestID: submitted.Request.ID, ChildID: submitted.Children[0].ID, ExistingStudentID: &existing.ID}))

	roll := validRolloverRequest(env.rolloverTestEnv, enrollmentNative.PhaseRolloverModeOptOut, true)
	for year := range 2 {
		roll.ServiceStartDate = calendar.NewDate(2027+year, 9, 1)
		roll.ServiceEndDate = calendar.NewDate(2028+year, 7, 31)
		roll.Name += "-next"
		result, err := testpkg.TenantQuery1(t, ctx, env.db, env.rolloverSvc.CreatePhaseFromSource, roll)
		require.NoError(t, err)
		children, err := testpkg.TenantQuery2(t, ctx, env.db, env.repos.Enrollment().ChildrenByPhaseStatuses, result.Phase.ID, []string{enrollmentNative.ChildStatusAutoRenewed})
		require.NoError(t, err)
		require.Len(t, children, 1)
		child := children[0]
		assert.True(t, child.MatchedByOffice, "every rollover generation must retain enrollment-only protection")
		outcome := env.decideOutcome(t, enrollmentAPI.DecideInput{RequestID: child.RequestID, ChildID: child.ID})
		assert.Nil(t, outcome.PendingInvite)
		require.NotNil(t, outcome.Child.CreatedStudentID)
		assert.Equal(t, existing.ID, *outcome.Child.CreatedStudentID)
		_, err = changeRequestApplierForTest(t, env.decisionTestEnv).SyncApprovedChildData(ctx, enrollmentAPI.SyncApprovedChildDataInput{
			RequestID: child.RequestID, ChildID: child.ID, ActorAccountID: env.creatorID, ReplaceTargetedData: true,
		})
		require.NoError(t, err)
		assert.Empty(t, env.guardianLinks(t, existing.ID), "a descendant edit must not link the unrelated submitting family")
		student, err := testpkg.TenantQuery1(t, ctx, env.db, env.repos.Student.FindByID, any(existing.ID))
		require.NoError(t, err)
		assert.Nil(t, student.HealthInfo, "a descendant edit must not apply the unrelated form data")
		assert.Equal(t, "1a", student.SchoolClass)
		roll.SourcePhaseID = result.Phase.ID
	}
}

func TestExistingChild_ReplacementEditPreservesOfficeProtection(t *testing.T) {
	t.Parallel()
	env := setupExistingChildTest(t, enrollmentNative.PhaseAudienceOpen)
	ctx := testpkg.Ctx(t)
	existing := enrolledChild(t, env.db, "Paul", "Offen", existingChildBirthday)
	submitted := env.submit(t, existingChildSubmission{children: []enrollmentAPI.SubmitChild{
		submitChild("Paul", "Offen", existingChildBirthday),
	}})
	require.NoError(t, env.decide(t, enrollmentAPI.DecideInput{RequestID: submitted.Request.ID, ChildID: submitted.Children[0].ID, ExistingStudentID: &existing.ID}))
	roll, err := testpkg.TenantQuery1(t, ctx, env.db, env.rolloverSvc.CreatePhaseFromSource,
		validRolloverRequest(env.rolloverTestEnv, enrollmentNative.PhaseRolloverModeOptOut, true))
	require.NoError(t, err)
	children, err := testpkg.TenantQuery2(t, ctx, env.db, env.repos.Enrollment().ChildrenByPhaseStatuses, roll.Phase.ID, []string{enrollmentNative.ChildStatusAutoRenewed})
	require.NoError(t, err)
	require.Len(t, children, 1)
	child := children[0]
	require.True(t, child.MatchedByOffice)
	require.NoError(t, testpkg.TenantCommand4(t, ctx, env.db, env.repos.Enrollment().UpdateChildStatus, child.ID, enrollmentNative.ChildStatusSubmitted, nil, env.creatorID))
	req, err := enrollmentAPI.ReadOwnerRequestForTest(ctx, env.repos.Enrollment(), child.RequestID)
	require.NoError(t, err)
	editChild := submitChild("Paul", "Offen", existingChildBirthday)
	editChild.TargetGradeLevel = child.TargetGradeLevel
	replaced, err := env.matching.ReplaceEditable(ctx, req.StatusToken, enrollmentAPI.SubmitRequest{
		TenantID: testpkg.Tenant(t), PhaseID: roll.Phase.ID,
		GuardianFirstName: "Eva", GuardianLastName: "Bestand", GuardianEmail: req.GuardianEmail,
		ConsentFlags: testFormValues(rolloverConsentFlags()), Children: []enrollmentAPI.SubmitChild{editChild},
	})
	require.NoError(t, err)
	require.Len(t, replaced.Children, 1)
	replacement := replaced.Children[0]
	assert.True(t, replacement.MatchedByOffice, "replacement must carry enrollment-only protection")
	require.NotNil(t, replacement.RolloverSourceChildID)
	assert.Equal(t, *child.RolloverSourceChildID, *replacement.RolloverSourceChildID)
	outcome := env.decideOutcome(t, enrollmentAPI.DecideInput{RequestID: req.ID, ChildID: replacement.ID})
	assert.Nil(t, outcome.PendingInvite)
	planned, err := testpkg.TenantQuery1(t, ctx, env.db, env.repos.Enrollment().ChildClassSwitch, replacement.ID)
	require.NoError(t, err)
	assert.Nil(t, planned, "replacement must not plan a class change")
	_, err = changeRequestApplierForTest(t, env.decisionTestEnv).SyncApprovedChildData(ctx, enrollmentAPI.SyncApprovedChildDataInput{
		RequestID: req.ID, ChildID: replacement.ID, ActorAccountID: env.creatorID, ReplaceTargetedData: true,
	})
	require.NoError(t, err)
	assert.Empty(t, env.guardianLinks(t, existing.ID), "replacement must not authorize the unrelated family")
}

// A direct staff correction of an office-matched child writes only the
// fields the office changed; the request's other data stays off the child
// and no guardian is linked (#6053).
func TestExistingChild_OfficeMatchedCorrectionAppliesOnlyCorrectedFields(t *testing.T) {
	t.Parallel()
	env := setupExistingChildTest(t, enrollmentNative.PhaseAudienceOpen)
	existing := enrolledChild(t, env.db, "Paul", "Offen", existingChildBirthday)
	submitted := env.submit(t, existingChildSubmission{children: []enrollmentAPI.SubmitChild{
		submitChild("Paul", "Offen", existingChildBirthday),
	}})
	child := submitted.Children[0]
	require.NoError(t, env.decide(t, enrollmentAPI.DecideInput{RequestID: submitted.Request.ID, ChildID: child.ID, ExistingStudentID: &existing.ID}))
	ctx := testpkg.Ctx(t)
	_, err := env.db.NewUpdate().TableExpr(`users.persons`).Set("first_name = ?", "Paula").Where("id = ?", existing.PersonID).Exec(ctx)
	require.NoError(t, err)

	corrected := calendar.NewDate(2018, 4, 16)
	svc := newChangeRequestServiceWithDecisionForTest(t, env.decisionTestEnv)
	require.NoError(t, testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		_, err := testpkg.TenantQuery1(t, txCtx, env.db, svc.CorrectApprovedChildData, enrollmentAPI.CorrectApprovedChildDataInput{
			RequestID: submitted.Request.ID, ChildID: child.ID, FirstName: child.FirstName, LastName: child.LastName,
			DateOfBirth: corrected, TargetGradeLevel: child.TargetGradeLevel, Reason: "Geburtsdatum berichtigt", ActorAccountID: env.creatorID,
		})
		return err
	}))
	person, err := testpkg.TenantQuery1(t, ctx, env.db, env.repos.Person.FindByID, any(existing.PersonID))
	require.NoError(t, err)
	require.NotNil(t, person.Birthday)
	assert.Equal(t, corrected, *person.Birthday, "the corrected birthday reaches the child")
	assert.Equal(t, "Paula", person.FirstName, "an uncorrected name keeps the school's master data")
	student, err := testpkg.TenantQuery1(t, ctx, env.db, env.repos.Student.FindByID, any(existing.ID))
	require.NoError(t, err)
	assert.Equal(t, "1a", student.SchoolClass, "an uncorrected class keeps the school's master data")
	assert.Empty(t, env.guardianLinks(t, existing.ID))
}

// A surname-only correction of an office-matched child writes only the
// surname: a first name the school changed on the child stays (#6053 review).
func TestExistingChild_OfficeMatchedSurnameCorrectionKeepsTheFirstName(t *testing.T) {
	t.Parallel()
	env := setupExistingChildTest(t, enrollmentNative.PhaseAudienceOpen)
	existing := enrolledChild(t, env.db, "Paul", "Offen", existingChildBirthday)
	submitted := env.submit(t, existingChildSubmission{children: []enrollmentAPI.SubmitChild{
		submitChild("Paul", "Offen", existingChildBirthday),
	}})
	child := submitted.Children[0]
	require.NoError(t, env.decide(t, enrollmentAPI.DecideInput{RequestID: submitted.Request.ID, ChildID: child.ID, ExistingStudentID: &existing.ID}))
	ctx := testpkg.Ctx(t)
	_, err := env.db.NewUpdate().TableExpr(`users.persons`).Set("first_name = ?", "Paula").Where("id = ?", existing.PersonID).Exec(ctx)
	require.NoError(t, err)

	svc := newChangeRequestServiceWithDecisionForTest(t, env.decisionTestEnv)
	require.NoError(t, testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		_, err := testpkg.TenantQuery1(t, txCtx, env.db, svc.CorrectApprovedChildData, enrollmentAPI.CorrectApprovedChildDataInput{
			RequestID: submitted.Request.ID, ChildID: child.ID, FirstName: child.FirstName, LastName: "Berichtigt",
			DateOfBirth: child.DateOfBirth, TargetGradeLevel: child.TargetGradeLevel, Reason: "Nachname berichtigt", ActorAccountID: env.creatorID,
		})
		return err
	}))
	person, err := testpkg.TenantQuery1(t, ctx, env.db, env.repos.Person.FindByID, any(existing.PersonID))
	require.NoError(t, err)
	assert.Equal(t, "Berichtigt", person.LastName, "the corrected surname reaches the child")
	assert.Equal(t, "Paula", person.FirstName, "an uncorrected first name keeps the school's master data")
	assert.Empty(t, env.guardianLinks(t, existing.ID))
}

// Two approvals of two requests for the same new child must not both create
// it. Each read no enrolled candidate before the other committed; without a
// lock on the child's identity both created a child (#6170). The second
// approval must wait for the first and then ask the office. The identity lock
// holds on its own; the tenant recurrence gate the root also binds must not
// be what keeps the child single.
func TestExistingChild_ConcurrentApprovalsCreateTheChildOnce(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		gated bool
	}{{"identity lock alone", false}, {"with the approval gates", true}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assertConcurrentApprovalsCreateTheChildOnce(t, tc.gated)
		})
	}
}

func assertConcurrentApprovalsCreateTheChildOnce(t *testing.T, gated bool) {
	t.Helper()
	testpkg.OwnTenant(t)
	env := setupExistingChildTest(t, enrollmentNative.PhaseAudienceOpen)
	if gated {
		lock := owners.MustNewTimetableRecurrenceLock(testpkg.NewDatabase(t, env.db)).LockRecurrenceWrites
		bookings := newBookingsForTest(env.rolloverTestEnv, nil, lock, nil)
		env.decision = newDecisionServiceForTestWithBookings(t, env.rolloverTestEnv, bookings, nil, lock, nil, nil)
	}
	first := env.submit(t, existingChildSubmission{email: "first-parent@example.com", children: []enrollmentAPI.SubmitChild{
		submitChild("Mila", "Zugleich", existingChildBirthday),
	}})
	second := env.submit(t, existingChildSubmission{email: "second-parent@example.com", children: []enrollmentAPI.SubmitChild{
		submitChild(" mila", "ZUGLEICH ", existingChildBirthday),
	}})
	approve := func(res *enrollmentAPI.SubmitResult) enrollmentAPI.DecideInput {
		return enrollmentAPI.DecideInput{
			RequestID: res.Request.ID, ChildID: res.Children[0].ID,
			Status: enrollmentNative.DecisionApproved, ReviewedBy: env.creatorID,
		}
	}

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 20*time.Second)
	defer cancel()
	done := make(chan error, 1)
	firstErr := testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
		if _, err := env.decision.Decide(txCtx, approve(first)); err != nil {
			return err
		}
		var pid int
		require.NoError(t, tx.NewRaw("SELECT pg_backend_pid()").Scan(txCtx, &pid))
		go func() {
			done <- testpkg.WithinTenantContext(t, ctx, env.db, testpkg.Tenant(t), func(otherCtx context.Context) error {
				_, err := env.decision.Decide(otherCtx, approve(second))
				return err
			})
		}()
		if finished := waitForBlockedWriter(t, txCtx, tx, pid, done); finished != nil {
			done <- *finished
			return fmt.Errorf("the second approval finished without waiting for the first: %w", *finished)
		}
		return nil
	})
	var secondErr error
	select {
	case secondErr = <-done:
	case <-ctx.Done():
		t.Fatalf("the second approval did not finish; the first returned %v", firstErr)
	}

	count, err := env.db.NewSelect().TableExpr(`users.student_profiles AS sp`).
		Join(`JOIN users.persons AS p ON p.id = sp.person_id`).
		Where("sp.tenant_id = ?", testpkg.Tenant(t)).
		Where("LOWER(TRIM(p.first_name)) = 'mila' AND LOWER(TRIM(p.last_name)) = 'zugleich'").
		Count(testpkg.Ctx(t))
	require.NoError(t, err)
	assert.Equal(t, 1, int(count), "the child exists once")
	require.NoError(t, firstErr)
	require.ErrorIs(t, secondErr, enrollmentNative.ErrChildPossiblyEnrolled,
		"the second approval asks the office about the child the first created")
	assert.Len(t, env.candidates(t, second.Request.ID, second.Children[0].ID), 1)
}
