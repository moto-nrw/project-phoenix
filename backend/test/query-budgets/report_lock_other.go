//go:build !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !windows

package querybudgets

import (
	"fmt"
	"os"
	"runtime"
)

func lockBudgetReport(_ *os.File) error {
	return fmt.Errorf("query budget report locking is unsupported on %s", runtime.GOOS)
}
