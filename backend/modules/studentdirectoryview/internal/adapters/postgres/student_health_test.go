package studentdirectoryview_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/studentdirectoryview/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// The health field of a child opens only for the care row it was sealed to
// (#5472), and a reader without a key ring neither seals nor opens one.
func TestStudentHealthOpensOnlyForItsCareRow(t *testing.T) {
	t.Parallel()
	reader := compose.NewReader(testpkg.TestHealthDataCipher)
	value := "Erdnussallergie"
	sealed, err := reader.SealStudentHealth(7, 11, &value)
	require.NoError(t, err)
	require.NotContains(t, string(sealed), value)

	opened, err := reader.OpenStudentHealth(7, 11, sealed)
	require.NoError(t, err)
	require.Equal(t, value, *opened)

	_, err = reader.OpenStudentHealth(7, 12, sealed)
	require.ErrorContains(t, err, "student health information does not open", "another care row")
	_, err = reader.OpenStudentHealth(8, 11, sealed)
	require.ErrorContains(t, err, "student health information does not open", "another school")

	none, err := reader.SealStudentHealth(7, 11, nil)
	require.NoError(t, err)
	require.Nil(t, none, "no health field stays NULL")
	empty, err := reader.OpenStudentHealth(7, 11, nil)
	require.NoError(t, err)
	require.Nil(t, empty)

	keyless := compose.NewReader(nil)
	_, err = keyless.SealStudentHealth(7, 11, &value)
	require.Error(t, err)
	_, err = keyless.OpenStudentHealth(7, 11, sealed)
	require.Error(t, err)
}
