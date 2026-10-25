package compose

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

type recordingResetDelivery struct {
	records identityaccess.PasswordResetDeliveryRecorder
}

func (recordingResetDelivery) DispatchPasswordReset(context.Context, identityaccess.PasswordResetLink, string, identityaccess.PasswordResetScope, time.Duration) error {

	return nil
}

type resetPasswords struct {
	identityPorts.PasswordPolicy
}

// The reset delivery built by DeliveryFor gets a recorder that answers like
// the module it is composed into, so it needs no reference back to it; the
// factory runs once, and it excludes a delivery value.
func TestPasswordResetDeliveryFor_RecordsLikeTheModule(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	compose := func(resets *PasswordResetDependencies) (identityaccess.Owner, error) {
		return New(Dependencies{
			DB: db, Observe: func(Observation) {}, TenantRuntime: testpkg.TenantRuntime(t, db), Resets: resets,
			Sessions: &identityPorts.SessionDependencies{
				PasswordLoginPolicy: func() (int, time.Duration, error) { return 5, time.Minute, nil },
				Schools:             sweepSchools{}, Persons: sweepPersons{}, Passwords: sweepPasswords{},
				Codec: sweepCodec{}, MFALock: sweepMFALock{}, Audit: sweepAudit{}, Push: sweepPush{}, PortalModules: application.UnbookedPortals{},
			},
		})
	}

	var built []recordingResetDelivery
	module, err := compose(&PasswordResetDependencies{
		Passwords: resetPasswords{}, Expiry: time.Hour,
		DeliveryFor: func(records identityaccess.PasswordResetDeliveryRecorder) identityPorts.PasswordResetDelivery {
			built = append(built, recordingResetDelivery{records: records})
			return built[len(built)-1]
		},
	})
	require.NoError(t, err)
	require.Len(t, built, 1)
	require.NotNil(t, built[0].records)

	canceled, cancel := context.WithCancel(testpkg.Ctx(t))
	cancel()
	resetID := int64(len(t.Name()))
	delivery := identityaccess.TokenDelivery{RetryCount: 1}
	viaRecorder := built[0].records.RecordPasswordResetDelivery(canceled, resetID, delivery)
	viaModule := module.RecordPasswordResetDelivery(canceled, resetID, delivery)
	require.Error(t, viaRecorder)
	require.Equal(t, viaModule.Error(), viaRecorder.Error())
	require.NoError(t, built[0].records.RecordPasswordResetDelivery(testpkg.Ctx(t), resetID, delivery))

	_, err = compose(&PasswordResetDependencies{Passwords: resetPasswords{}, Expiry: time.Hour})
	require.ErrorContains(t, err, "every password reset dependency is required")
}
