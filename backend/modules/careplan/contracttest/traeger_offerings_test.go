package contracttest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func traegerOfferingDraft(name string, capacity int) careplan.TraegerOfferingDraft {
	return careplan.TraegerOfferingDraft{
		Fields: careplan.TraegerOfferingFields{
			Name: name, DaysOfWeekMode: "fixed", AvailableDays: []string{"mon", "tue"},
			Capacity: &capacity, CountsAsCare: true, SelectionRule: "optional",
			PickupTimes: map[string]string{"mon": "15:00", "tue": "15:00"},
		},
		Prices: []careplan.TraegerCarePrice{{Weekday: "mon", MonthlyCents: 1600}, {Weekday: "tue", MonthlyCents: 1600}},
	}
}

func newOfferingTraeger(t *testing.T, name string) (int64, int64) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: name, Slug: fmt.Sprintf("traeger-angebot-%d", testpkg.UniqueTestTenantID(t)), Active: true,
	}).ID
	testpkg.OwnTraegerCatalog(t, db, organization)
	var phase int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.traeger_phases (organization_id, draft) VALUES (?, '{}') RETURNING id`,
		organization).Scan(context.Background(), &phase))
	return organization, phase
}

// TestTraegerOfferingStatementsKeepTheirTraegerBeneathRLS pins the Träger
// filter of every statement of the Träger offering store (ADR 0195 section
// 2) beneath RLS, as #5952 and #6001 do: in the Träger transaction of A, as
// the owner of the tables, each statement is asked for B's offering. Each
// assertion goes red when its statement drops its organization_id filter.
func TestTraegerOfferingStatementsKeepTheirTraegerBeneathRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	bg := context.Background()
	own, ownPhase := newOfferingTraeger(t, "Trägerangebot A")
	foreign, foreignPhase := newOfferingTraeger(t, "Trägerangebot B")
	var foreignOffering int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.traeger_care_offerings (organization_id, traeger_phase_id, draft)
		VALUES (?, ?, '{"fields":{"name":"Fremd"}}') RETURNING id`, foreign, foreignPhase).Scan(bg, &foreignOffering))
	store := careplanCompose.NewTraegerOfferings()

	ctx := testpkg.ContextWithTenantRuntime(bg, testpkg.TenantRuntime(t, db))
	require.NoError(t, testpkg.WithinTraeger(ctx, own, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		offerings, err := store.ListTraegerOfferings(ctx, own, foreignPhase)
		require.NoError(t, err)
		assert.Empty(t, offerings, "A lists no offering of B")
		_, err = store.TraegerOffering(ctx, own, foreignOffering)
		assert.ErrorIs(t, err, careplan.ErrTraegerOfferingNotFound, "A reads no offering of B")
		_, err = store.UpdateTraegerOffering(ctx, own, foreignOffering, traegerOfferingDraft("Übernommen", 10))
		assert.ErrorIs(t, err, careplan.ErrTraegerOfferingNotFound, "A changes no offering of B")
		_, err = store.WithdrawTraegerOffering(ctx, own, foreignOffering)
		assert.ErrorIs(t, err, careplan.ErrTraegerOfferingNotFound, "A withdraws no offering of B")
		err = store.RecordTraegerOfferingOutcomes(ctx, own, map[int64][]careplan.TraegerOfferingOutcome{foreignOffering: {{SchoolID: 1, Outcome: "applied"}}})
		assert.ErrorIs(t, err, careplan.ErrTraegerOfferingNotFound, "A records no outcome on B's offering")
		released, err := store.ReleaseTraegerOfferings(ctx, own, foreignPhase, 0)
		require.NoError(t, err)
		assert.Empty(t, released, "A releases no offering of B")
		created, err := store.CreateTraegerOffering(ctx, own, ownPhase, traegerOfferingDraft("Eigen", 10))
		require.NoError(t, err)
		err = store.RecordTraegerOfferingOutcomes(ctx, own, map[int64][]careplan.TraegerOfferingOutcome{
			created.ID: {{SchoolID: testpkg.Tenant(t), Outcome: "applied"}}, foreignOffering: {{SchoolID: testpkg.Tenant(t), Outcome: "applied"}},
		})
		assert.ErrorIs(t, err, careplan.ErrTraegerOfferingNotFound, "a mixed batch validates all offerings before writing any outcome")
		ownOffering, err := store.TraegerOffering(ctx, own, created.ID)
		require.NoError(t, err)
		assert.Empty(t, ownOffering.Schools)
		return nil
	}))
	require.NoError(t, testpkg.WithinTraeger(ctx, own, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		_, err := store.CreateTraegerOffering(ctx, own, foreignPhase, traegerOfferingDraft("In B", 10))
		assert.ErrorIs(t, err, careplan.ErrTraegerOfferingNotFound, "A adds no offering to B's phase")
		return nil
	}))

	var name string
	var released *int
	require.NoError(t, db.NewRaw(`SELECT draft->'fields'->>'name', released_version FROM enrollment.traeger_care_offerings WHERE id = ?`,
		foreignOffering).Scan(bg, &name, &released))
	assert.Equal(t, "Fremd", name)
	assert.Nil(t, released, "A releases no offering of B")
}

// TestTraegerOfferingDraftFollowsTheCatalogRules pins that an office draft is
// an offering every school could store, and that its prices follow the form
// of the Elternabrechnung.
func TestTraegerOfferingDraftFollowsTheCatalogRules(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	own, phase := newOfferingTraeger(t, "Trägerangebot Regeln")
	offerings := careplanCompose.NewTraegerOfferings()
	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	require.NoError(t, testpkg.WithinTraeger(ctx, own, func(ctx context.Context) error {
		missingPickup := traegerOfferingDraft("Ohne Gehzeit", 10)
		missingPickup.Fields.PickupTimes = nil
		_, err := offerings.CreateTraegerOffering(ctx, own, phase, missingPickup)
		assert.ErrorIs(t, err, careplan.ErrCareOfferingPickupTimesRequired)
		mixed := traegerOfferingDraft("Gemischt", 10)
		days := 2
		mixed.Prices = append(mixed.Prices, careplan.TraegerCarePrice{DaysPerWeek: &days, MonthlyCents: 2000})
		_, err = offerings.CreateTraegerOffering(ctx, own, phase, mixed)
		assert.ErrorIs(t, err, careplan.ErrCareOfferingConfigInvalid, "a module has one price form")
		sibling := int64(2000)
		expensive := traegerOfferingDraft("Geschwister", 10)
		expensive.Prices[0].SiblingCents = &sibling
		_, err = offerings.CreateTraegerOffering(ctx, own, phase, expensive)
		assert.ErrorIs(t, err, careplan.ErrCareOfferingConfigInvalid, "a further child never pays more")
		template := traegerOfferingDraft("Vertrag", 10)
		template.ContractTemplate = &careplan.TraegerContractTemplate{TermKind: "fixed_months", NoticeTo: "month_end"}
		_, err = offerings.CreateTraegerOffering(ctx, own, phase, template)
		assert.ErrorIs(t, err, careplan.ErrCareOfferingConfigInvalid, "a fixed term needs its months")

		created, err := offerings.CreateTraegerOffering(ctx, own, phase, traegerOfferingDraft("Frühbetreuung", 20))
		require.NoError(t, err)
		released, err := offerings.ReleaseTraegerOfferings(ctx, own, phase, 0)
		require.NoError(t, err)
		require.Len(t, released, 1)
		require.NotNil(t, released[0].ReleasedVersion)
		assert.Equal(t, 1, *released[0].ReleasedVersion)
		released, err = offerings.ReleaseTraegerOfferings(ctx, own, phase, 0)
		require.NoError(t, err)
		assert.Equal(t, 1, *released[0].ReleasedVersion, "an unchanged draft writes no version")
		_, err = offerings.WithdrawTraegerOffering(ctx, own, created.ID)
		require.NoError(t, err)
		_, err = offerings.UpdateTraegerOffering(ctx, own, created.ID, traegerOfferingDraft("Neu", 20))
		assert.ErrorIs(t, err, careplan.ErrTraegerOfferingWithdrawn)
		return nil
	}))
}

// TestTraegerOfferingCopyKeepsTheSchoolsChanges drives a copy through the
// catalog (ADR 0195 sections 5 to 7): a new copy takes every value of the
// version, a school change becomes a deviation that the next version keeps,
// the school cannot delete the copy, and a withdrawal deletes a copy without
// bookings.
func TestTraegerOfferingCopyKeepsTheSchoolsChanges(t *testing.T) {
	t.Parallel()
	db, _, phase := setupCareTest(t)
	catalog := testCareOfferingCatalog(t, db)
	copies, err := careplanCompose.NewTraegerOfferingCopies(catalog, func(context.Context) error { return nil })
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	inTx := func(fn func(context.Context) error) {
		t.Helper()
		require.NoError(t, fixtureEnrollmentCommand(ctx, fn))
	}
	sourceID := testpkg.UniqueTestTenantID(t)
	first := traegerOfferingDraft("Frühbetreuung", 20).Fields

	var copyID int64
	inTx(func(ctx context.Context) error {
		applied, err := copies.ApplyTraegerOffering(ctx, careplan.TraegerOfferingCopy{SourceID: sourceID, Version: 1, Fields: first, PhaseID: phase.ID})
		require.NoError(t, err)
		assert.True(t, applied.Created)
		copyID = applied.CopyID
		return nil
	})
	inTx(func(ctx context.Context) error {
		offering, err := catalog.FindOffering(ctx, copyID)
		require.NoError(t, err)
		require.NotNil(t, offering.Traeger)
		assert.Empty(t, offering.Traeger.Deviations, "a new copy carries the Träger's values")
		capacity := 25
		offering.Capacity = &capacity
		offering.IsActive = false
		_, err = catalog.UpdateOffering(ctx, offering)
		return err
	})

	second := first
	second.Name = "Frühbetreuung ab 7 Uhr"
	thirty := 30
	second.Capacity = &thirty
	inTx(func(ctx context.Context) error {
		applied, err := copies.ApplyTraegerOffering(ctx, careplan.TraegerOfferingCopy{SourceID: sourceID, Version: 2, Fields: second, PhaseID: phase.ID})
		require.NoError(t, err)
		states := map[string]string{}
		for _, field := range applied.Fields {
			states[field.Field] = field.State
		}
		assert.Equal(t, "changes", states["name"])
		assert.Equal(t, "keeps", states["capacity"], "the school's capacity stays")
		assert.Equal(t, "keeps", states["is_active"], "the school's deactivation stays")
		offering, err := catalog.FindOffering(ctx, copyID)
		require.NoError(t, err)
		assert.Equal(t, "Frühbetreuung ab 7 Uhr", offering.Name)
		require.NotNil(t, offering.Capacity)
		assert.Equal(t, 25, *offering.Capacity)
		assert.False(t, offering.IsActive)
		assert.Equal(t, []string{"capacity", "is_active"}, offering.Traeger.Deviations)
		assert.Equal(t, 2, offering.Traeger.Version)
		return nil
	})

	bookedSource := sourceID + 1
	var bookedID int64
	inTx(func(ctx context.Context) error {
		applied, err := copies.ApplyTraegerOffering(ctx, careplan.TraegerOfferingCopy{SourceID: bookedSource, Version: 1, Fields: traegerOfferingDraft("Spätbetreuung", 20).Fields, PhaseID: phase.ID})
		require.NoError(t, err)
		bookedID = applied.CopyID
		return nil
	})
	bookTraegerCopy(t, db, phase.ID, bookedID)

	err = fixtureEnrollmentCommand(ctx, func(ctx context.Context) error { return catalog.DeleteOffering(ctx, copyID) })
	assert.ErrorIs(t, err, careplan.ErrTraegerCopyDelete, "only the office withdraws a copy")

	inTx(func(ctx context.Context) error {
		withdrawal, err := copies.WithdrawTraegerOffering(ctx, sourceID)
		require.NoError(t, err)
		assert.Equal(t, careplan.TraegerOfferingWithdrawal{Found: true, Deleted: true}, withdrawal)
		_, err = catalog.FindOffering(ctx, copyID)
		assert.ErrorIs(t, err, careplan.ErrCareOfferingNotFound)
		return nil
	})
	inTx(func(ctx context.Context) error {
		withdrawal, err := copies.WithdrawTraegerOffering(ctx, bookedSource)
		require.NoError(t, err)
		assert.Equal(t, careplan.TraegerOfferingWithdrawal{Found: true}, withdrawal, "a booked copy stays")
		offering, err := catalog.FindOffering(ctx, bookedID)
		require.NoError(t, err)
		assert.False(t, offering.IsActive, "the booked copy becomes inactive")
		assert.Nil(t, offering.Traeger, "and an ordinary offering of the school")
		return nil
	})
}

// bookTraegerCopy books a child onto the offering, the row whose ON DELETE
// RESTRICT key keeps the offering.
func bookTraegerCopy(t *testing.T, db *bun.DB, phaseID, offeringID int64) {
	t.Helper()
	schoolID := testpkg.Tenant(t)
	student := testpkg.CreateTestStudent(t, db, "Kopie", "Gebucht", "2a")
	err := testpkg.WithTenantTx(t, testpkg.Ctx(t), db, schoolID, func(ctx context.Context, tx bun.Tx) error {
		var requestID, childID int64
		if err := tx.NewRaw(`INSERT INTO enrollment.requests (tenant_id,phase_id,guardian_first_name,guardian_last_name,guardian_email,status_token,consent_flags,custom_data)
			VALUES (?,?,'Erika','Muster','traeger-kopie@example.test',?,'{}'::jsonb,'{}'::jsonb) RETURNING id`,
			schoolID, phaseID, fmt.Sprintf("traeger-kopie-%d", offeringID)).Scan(ctx, &requestID); err != nil {
			return err
		}
		if err := tx.NewRaw(`INSERT INTO enrollment.request_children
			(tenant_id,request_id,first_name,last_name,date_of_birth,status,activation_mode,sort_order,custom_data,created_student_id)
			VALUES (?,?,'Kopie','Gebucht','2018-04-15','approved','scheduled',0,'{}'::jsonb,?) RETURNING id`,
			schoolID, requestID, student.ID).Scan(ctx, &childID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO enrollment.care_offering_bookings (tenant_id,request_child_id,care_offering_id,manual_selected_days)
			VALUES (?,?,?,'["mon"]'::jsonb)`, schoolID, childID, offeringID)
		return err
	})
	require.NoError(t, err)
}

// Outcomes validate and persist a whole release in two statements. School
// billing comparisons retain the immutable applied draft after newer releases.
func TestTraegerOfferingOutcomesAreBatchedAndKeepAppliedTerms(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	own, phase := newOfferingTraeger(t, "Trägerangebot Ergebnisse")
	school := testpkg.CreateTestSchoolInOrganization(t, db, own, "Ergebnisse")
	offerings := careplanCompose.NewTraegerOfferings()
	ctx := testpkg.ContextWithTenantRuntime(t.Context(), testpkg.TenantRuntime(t, db))
	counter := testpkg.CaptureQueriesForContext(t, db)
	one := 1
	outcomes := map[int64][]careplan.TraegerOfferingOutcome{}
	copiedIDs := map[int64]int64{}
	require.NoError(t, testpkg.WithinTraeger(counter.Context(ctx), own, func(ctx context.Context) error {
		for index := range 5 {
			created, err := offerings.CreateTraegerOffering(ctx, own, phase, traegerOfferingDraft(fmt.Sprintf("Modul %d", index), 20))
			require.NoError(t, err)
			copyID := testpkg.UniqueTestTenantID(t)
			copiedIDs[created.ID] = copyID
			outcomes[created.ID] = []careplan.TraegerOfferingOutcome{{SchoolID: school, AppliedVersion: &one, AppliedCopyID: &copyID, Outcome: "applied"}}
			if index == 0 || index == 4 {
				_, err = offerings.ReleaseTraegerOfferings(ctx, own, phase, 0)
				require.NoError(t, err)
				counter.Reset()
				require.NoError(t, offerings.RecordTraegerOfferingOutcomes(ctx, own, outcomes))
				assert.Equal(t, 2, counter.Total(), "validation and persistence are independent of offering count")
			}
		}
		for id, schools := range outcomes {
			counter.Reset()
			err := offerings.RecordTraegerOfferingOutcomes(ctx, own, map[int64][]careplan.TraegerOfferingOutcome{id: {schools[0], schools[0]}})
			assert.ErrorIs(t, err, careplan.ErrCareOfferingConfigInvalid)
			assert.Zero(t, counter.Total(), "duplicate schools fail before persistence")
			require.NoError(t, offerings.RecordTraegerOfferingOutcomes(ctx, own, map[int64][]careplan.TraegerOfferingOutcome{id: {{SchoolID: school, Outcome: "rejected"}}}), "a financial refusal keeps the prior baseline")
			changed := traegerOfferingDraft("New terms", 30)
			changed.Prices[0].MonthlyCents = 1800
			_, err = offerings.UpdateTraegerOffering(ctx, own, id, changed)
			require.NoError(t, err)
		}
		_, err := offerings.ReleaseTraegerOfferings(ctx, own, phase, 0)
		return err
	}))
	require.NoError(t, testpkg.WithinTraeger(ctx, own, func(ctx context.Context) error {
		rows, err := offerings.ListTraegerOfferings(ctx, own, phase)
		require.NoError(t, err)
		require.Len(t, rows, 5)
		for _, row := range rows {
			require.Len(t, row.Schools, 1)
			require.NotNil(t, row.Schools[0].Applied)
			assert.Equal(t, 1, *row.Schools[0].AppliedVersion)
			require.NotNil(t, row.Schools[0].AppliedCopyID)
			assert.Equal(t, copiedIDs[row.ID], *row.Schools[0].AppliedCopyID)
			assert.Equal(t, int64(1600), row.Schools[0].Applied.Prices[0].MonthlyCents)
			assert.Equal(t, int64(1800), row.Released.Prices[0].MonthlyCents)
		}
		withdrawnID := rows[0].ID
		require.NoError(t, offerings.RecordTraegerOfferingOutcomes(ctx, own, map[int64][]careplan.TraegerOfferingOutcome{withdrawnID: {{SchoolID: school, Outcome: "withdrawn"}}}))
		withdrawn, err := offerings.TraegerOffering(ctx, own, withdrawnID)
		require.NoError(t, err)
		assert.Nil(t, withdrawn.Schools[0].AppliedVersion)
		assert.Nil(t, withdrawn.Schools[0].AppliedCopyID)
		return nil
	}))
}

func TestTraegerOfferingBatchKeepsDeviationsAndBookedWithdrawals(t *testing.T) {
	t.Parallel()
	db, _, phase := setupCareTest(t)
	catalog := testCareOfferingCatalog(t, db)
	batchCommand, ok := catalog.(careplan.TraegerOfferingBatch)
	require.True(t, ok)
	ctx := testpkg.Ctx(t)
	sources := []int64{testpkg.UniqueTestTenantID(t), testpkg.UniqueTestTenantID(t)}
	inputs := []careplan.TraegerOfferingCopy{
		{SourceID: sources[0], Version: 1, PhaseID: phase.ID, Fields: traegerOfferingDraft("Früh", 20).Fields},
		{SourceID: sources[1], Version: 1, PhaseID: phase.ID, Fields: traegerOfferingDraft("Spät", 20).Fields},
	}
	copyIDs := map[int64]int64{}
	require.NoError(t, fixtureEnrollmentCommand(ctx, func(ctx context.Context) error {
		batch, err := batchCommand.ApplyTraegerOfferings(ctx, inputs, nil)
		if err != nil {
			return err
		}
		for _, row := range batch.Applied {
			copyIDs[row.SourceID] = row.CopyID
		}
		return nil
	}))
	require.Len(t, copyIDs, 2)
	bookTraegerCopy(t, db, phase.ID, copyIDs[sources[1]])
	require.NoError(t, fixtureEnrollmentCommand(ctx, func(ctx context.Context) error {
		row, err := catalog.FindOffering(ctx, copyIDs[sources[0]])
		if err != nil {
			return err
		}
		capacity := 25
		row.Capacity, row.IsActive = &capacity, false
		_, err = catalog.UpdateOffering(ctx, row)
		return err
	}))
	for index := range inputs {
		capacity := 30
		inputs[index].Version, inputs[index].Fields.Capacity = 2, &capacity
	}
	require.NoError(t, fixtureEnrollmentCommand(ctx, func(ctx context.Context) error {
		_, err := batchCommand.ApplyTraegerOfferings(ctx, inputs, nil)
		return err
	}))
	row, err := catalog.FindOffering(ctx, copyIDs[sources[0]])
	require.NoError(t, err)
	require.NotNil(t, row.Capacity)
	assert.Equal(t, 25, *row.Capacity)
	assert.False(t, row.IsActive)
	require.NotNil(t, row.Traeger)
	assert.Equal(t, 2, row.Traeger.Version)
	assert.Equal(t, []string{"capacity", "is_active"}, row.Traeger.Deviations)
	require.NoError(t, fixtureEnrollmentCommand(ctx, func(ctx context.Context) error {
		batch, err := batchCommand.ApplyTraegerOfferings(ctx, nil, sources)
		if err != nil {
			return err
		}
		assert.Equal(t, careplan.TraegerOfferingWithdrawal{Found: true, Deleted: true}, batch.Withdrawn[sources[0]])
		assert.Equal(t, careplan.TraegerOfferingWithdrawal{Found: true}, batch.Withdrawn[sources[1]])
		return nil
	}))
	_, err = catalog.FindOffering(ctx, copyIDs[sources[0]])
	assert.ErrorIs(t, err, careplan.ErrCareOfferingNotFound)
	row, err = catalog.FindOffering(ctx, copyIDs[sources[1]])
	require.NoError(t, err)
	assert.False(t, row.IsActive)
	assert.Nil(t, row.Traeger)
}

func TestTraegerOfferingBatchPrunesWithdrawnAutomaticTriggers(t *testing.T) {
	t.Parallel()
	db, _, phase := setupCareTest(t)
	catalog := testCareOfferingCatalog(t, db)
	batchCommand, ok := catalog.(careplan.TraegerOfferingBatch)
	require.True(t, ok)
	ctx := testpkg.Ctx(t)
	first, second := testpkg.UniqueTestTenantID(t), testpkg.UniqueTestTenantID(t)
	fields := traegerOfferingDraft("Automatisch", 20).Fields
	fields.DaysOfWeekMode = "parent_choice"
	inputs := []careplan.TraegerOfferingCopy{
		{SourceID: first, Version: 1, PhaseID: phase.ID, Fields: traegerOfferingDraft("Auslöser", 20).Fields},
		{SourceID: second, Version: 1, PhaseID: phase.ID, Fields: fields},
	}
	copies := map[int64]int64{}
	require.NoError(t, fixtureEnrollmentCommand(ctx, func(ctx context.Context) error {
		batch, err := batchCommand.ApplyTraegerOfferings(ctx, inputs, nil)
		if err != nil {
			return err
		}
		for _, row := range batch.Applied {
			copies[row.SourceID] = row.CopyID
		}
		row, err := catalog.FindOffering(ctx, copies[second])
		if err != nil {
			return err
		}
		row.AutoAddTriggerOfferingIDs = []int64{copies[first]}
		_, err = catalog.UpdateOffering(ctx, row)
		return err
	}))
	require.NoError(t, fixtureEnrollmentCommand(ctx, func(ctx context.Context) error {
		_, err := batchCommand.ApplyTraegerOfferings(ctx, inputs[1:], []int64{first})
		return err
	}))
	row, err := catalog.FindOffering(ctx, copies[second])
	require.NoError(t, err)
	assert.Empty(t, row.AutoAddTriggerOfferingIDs)
	_, err = catalog.FindOffering(ctx, copies[first])
	assert.ErrorIs(t, err, careplan.ErrCareOfferingNotFound)
}

func TestTraegerOfferingRefusesRemovalOfReleasedFinancialTerms(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	own, phase := newOfferingTraeger(t, "Veröffentlichte Konditionen")
	offerings := careplanCompose.NewTraegerOfferings()
	ctx := testpkg.ContextWithTenantRuntime(t.Context(), testpkg.TenantRuntime(t, db))
	draft := traegerOfferingDraft("Mit Vertrag", 20)
	draft.ContractTemplate = &careplan.TraegerContractTemplate{TermKind: "school_year", NoticeTo: "month_end"}
	var offeringID int64
	require.NoError(t, testpkg.WithinTraeger(ctx, own, func(ctx context.Context) error {
		created, err := offerings.CreateTraegerOffering(ctx, own, phase, draft)
		if err != nil {
			return err
		}
		offeringID = created.ID
		_, err = offerings.ReleaseTraegerOfferings(ctx, own, phase, 0)
		if err != nil {
			return err
		}
		withoutPrices := draft
		withoutPrices.Prices = nil
		_, err = offerings.UpdateTraegerOffering(ctx, own, created.ID, withoutPrices)
		assert.ErrorIs(t, err, careplan.ErrCareOfferingConfigInvalid, "released prices cannot disappear silently")
		withoutTemplate := draft
		withoutTemplate.ContractTemplate = nil
		_, err = offerings.UpdateTraegerOffering(ctx, own, created.ID, withoutTemplate)
		assert.ErrorIs(t, err, careplan.ErrCareOfferingConfigInvalid, "released templates cannot disappear silently")
		return nil
	}))
	for _, clear := range []string{"prices", "template"} {
		legacy := draft
		if clear == "prices" {
			legacy.Prices = nil
		} else {
			legacy.ContractTemplate = nil
		}
		encoded, err := json.Marshal(legacy)
		require.NoError(t, err)
		// Simulate a draft stored before removal validation was installed.
		_, err = db.ExecContext(t.Context(), `UPDATE enrollment.traeger_care_offerings SET draft=?::jsonb WHERE organization_id=? AND id=?`, string(encoded), own, offeringID)
		require.NoError(t, err)
		err = testpkg.WithinTraeger(ctx, own, func(ctx context.Context) error {
			_, err := offerings.ReleaseTraegerOfferings(ctx, own, phase, 0)
			return err
		})
		require.ErrorIs(t, err, careplan.ErrCareOfferingConfigInvalid, "release rejects removal of %s from a legacy draft", clear)
	}
}
