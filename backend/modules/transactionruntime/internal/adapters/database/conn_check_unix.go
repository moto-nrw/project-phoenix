//go:build linux || darwin

package database

import (
	"errors"
	"net"
	"syscall"
)

// socketIdle reports whether conn has neither pending bytes nor EOF.
func socketIdle(conn net.Conn) bool {
	sysConn, ok := conn.(syscall.Conn)
	if !ok {
		return true
	}
	raw, err := sysConn.SyscallConn()
	if err != nil {
		return false
	}
	idle := false
	err = raw.Read(func(fd uintptr) bool {
		var buf [1]byte
		// Pending data or EOF (n >= 0) ends the session; only "nothing to
		// read yet" keeps it.
		_, _, recvErr := syscall.Recvfrom(int(fd), buf[:], syscall.MSG_PEEK|syscall.MSG_DONTWAIT)
		idle = errors.Is(recvErr, syscall.EAGAIN) || errors.Is(recvErr, syscall.EINTR)
		return true
	})
	return err == nil && idle
}
