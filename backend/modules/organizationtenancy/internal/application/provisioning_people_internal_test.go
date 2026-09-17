package application

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedPersonListings(h *provisioningHarness) {
	seedDeviceSchools(h)
	h.addSchool(50, 1, "Geloescht", "geloescht").DeletedAt = deletedNow()
	h.people.listings = []domain.PersonListing{
		{ID: 1, TenantID: 10, FirstName: "Ada", LastName: "Lovelace", HasAccount: true, AccountEmail: strPtr("ada@example.test"), IsStaff: true, CreatedAt: fixedTime},
		{ID: 2, TenantID: 20, FirstName: "Bob", LastName: "Builder", HasRFIDCard: true, IsStudent: true, CreatedAt: fixedTime},
		{ID: 3, TenantID: 30, FirstName: "Cem", LastName: "Other", CreatedAt: fixedTime},
		{ID: 4, TenantID: 50, FirstName: "Dora", LastName: "Deleted", CreatedAt: fixedTime},
	}
}

func TestProvisioningListSchoolPersons(t *testing.T) {
	t.Parallel()

	t.Run("lists the school's persons with context", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seedPersonListings(h)

		persons, err := h.svc.ListSchoolPersons(context.Background(), 10)

		require.NoError(t, err)
		assert.Equal(t, []organizationtenancy.OperatorPerson{{
			ID: 1, FirstName: "Ada", LastName: "Lovelace", HasAccount: true, AccountEmail: strPtr("ada@example.test"),
			IsStaff: true, SchoolID: 10, SchoolName: "Burbach", OrganizationID: 1, OrganizationName: "Talent OGS",
			CreatedAt: fixedTime,
		}}, persons)
		assert.Equal(t, [][]int64{{10}}, h.people.listedTenants)
	})

	t.Run("a deleted school lists nobody", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seedPersonListings(h)

		persons, err := h.svc.ListSchoolPersons(context.Background(), 50)

		require.NoError(t, err)
		assert.NotNil(t, persons)
		assert.Empty(t, persons)
		assert.Empty(t, h.people.listedTenants)
	})

	for name, tc := range map[string]struct {
		seed  func(*provisioningHarness)
		check func(*testing.T, error)
	}{
		"missing school": {
			check: func(t *testing.T, err error) {
				var notFound *organizationtenancy.SchoolNotFoundError
				require.ErrorAs(t, err, &notFound)
				assert.Equal(t, int64(10), notFound.SchoolID)
			},
		},
		"school lookup failure": {
			seed: func(h *provisioningHarness) {
				seedPersonListings(h)
				h.engine.fail["FindSchoolByID"] = errBoom
			},
			check: func(t *testing.T, err error) { require.ErrorIs(t, err, errBoom) },
		},
		"school rows failure": {
			seed: func(h *provisioningHarness) {
				seedPersonListings(h)
				h.dashboard.fail["SchoolSummaries"] = errBoom
			},
			check: func(t *testing.T, err error) { require.ErrorIs(t, err, errBoom) },
		},
		"person listing failure": {
			seed: func(h *provisioningHarness) {
				seedPersonListings(h)
				h.people.fail["ListPersons"] = errBoom
			},
			check: func(t *testing.T, err error) { require.ErrorIs(t, err, errBoom) },
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			if tc.seed != nil {
				tc.seed(h)
			}

			persons, err := h.svc.ListSchoolPersons(context.Background(), 10)

			assert.Nil(t, persons)
			tc.check(t, err)
		})
	}
}

func TestProvisioningListOrganizationPersons(t *testing.T) {
	t.Parallel()

	t.Run("lists the persons of the organisation's live schools", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seedPersonListings(h)

		persons, err := h.svc.ListOrganizationPersons(context.Background(), 1)

		require.NoError(t, err)
		require.Len(t, persons, 2)
		assert.Equal(t, int64(1), persons[0].ID)
		assert.Equal(t, "Burbach", persons[0].SchoolName)
		assert.Equal(t, int64(2), persons[1].ID)
		assert.Equal(t, "Walbach", persons[1].SchoolName)
		assert.True(t, persons[1].HasRFIDCard)
		assert.True(t, persons[1].IsStudent)
		assert.Equal(t, "Talent OGS", persons[1].OrganizationName)
		assert.Equal(t, [][]int64{{10, 20}}, h.people.listedTenants, "the deleted school is not queried")
		require.Len(t, h.dashboard.schoolFilter, 1)
		require.NotNil(t, h.dashboard.schoolFilter[0])
		assert.Equal(t, int64(1), *h.dashboard.schoolFilter[0])
	})

	for name, tc := range map[string]struct {
		seed  func(*provisioningHarness)
		check func(*testing.T, error)
	}{
		"missing organisation": {
			check: func(t *testing.T, err error) {
				var notFound *organizationtenancy.OrganizationNotFoundError
				require.ErrorAs(t, err, &notFound)
				assert.Equal(t, int64(1), notFound.OrganizationID)
			},
		},
		"organisation lookup failure": {
			seed: func(h *provisioningHarness) {
				seedPersonListings(h)
				h.engine.fail["FindByID"] = errBoom
			},
			check: func(t *testing.T, err error) { require.ErrorIs(t, err, errBoom) },
		},
		"school rows failure": {
			seed: func(h *provisioningHarness) {
				seedPersonListings(h)
				h.dashboard.fail["SchoolSummaries"] = errBoom
			},
			check: func(t *testing.T, err error) { require.ErrorIs(t, err, errBoom) },
		},
		"person listing failure": {
			seed: func(h *provisioningHarness) {
				seedPersonListings(h)
				h.people.fail["ListPersons"] = errBoom
			},
			check: func(t *testing.T, err error) { require.ErrorIs(t, err, errBoom) },
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			if tc.seed != nil {
				tc.seed(h)
			}

			persons, err := h.svc.ListOrganizationPersons(context.Background(), 1)

			assert.Nil(t, persons)
			tc.check(t, err)
		})
	}
}

func TestProvisioningSoftDeletePerson(t *testing.T) {
	t.Parallel()

	const personID int64 = 42
	seedStaffWithAccount := func(h *provisioningHarness) {
		h.people.persons[personID] = domain.Person{ID: personID, TenantID: 10, AccountID: int64Ptr(100), HasRFIDCard: true}
		h.people.staff[personID] = domain.StaffMember{ID: 300, TenantID: 10}
	}

	t.Run("unlinks, retires the account and anonymises the person", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seedStaffWithAccount(h)

		err := h.svc.SoftDeletePerson(context.Background(), personID, testOperatorID, operatorIP)

		require.NoError(t, err)
		assert.Equal(t, []string{
			"FindPerson", "FindStaff", "UnlinkRFIDCard", "UnlinkAccount", "AnonymizeAndSoftDelete",
		}, h.people.calls)
		assert.Equal(t, []string{"DeactivateAccount", "AnonymizeAccount"}, h.identity.calls)
		assert.Equal(t, []int64{100}, h.identity.deactivated)
		assert.Equal(t, map[int64]string{100: "deleted-42@anonymized.local"}, h.identity.anonymized)
		assert.Equal(t, [][2]int64{{10, 300}}, h.presence.supervisionCalls, "supervisions are read at the staff member's school")
		assert.NotContains(t, h.people.persons, personID)

		entry := h.onlyAudit(t, domain.AuditActionSoftDelete, domain.AuditResourcePerson, personID)
		assert.Equal(t, map[string]any{"person_id": float64(42), "school_id": float64(10)}, auditChanges(t, entry))
		assert.Contains(t, h.logs.String(), "person_soft_deleted")
	})

	t.Run("a person without card, account or staff record is only anonymised", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.people.persons[personID] = domain.Person{ID: personID, TenantID: 20}

		require.NoError(t, h.svc.SoftDeletePerson(context.Background(), personID, testOperatorID, operatorIP))

		assert.Equal(t, []string{"FindPerson", "FindStaff", "AnonymizeAndSoftDelete"}, h.people.calls)
		assert.Empty(t, h.identity.calls)
		assert.Empty(t, h.presence.supervisionCalls)
		entry := h.onlyAudit(t, domain.AuditActionSoftDelete, domain.AuditResourcePerson, personID)
		assert.Equal(t, float64(20), auditChanges(t, entry)["school_id"])
	})

	t.Run("an active supervision blocks the deletion", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seedStaffWithAccount(h)
		h.presence.supervisions[300] = 2

		err := h.svc.SoftDeletePerson(context.Background(), personID, testOperatorID, operatorIP)

		var supervising *organizationtenancy.PersonHasActiveSupervisionsError
		require.ErrorAs(t, err, &supervising)
		assert.Equal(t, personID, supervising.PersonID)
		assert.Equal(t, 2, supervising.Count)
		assert.Equal(t, []string{"FindPerson", "FindStaff"}, h.people.calls)
		assert.Empty(t, h.identity.calls)
		assert.Empty(t, h.audit.entries)
	})

	for name, seed := range map[string]func(*provisioningHarness){
		"supervision lookup failure": func(h *provisioningHarness) { h.presence.supervisionErr = errBoom },
		"staff lookup failure":       func(h *provisioningHarness) { h.people.fail["FindStaff"] = errBoom },
	} {
		t.Run(name+" does not block the deletion", func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			seedStaffWithAccount(h)
			seed(h)

			require.NoError(t, h.svc.SoftDeletePerson(context.Background(), personID, testOperatorID, operatorIP))
			assert.NotContains(t, h.people.persons, personID)
			assert.Len(t, h.audit.entries, 1)
		})
	}

	t.Run("account deactivation failure does not block the deletion", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seedStaffWithAccount(h)
		h.identity.fail["DeactivateAccount"] = errBoom

		require.NoError(t, h.svc.SoftDeletePerson(context.Background(), personID, testOperatorID, operatorIP))
		assert.Equal(t, map[int64]string{100: "deleted-42@anonymized.local"}, h.identity.anonymized)
		assert.Contains(t, h.logs.String(), "soft_delete_account_deactivation_failed")
		assert.NotContains(t, h.people.persons, personID)
	})

	t.Run("audit failure does not fail the deletion", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		seedStaffWithAccount(h)
		h.audit.err = errBoom

		require.NoError(t, h.svc.SoftDeletePerson(context.Background(), personID, testOperatorID, operatorIP))
	})

	for name, tc := range map[string]struct {
		fail    func(*provisioningHarness)
		message string
	}{
		"rfid unlink":        {func(h *provisioningHarness) { h.people.fail["UnlinkRFIDCard"] = errBoom }, "SoftDeletePerson: unlink rfid"},
		"account anonymise":  {func(h *provisioningHarness) { h.identity.fail["AnonymizeAccount"] = errBoom }, "SoftDeletePerson: anonymize account"},
		"account unlink":     {func(h *provisioningHarness) { h.people.fail["UnlinkAccount"] = errBoom }, "SoftDeletePerson: unlink account"},
		"person anonymise":   {func(h *provisioningHarness) { h.people.fail["AnonymizeAndSoftDelete"] = errBoom }, "SoftDeletePerson: anonymize and soft delete"},
		"person lookup fail": {func(h *provisioningHarness) { h.people.fail["FindPerson"] = errBoom }, "SoftDeletePerson: find person"},
	} {
		t.Run(name+" failure fails the deletion", func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			seedStaffWithAccount(h)
			tc.fail(h)

			err := h.svc.SoftDeletePerson(context.Background(), personID, testOperatorID, operatorIP)

			require.ErrorIs(t, err, errBoom)
			assert.Contains(t, err.Error(), tc.message)
			require.ErrorIs(t, h.tx.lastErr, errBoom)
			assert.Empty(t, h.audit.entries)
		})
	}

	t.Run("missing person", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)

		err := h.svc.SoftDeletePerson(context.Background(), personID, testOperatorID, operatorIP)

		var notFound *organizationtenancy.PersonNotFoundError
		require.ErrorAs(t, err, &notFound)
		assert.Equal(t, personID, notFound.PersonID)
		assert.Equal(t, []string{"FindPerson"}, h.people.calls)
	})

	for _, id := range []int64{0, -1} {
		t.Run("rejects an invalid id", func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)

			err := h.svc.SoftDeletePerson(context.Background(), id, testOperatorID, operatorIP)

			var invalid *organizationtenancy.InvalidProvisioningDataError
			require.ErrorAs(t, err, &invalid)
			assert.Zero(t, h.tx.adminRuns)
		})
	}
}
