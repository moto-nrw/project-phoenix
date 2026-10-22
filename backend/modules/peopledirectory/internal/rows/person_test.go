package users

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/ptrtest"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestPerson_Validate(t *testing.T) {
	t.Parallel()
	account := testpkg.CreateTestAccount(t, testpkg.SetupTestDB(t), "private-row-account")

	tests := []struct {
		name    string
		person  *Person
		wantErr bool
	}{
		{
			name: "valid person with names only",
			person: &Person{
				FirstName: "John",
				LastName:  "Doe",
			},
			wantErr: false,
		},
		{
			name: "valid person with all fields",
			person: &Person{
				FirstName: "Jane",
				LastName:  "Smith",
				AccountID: ptrtest.Ptr(account.ID),
				TagID:     ptrtest.Ptr("tag123"),
			},
			wantErr: false,
		},
		{
			name: "empty first name",
			person: &Person{
				FirstName: "",
				LastName:  "Doe",
			},
			wantErr: true,
		},
		{
			name: "empty last name",
			person: &Person{
				FirstName: "John",
				LastName:  "",
			},
			wantErr: true,
		},
		{
			name: "both names empty",
			person: &Person{
				FirstName: "",
				LastName:  "",
			},
			wantErr: true,
		},
		{
			name: "whitespace only first name - passes validation then trimmed",
			person: &Person{
				FirstName: "   ",
				LastName:  "Doe",
			},
			wantErr: false, // Note: validation checks empty before trimming
		},
		{
			name: "whitespace only last name - passes validation then trimmed",
			person: &Person{
				FirstName: "John",
				LastName:  "   ",
			},
			wantErr: false, // Note: validation checks empty before trimming
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.person.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Person.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPerson_Validate_TrimSpaces(t *testing.T) {
	t.Parallel()

	person := &Person{
		FirstName: "  John  ",
		LastName:  "  Doe  ",
	}

	err := person.Validate()
	if err != nil {
		t.Fatalf("Person.Validate() unexpected error = %v", err)
	}

	if person.FirstName != "John" {
		t.Errorf("Person.Validate() did not trim FirstName, got %q", person.FirstName)
	}

	if person.LastName != "Doe" {
		t.Errorf("Person.Validate() did not trim LastName, got %q", person.LastName)
	}
}

func TestPerson_SetAccount(t *testing.T) {
	t.Parallel()

	t.Run("set account", func(t *testing.T) {
		person := &Person{
			FirstName: "John",
			LastName:  "Doe",
		}

		account := &PersonAccount{
			ID:    42,
			Email: "john@example.com",
		}

		person.SetAccount(account)

		if person.Account != account {
			t.Error("Person.SetAccount() did not set Account reference")
		}

		if person.AccountID == nil || *person.AccountID != 42 {
			t.Errorf("Person.SetAccount() did not set AccountID, got %v", person.AccountID)
		}
	})

	t.Run("set nil account", func(t *testing.T) {
		accountID := int64(42)
		person := &Person{
			FirstName: "John",
			LastName:  "Doe",
			AccountID: &accountID,
		}

		person.SetAccount(nil)

		if person.Account != nil {
			t.Error("Person.SetAccount(nil) did not clear Account reference")
		}

		if person.AccountID != nil {
			t.Errorf("Person.SetAccount(nil) did not clear AccountID, got %v", person.AccountID)
		}
	})
}
