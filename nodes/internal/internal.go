package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/netip"
	"os/exec"

	"github.com/jftuga/geodist"
	"github.com/pkg/errors"
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

// todo: temp, should from admin
func IPCoord(addr netip.Addr) (geodist.Coord, error) {
	if !addr.Is4() {
		return geodist.Coord{}, errors.New("only support ipv4")
	}

	url := fmt.Sprintf(`http://ip-api.com/json/%s?fields=status,country,lat,lon,query`, addr.String())

	resp, err := http.Get(url)
	if err != nil {
		return geodist.Coord{}, errors.WithStack(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return geodist.Coord{}, errors.Errorf("http code %d", resp.StatusCode)
	}

	var ret = struct {
		Status  string
		Country string
		Lat     float64
		Lon     float64
		Query   string
	}{}
	err = json.NewDecoder(resp.Body).Decode(&ret)
	if err != nil {
		return geodist.Coord{}, err
	}
	if ret.Status != "success" && ret.Query != addr.String() {
		return geodist.Coord{}, errors.Errorf("invalid response %#v", ret)
	}

	return geodist.Coord{Lat: ret.Lat, Lon: ret.Lon}, nil
}

// todo: temp
func PublicAddr() (netip.Addr, error) {
	resp, err := http.Get("http://ifconfig.cc")
	if err != nil {
		return netip.Addr{}, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return netip.Addr{}, err
	}
	return netip.ParseAddr(string(data))
}
