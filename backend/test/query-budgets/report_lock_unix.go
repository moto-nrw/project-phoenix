//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package querybudgets

import (
	"os"
	"syscall"
)

func lockBudgetReport(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_EX)
}
