package students

import (
	context "context"
	errors "errors"
	slog "log/slog"
	http "net/http"

	common "github.com/moto-nrw/project-phoenix/api/common"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// FamilyProtectionCapability is the People Directory owner surface behind the
// per-child privacy ledger: the current flag and the append-only change.
type FamilyProtectionCapability interface {
	peopleModule.FamilyProtectionQuery
	peopleModule.FamilyProtectionCommand
}

// StudentNotesCapability is the People Directory owner surface behind a
// child's note card (#3632): the audience-filtered timeline and the writes.
// Author names arrive resolved on the notes, so this resource never joins
// accounts to persons itself.
type StudentNotesCapability interface {
	peopleModule.StudentNoteQuery
	peopleModule.StudentNoteCommand
}

// ClassListEntryReader hands over the entries in the class-then-name display
// order the "Klassenliste" export and the class dropdown both rely on. The
// root binds it to the School Membership capability that owns them.
type ClassListEntryReader interface {
	ListClassListEntriesInDisplayOrder(context.Context) ([]peopleModule.StudentClassListEntry, error)
}

// ChildQuotaUsage is the school's Kinderkontingent (Booked) next to its
// Kontingentzahl (Occupied), as the Datenverwaltung shows it (#3569).
type ChildQuotaUsage struct {
	Booked   int
	Occupied int
}

// ChildQuotaReader reads the Kinderkontingent of the caller's school. The
// root binds it to the School Membership capability that counts the
// Kontingentzahl; limited is false when the school has no Kinderkontingent.
type ChildQuotaReader interface {
	ChildQuotaUsage(context.Context) (usage ChildQuotaUsage, limited bool, err error)
}

// DeviceIdentity reports the device id of the kiosk the device authenticator
// admitted on this request, or false when none did.
type DeviceIdentity func(context.Context) (deviceID string, ok bool)

// requiredDeviceAuthenticator returns the configured device authenticator, or
// a fail-closed one when the composition supplied none: a bare handler test or
// a broken graph logs the misconfiguration and never serves the RFID routes
// unauthenticated.
func (rs *Resource) requiredDeviceAuthenticator() common.Middleware {
	if rs.DeviceAuthenticator != nil {
		return rs.DeviceAuthenticator
	}
	rs.getLogger().Error("device route group mounted without an authenticator; every request is rejected",
		slog.String("middleware", "DeviceAuthenticator"),
	)
	return func(http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			renderError(w, r, common.ErrorUnauthorized(errors.New("device API key is required")))
		})
	}
}
