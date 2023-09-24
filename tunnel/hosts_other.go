//go:build !windows
// +build !windows

package tunnel

const (
	HostsFile = "/etc/hosts"
	LineSep   = "\n"
)
