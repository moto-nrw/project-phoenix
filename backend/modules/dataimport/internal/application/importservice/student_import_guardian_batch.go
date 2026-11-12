package importpkg

import (
	"strings"

	importModels "github.com/moto-ogs/project-phoenix/modules/dataimport"
	"github.com/moto-ogs/project-phoenix/modules/dataimport/ports"
)

// Step 5c of #5552 (ADR 0067 addendum of 2026-11-08): in a batch segment, a
// row whose guardians are all new by construction has them written after the
// rows, one statement per table (profile, phones, relationship with the
// retention reset, pickup permission, portal access). A guardian without
// e-mail is always new on the row path (CreateGuardian reuses by e-mail
// only), and so it touches only rows of its own transaction. Every other row
// keeps its guardians in place: an e-mail guardian may be reused or shared,
// and a second primary guardian or a later primary phone demotes an earlier
// one, which the single commands do in their order.

// guardianBatchRow reports whether the row's guardians go into the batch:
// none carries an e-mail, at most one is primary, and no phone after a
// guardian's first written phone asks to be primary.
func guardianBatchRow(row importModels.StudentImportRow) bool {
	primaries := 0
	for _, guardian := range row.Guardians {
		if importModels.OptionalCell(guardian.Email) != nil {
			return false
		}
		if guardian.IsPrimary {
			primaries++
		}
		for i, phone := range writtenGuardianPhones(guardian.PhoneNumbers) {
			if i > 0 && phone.IsPrimary {
				return false
			}
		}
	}
	return primaries <= 1
}

// writtenGuardianPhones are the phones createGuardianPhoneNumbers writes on a
// new profile, in order: no empty number and no number twice.
func writtenGuardianPhones(phones []importModels.PhoneImportData) []importModels.PhoneImportData {
	written := make([]importModels.PhoneImportData, 0, len(phones))
	seen := make(map[string]bool, len(phones))
	for _, phone := range phones {
		if phone.PhoneNumber == "" || seen[phone.PhoneNumber] {
			continue
		}
		seen[phone.PhoneNumber] = true
		written = append(written, phone)
	}
	return written
}

// collectGuardians adds the row's guardians, in their order, as the single
// commands would receive them.
func collectGuardians(studentID int64, row importModels.StudentImportRow, leafs *studentLeafWrites) {
	for _, data := range row.Guardians {
		guardian := ports.NewStudentGuardian{
			FirstName: strings.TrimSpace(data.FirstName), LastName: strings.TrimSpace(data.LastName),
			AddressStreet: data.AddressStreet, AddressCity: data.AddressCity, AddressPostalCode: data.AddressPostalCode,
			Notes: data.Notes, LanguagePreference: guardianLanguagePreference(data.LanguagePreference),
			RelationshipType: ports.MapRelationshipType(data.RelationshipType),
			IsPrimary:        data.IsPrimary, IsEmergencyContact: data.IsEmergencyContact, CanPickup: data.CanPickup,
			PickupNotes: data.PickupNotes, EmergencyPriority: max(1, data.EmergencyPriority),
		}
		if role, ok := importModels.MapGuardianRole(data.GuardianRole); ok && role != "" {
			guardian.GuardianRole = role
		}
		for _, phone := range writtenGuardianPhones(data.PhoneNumbers) {
			guardian.PhoneNumbers = append(guardian.PhoneNumbers, ports.NewGuardianPhone{
				PhoneNumber: phone.PhoneNumber, PhoneType: mapPhoneType(phone.PhoneType), Label: phone.Label, IsPrimary: phone.IsPrimary,
			})
		}
		leafs.guardians = append(leafs.guardians, ports.NewStudentGuardianLink{StudentID: studentID, Guardian: guardian})
	}
}
