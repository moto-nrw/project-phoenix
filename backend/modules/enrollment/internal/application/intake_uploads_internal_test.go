package application

import (
	"context"
	"testing"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"

	"github.com/stretchr/testify/require"
)

type uploadGateCatalog struct {
	phase  *enrollmentNative.Phase
	schema *enrollmentNative.FormSchema
}

func (c uploadGateCatalog) Phase(context.Context, int64) (*enrollmentNative.Phase, error) {
	phase := *c.phase
	return &phase, nil
}

func (c uploadGateCatalog) Schema(context.Context, int64) (*enrollmentNative.FormSchema, error) {
	schema := *c.schema
	return &schema, nil
}

func TestUploadField_RequiresPublicGateLateInviteOrEnrolleeAccess(t *testing.T) {
	t.Parallel()

	schemaID := int64(9)
	svc := NewIntake(IntakeDependencies{
		Settings: intakeSettingsFake{enrollmentEnabled: true},
		Catalog: uploadGateCatalog{
			phase: &enrollmentNative.Phase{
				ID: 4, TenantID: 7, IsActive: true, FormSchemaID: &schemaID,
				Audience: enrollmentNative.PhaseAudienceLinkedParents,
			},
			schema: &enrollmentNative.FormSchema{ID: schemaID, Fields: []enrollmentNative.FormField{{
				Key: "proof", Type: enrollmentNative.FormFieldFile,
				FileOptions: &enrollmentNative.FormFileOptions{AllowedTypes: []string{enrollmentNative.FileTypePDF}, MaxSizeMB: 1},
			}}},
		},
		LateInvites: stubAudienceLateInviteRepo{invite: &enrollmentNative.LateInvite{}},
		Fingerprint: testFingerprint,
	})
	input := enrollmentNative.UploadInput{TenantID: 7, PhaseID: 4, FieldKey: "proof"}

	_, err := svc.uploadField(context.Background(), input)
	require.ErrorIs(t, err, enrollmentNative.ErrPhaseAudienceRestricted)

	input.LateInviteToken = "valid-token"
	field, err := svc.uploadField(context.Background(), input)
	require.NoError(t, err)
	require.NotNil(t, field)

	input.LateInviteToken = ""
	input.EnrolleeAccess = enrollmentNative.EnrolleeAudienceAccess{LinkedParents: true}
	field, err = svc.uploadField(context.Background(), input)
	require.NoError(t, err)
	require.NotNil(t, field)
}
