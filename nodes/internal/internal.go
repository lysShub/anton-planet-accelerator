package internal

import (
	"errors"
	"log/slog"
	"os/exec"
)

func DisableOffload(logger *slog.Logger) error {
	/*
	   ethtool -K eth0 tso off
	   ethtool -K eth0 gso off
	   ethtool -K eth0 gro off
	   ethtool -K eth0 lro off
	   ethtool -K eth0 rx-gro-hw off
	*/

	var eth0 = "eth0" // todo: optimize
	var keys = []string{"tso", "gso", "gro", "lro", "rx-gro-hw"}

	ok := false
	for _, key := range keys {
		cmd := exec.Command("ethtool", "-K", eth0, key, "off")
		out, err := cmd.CombinedOutput()
		if cmd.ProcessState.ExitCode() != 0 || err != nil {
			var attrs = []any{slog.String("command", cmd.String())}
			if len(out) > 0 {
				attrs = append(attrs, slog.String("output", string(out)))
			}
			if err != nil {
				attrs = append(attrs, slog.String("error", err.Error()))
			}
			logger.Error("disable offload", attrs...)
		} else {
			ok = true
		}
	}
	if !ok {
		return errors.New("disable offset failed")
	}
	return nil
}
