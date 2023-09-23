//go:build !windows
// +build !windows

package tunnel

const (
	HostsFile = "/etc/hosts"
	Sep       = "\n"
)
