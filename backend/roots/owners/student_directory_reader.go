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
// The key ring is required infrastructure: a composition without it, or with
// a malformed one, stops at once, as the serving roots and migrate do.
func StudentDirectoryReader() studentDirectoryView.Reader {
	keys := strings.TrimSpace(viper.GetString("health_data_keys"))
	if keys == "" {
		panic("student directory reader: HEALTH_DATA_KEYS is required")
	}
	cipher, err := securityRuntimeCompose.NewFieldCipher(keys)
	if err != nil {
		panic(fmt.Sprintf("student directory reader: HEALTH_DATA_KEYS: %v", err))
	}
	return studentDirectoryView.NewReader(cipher)
}
