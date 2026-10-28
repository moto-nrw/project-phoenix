//go:build !linux && !darwin

package database

import "net"

func socketIdle(net.Conn) bool { return true }
