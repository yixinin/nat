package tunnel

import "nat/stderr"

const (
	NoRecentNetworkActivity = "timeout: no recent network activity"
)

var ErrorTunnelClosed = stderr.New("TunnelClosed", NoRecentNetworkActivity)
