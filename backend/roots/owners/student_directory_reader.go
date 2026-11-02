package owners

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"

	securityRuntimeCompose "github.com/moto-nrw/project-phoenix/modules/securityruntime/compose"
	studentDirectoryView "github.com/moto-nrw/project-phoenix/modules/studentdirectoryview/compose"
)

// StudentDirectoryReader is the student directory projection the roots bind
// to the student directory ports of Care Plan and People Directory
// (ADR 0058). Its rows carry the children's health field sealed (#5472): the
// reader seals and opens it with the key ring HEALTH_DATA_KEYS of the process
// configuration, the key ring of #4722. The root helpers that build a reader
// take only the database, so the reader reads the key ring where it is
// built rather than through every helper's signature.
//
// The serving roots and migrate refuse to start without the key ring
// (newRootEnvironment). A command composed without it refuses every
// non-empty health field instead of reading or writing it in plaintext.
func StudentDirectoryReader() studentDirectoryView.Reader {
	return studentDirectoryView.NewReader(configuredHealthData())
}

// configuredHealthData builds the field cipher over HEALTH_DATA_KEYS, or none
// when the configuration names no key ring. A malformed key ring is a
// configuration error that stops the composition.
func configuredHealthData() studentDirectoryView.HealthCipher {
	keys := strings.TrimSpace(viper.GetString("health_data_keys"))
	if keys == "" {
		return nil
	}
	cipher, err := securityRuntimeCompose.NewFieldCipher(keys)
	if err != nil {
		panic(fmt.Sprintf("student directory reader: HEALTH_DATA_KEYS: %v", err))
	}
	return cipher
}
