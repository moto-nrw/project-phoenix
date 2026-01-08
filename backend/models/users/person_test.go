package users

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
)

func TestPerson_Validate(t *testing.T) {
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
				AccountID: int64Ptr(1),
				TagID:     stringPtr("tag123"),
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

func TestPerson_GetFullName(t *testing.T) {
	tests := []struct {
		name     string
		person   *Person
		expected string
	}{
		{
			name: "standard names",
			person: &Person{
				FirstName: "John",
				LastName:  "Doe",
			},
			expected: "John Doe",
		},
		{
			name: "single character names",
			person: &Person{
				FirstName: "J",
				LastName:  "D",
			},
			expected: "J D",
		},
		{
			name: "names with spaces",
			person: &Person{
				FirstName: "Mary Ann",
				LastName:  "Van Der Berg",
			},
			expected: "Mary Ann Van Der Berg",
		},
		{
			name: "empty names",
			person: &Person{
				FirstName: "",
				LastName:  "",
			},
			expected: " ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.person.GetFullName()
			if got != tt.expected {
				t.Errorf("Person.GetFullName() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestPerson_SetAccount(t *testing.T) {
	t.Run("set account", func(t *testing.T) {
		person := &Person{
			FirstName: "John",
			LastName:  "Doe",
		}

		account := &auth.Account{
			Model: base.Model{ID: 42},
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

func TestPerson_SetRFIDCard(t *testing.T) {
	t.Run("set RFID card", func(t *testing.T) {
		person := &Person{
			FirstName: "John",
			LastName:  "Doe",
		}

		card := &RFIDCard{
			StringIDModel: base.StringIDModel{ID: "RFID123456AB"},
		}

		person.SetRFIDCard(card)

		if person.RFIDCard != card {
			t.Error("Person.SetRFIDCard() did not set RFIDCard reference")
		}

		if person.TagID == nil || *person.TagID != "RFID123456AB" {
			t.Errorf("Person.SetRFIDCard() did not set TagID, got %v", person.TagID)
		}
	})

	t.Run("set nil RFID card", func(t *testing.T) {
		tagID := "RFID123456AB"
		person := &Person{
			FirstName: "John",
			LastName:  "Doe",
			TagID:     &tagID,
		}

		person.SetRFIDCard(nil)

		if person.RFIDCard != nil {
			t.Error("Person.SetRFIDCard(nil) did not clear RFIDCard reference")
		}

		if person.TagID != nil {
			t.Errorf("Person.SetRFIDCard(nil) did not clear TagID, got %v", person.TagID)
		}
	})
}

func TestPerson_HasRFIDCard(t *testing.T) {
	tests := []struct {
		name     string
		person   *Person
		expected bool
	}{
		{
			name: "has RFID card",
			person: &Person{
				FirstName: "John",
				LastName:  "Doe",
				TagID:     stringPtr("RFID-123"),
			},
			expected: true,
		},
		{
			name: "no RFID card (nil)",
			person: &Person{
				FirstName: "John",
				LastName:  "Doe",
				TagID:     nil,
			},
			expected: false,
		},
		{
			name: "empty RFID card",
			person: &Person{
				FirstName: "John",
				LastName:  "Doe",
				TagID:     stringPtr(""),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.person.HasRFIDCard()
			if got != tt.expected {
				t.Errorf("Person.HasRFIDCard() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPerson_HasAccount(t *testing.T) {
	tests := []struct {
		name     string
		person   *Person
		expected bool
	}{
		{
			name: "has account",
			person: &Person{
				FirstName: "John",
				LastName:  "Doe",
				AccountID: int64Ptr(1),
			},
			expected: true,
		},
		{
			name: "no account (nil)",
			person: &Person{
				FirstName: "John",
				LastName:  "Doe",
				AccountID: nil,
			},
			expected: false,
		},
		{
			name: "zero account ID",
			person: &Person{
				FirstName: "John",
				LastName:  "Doe",
				AccountID: int64Ptr(0),
			},
			expected: false,
		},
		{
			name: "negative account ID",
			person: &Person{
				FirstName: "John",
				LastName:  "Doe",
				AccountID: int64Ptr(-1),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.person.HasAccount()
			if got != tt.expected {
				t.Errorf("Person.HasAccount() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPerson_TableName(t *testing.T) {
	person := &Person{}
	expected := "users.persons"

	got := person.TableName()
	if got != expected {
		t.Errorf("Person.TableName() = %q, want %q", got, expected)
	}
}

func TestPerson_EntityInterface(t *testing.T) {
	now := time.Now()
	person := &Person{
		Model: base.Model{
			ID:        123,
			CreatedAt: now,
			UpdatedAt: now.Add(time.Hour),
		},
		FirstName: "John",
		LastName:  "Doe",
	}

	t.Run("GetID", func(t *testing.T) {
		got := person.GetID()
		if got != int64(123) {
			t.Errorf("Person.GetID() = %v, want %v", got, int64(123))
		}
	})

	t.Run("GetCreatedAt", func(t *testing.T) {
		got := person.GetCreatedAt()
		if !got.Equal(now) {
			t.Errorf("Person.GetCreatedAt() = %v, want %v", got, now)
		}
	})

	t.Run("GetUpdatedAt", func(t *testing.T) {
		expected := now.Add(time.Hour)
		got := person.GetUpdatedAt()
		if !got.Equal(expected) {
			t.Errorf("Person.GetUpdatedAt() = %v, want %v", got, expected)
		}
	})
}

// Helper functions for creating pointers
func stringPtr(s string) *string {
	return &s
}

func int64Ptr(i int64) *int64 {
	return &i
}
