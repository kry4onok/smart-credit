package assistanthelp

import (
	"os"
	"time"
)

// rpcTimeout stays below the 60-second Help context lifetime.
// Deployments increasing it must also allow a longer HTTP write timeout.
// Invalid configuration retains the existing five-second fallback deadline.
func rpcTimeout() time.Duration {
	timeout, err := time.ParseDuration(os.Getenv("ASSISTANT_HELP_TIMEOUT"))
	if err != nil || timeout <= 0 || timeout > 45*time.Second {
		return 5 * time.Second
	}
	return timeout
}
