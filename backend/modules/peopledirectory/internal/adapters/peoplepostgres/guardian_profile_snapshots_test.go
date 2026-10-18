package users

import (
	"errors"
	"reflect"
	"testing"
	"time"

	users "github.com/moto-nrw/project-phoenix/models/users"
	directory "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// The workflow used the retained validator's partially normalized value even
// when validation failed. Replacing it with the command normalizer would lose
// that value and change the error class.
func TestGuardianProfileNormalizationRetainsFailureMutation(t *testing.T) {
	t.Parallel()
	for _, input := range []struct{ name, email, method string }{
		{"invalid email", " BROKEN@ ", "phone"},
		{"invalid method", " MIA@EXAMPLE.TEST ", "letter"},
		{"valid", " MIA@EXAMPLE.TEST ", "phone"},
	} {
		t.Run(input.name, func(t *testing.T) {
			t.Parallel()
			email, wantEmail := input.email, input.email
			value := directory.GuardianProfileSnapshot{FirstName: " Mia ", LastName: " Meier ", Email: &email, PreferredContactMethod: input.method}
			want := users.GuardianProfile{FirstName: " Mia ", LastName: " Meier ", Email: &wantEmail, PreferredContactMethod: input.method}
			wantErr := want.Validate()
			gotErr := (&guardianProfileRepository{}).NormalizeGuardianProfile(&value)
			if (gotErr == nil) != (wantErr == nil) || gotErr != nil && gotErr.Error() != wantErr.Error() {
				t.Fatalf("canonical error changed: got %v, want %v", gotErr, wantErr)
			}
			if value.FirstName != want.FirstName || value.LastName != want.LastName || *value.Email != *want.Email {
				t.Fatalf("normalization on failure changed: got %+v, want %+v", value, want)
			}
			if gotErr != nil && errors.Is(gotErr, directory.ErrInvalidGuardian) {
				t.Fatal("the retained plain validation error acquired a different error class")
			}
		})
	}
}

func TestGuardianProfileSnapshotRetainsMetadataAndLoadedPhones(t *testing.T) {
	t.Parallel()
	text := "kept"
	stamp := time.Date(2026, time.October, 17, 10, 20, 30, 42, time.UTC)
	row := &users.GuardianProfile{FirstName: "Mia", LastName: "Meier", Email: &text, AddressStreet: &text, AddressCity: &text, AddressPostalCode: &text, HasAccount: false, PreferredContactMethod: "phone", LanguagePreference: "de", PortalLocale: &text, Notes: &text}
	row.ID, row.TenantID, row.CreatedAt, row.UpdatedAt = 23, 29, stamp, stamp
	row.AccountID = &row.ID
	phone := &users.GuardianPhoneNumber{GuardianProfileID: row.ID, PhoneNumber: "+49 1234", PhoneType: users.PhoneTypeWork, Label: &text, IsPrimary: true, Priority: 3}
	phone.ID, phone.TenantID, phone.CreatedAt, phone.UpdatedAt = 31, row.TenantID, stamp, stamp
	for _, relation := range [][]*users.GuardianPhoneNumber{nil, {}, {phone, nil}} {
		row.PhoneNumbers = relation
		restored := GuardianProfileRow(NativeGuardianProfile(row))
		if !reflect.DeepEqual(row, restored) {
			t.Fatalf("profile snapshot lost fields or nil semantics: got %+v, want %+v", restored, row)
		}
	}
	if NativeGuardianProfile(nil) != nil {
		t.Fatal("missing profile must stay nil")
	}
}
