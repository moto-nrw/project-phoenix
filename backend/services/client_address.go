package services

import "net"

// parseClientIP wraps net.ParseIP with the empty-string guard so the audit
// rows the compositions write never get a malformed inet value (#3364).
func parseClientIP(address string) net.IP {
	if address == "" {
		return nil
	}
	return net.ParseIP(address)
}
