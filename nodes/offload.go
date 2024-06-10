//go:build linux
// +build linux

package nodes

import (
	"log/slog"
	"os/exec"
)

func DisableOffload(logger *slog.Logger) {
	/*
	   ethtool -K eth0 tso off
	   ethtool -K eth0 gso off
	   ethtool -K eth0 gro off
	   ethtool -K eth0 lro off
	   ethtool -K eth0 rx-gro-hw off
	*/

	var eth0 = "eth0" // todo: optimize
	var keys = []string{"tso", "gso", "gro", "lro", "rx-gro-hw"}

	for _, key := range keys {
		cmd := exec.Command("ethtook", "-K", eth0, key, "off")
		out, err := cmd.CombinedOutput()
		if cmd.ProcessState.ExitCode() != 0 || err != nil {
			logger.Error("disable offload", slog.String("command", cmd.String()), slog.String("output", string(out)))
		}
	}
}
