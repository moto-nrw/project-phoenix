package settings

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	configModel "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

func TestBookedModulesFollowsTheModuleList(t *testing.T) {
	t.Parallel()
	off := map[string]bool{configModel.KeyModuleNFCAnwesenheit: true, configModel.KeyModuleVertraege: true}
	booked, err := BookedModules(func(key string) (bool, error) { return !off[key], nil })
	require.NoError(t, err)
	assert.Len(t, booked, len(configModel.Modules())-2)
	assert.NotContains(t, booked, ModuleNFCAnwesenheit)
	assert.Equal(t, ModuleMittagessen, booked[0])

	_, err = BookedModules(func(string) (bool, error) { return false, errors.New("boom") })
	require.Error(t, err)
}
