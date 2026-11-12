package schoolstructurehttp_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/modules/peopledirectory"
	schoolHTTP "github.com/moto-ogs/project-phoenix/modules/schoolstructure/http"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

type groupStudentPeopleProbe struct {
	schoolHTTP.GroupPeople
	list func(context.Context, []int64) (map[int64]peopledirectory.Person, error)
	find func(context.Context, int64) (peopledirectory.Person, error)
}

func (p groupStudentPeopleProbe) ListPersonsByID(ctx context.Context, ids []int64) (map[int64]peopledirectory.Person, error) {
	return p.list(ctx, ids)
}

func (p groupStudentPeopleProbe) FindPerson(ctx context.Context, id int64) (peopledirectory.Person, error) {
	return p.find(ctx, id)
}

func TestGetGroupStudents_BatchedPersons(t *testing.T) {
	t.Parallel()
	for _, variant := range []string{"full", "redacted", "missing", "batch_error", "linked_account_error"} {
		t.Run(variant, func(t *testing.T) {
			t.Parallel()
			testpkg.OwnTenant(t)
			tc, router := setupProtectedRouter(t)
			group := testpkg.CreateTestEducationGroup(t, tc.db, "BatchPersons")
			_, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "Batch", "Reader")
			var expected []schoolHTTP.GroupStudentResponse
			var personIDs []int64
			for i := range 3 {
				lastName := fmt.Sprintf("Child%02d", i)
				child := testpkg.CreateTestStudent(t, tc.db, "Batch", lastName, "B1")
				testpkg.AssignStudentToGroup(t, tc.db, child.ID, group.ID)
				card := testpkg.CreateTestRFIDCard(t, tc.db, "GROUPBATCH")
				testpkg.LinkRFIDToStudent(t, tc.db, child.PersonID, card.ID)
				personIDs = append(personIDs, child.PersonID)
				row := schoolHTTP.GroupStudentResponse{ID: child.ID, PersonID: child.PersonID,
					FirstName: "Batch", LastName: lastName, SchoolClass: "B1", GroupID: group.ID,
					GroupName: group.Name, Location: "Abwesend", TagID: card.ID}
				if variant == "redacted" {
					row.TagID = ""
				}
				if i != 1 || (variant == "full" || variant == "redacted") {
					expected = append(expected, row)
				}
			}
			original := tc.resource.People
			bulkCalls, singleCalls := 0, 0
			tc.resource.People = groupStudentPeopleProbe{
				GroupPeople: original,
				list: func(ctx context.Context, ids []int64) (map[int64]peopledirectory.Person, error) {
					bulkCalls++
					require.ElementsMatch(t, personIDs, ids)
					if variant == "batch_error" {
						return nil, errors.New("batch unavailable")
					}
					persons, err := original.ListPersonsByID(ctx, ids)
					require.NoError(t, err)
					if variant == "missing" {
						delete(persons, personIDs[1])
					}
					if variant == "linked_account_error" {
						person := persons[personIDs[1]]
						person.AccountID = &account.ID
						persons[person.ID] = person
					}
					return persons, nil
				},
				find: func(ctx context.Context, id int64) (peopledirectory.Person, error) {
					singleCalls++
					if id == personIDs[1] {
						return peopledirectory.Person{}, errors.New("person unavailable")
					}
					return original.FindPerson(ctx, id)
				},
			}
			claims := testutil.AdminTestClaimsForTenant(int(account.ID), testpkg.Tenant(t))
			permissions := []string{"groups:read", "admin:*"}
			if variant == "redacted" {
				claims.IsAdmin = false
				permissions = []string{"groups:read"}
			}
			rr := testutil.ExecuteRequest(router, newReq(t, http.MethodGet,
				fmt.Sprintf("/groups/%d/students", group.ID), nil, claims, permissions...))
			require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
			var envelope struct {
				Data    []schoolHTTP.GroupStudentResponse `json:"data"`
				Message string                            `json:"message"`
			}
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &envelope))
			require.Equal(t, expected, envelope.Data)
			require.Equal(t, fmt.Sprintf("Found %d students in group", len(expected)), envelope.Message)
			require.Equal(t, 1, bulkCalls)
			wantSingles := 0
			if variant == "batch_error" {
				wantSingles = len(personIDs)
			}
			if variant == "linked_account_error" {
				wantSingles = 1
			}
			require.Equal(t, wantSingles, singleCalls)
		})
	}
}
