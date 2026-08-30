package clientip

import "net"

// ParseIPString parses a client IP string, preserving nil for empty or invalid
// inputs so callers do not persist malformed inet values.
func ParseIPString(ipAddress string) net.IP {
	if ipAddress == "" {
		return nil
	}
	return net.ParseIP(ipAddress)
}
