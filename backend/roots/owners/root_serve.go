package owners

import (
	"fmt"
)

// runRootSteps runs a root's build steps in order and stops at the first
// failure. Each step assigns locals of the root function that declares it,
// so a later step only reads values an earlier one built.
func RunRootSteps(steps ...func() error) error {
	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}
	}
	return nil
}

// stepError names the value a failed root step was building.
func StepError(what string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", what, err)
}
