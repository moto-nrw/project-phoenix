//go:build windows

package querybudgets

import (
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

var budgetLockFileEx = syscall.NewLazyDLL("kernel32.dll").NewProc("LockFileEx")

func lockBudgetReport(file *os.File) error {
	// All writers exclusively lock byte zero. LockFileEx blocks on the
	// synchronous file handle; closing that handle releases the lock.
	const exclusiveLock = 2
	var overlapped syscall.Overlapped
	result, _, err := budgetLockFileEx.Call(file.Fd(), exclusiveLock, 0, 1, 0, uintptr(unsafe.Pointer(&overlapped)))
	runtime.KeepAlive(&overlapped)
	if result == 0 {
		return err
	}
	return nil
}
