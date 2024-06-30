package client

import (
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func Test_NetworkStates(t *testing.T) {
	var stats = NetworkStates{}
	str := stats.String()
	require.Equal(t, 6, strings.Count(str, "--.-"))
}

func Test_TrunkRoute(t *testing.T) {
	var (
		gaddr = netip.MustParseAddrPort("1.2.3.4:567")
		faddr = netip.MustParseAddrPort("8.8.8.8:123")
	)

	t.Run("not update", func(t *testing.T) {
		var tr = newTrunkRouteRecorder(time.Second, gaddr, faddr)

		p, f, update := tr.Trunk()
		require.Equal(t, gaddr, p)
		require.Equal(t, faddr, f)
		require.True(t, update)
	})

	t.Run("not update2", func(t *testing.T) {
		var tr = newTrunkRouteRecorder(time.Second, gaddr, faddr)

		{
			p, f, update := tr.Trunk()
			require.Equal(t, gaddr, p)
			require.Equal(t, faddr, f)
			require.True(t, update)
		}
		{
			p, f, update := tr.Trunk()
			require.Equal(t, gaddr, p)
			require.Equal(t, faddr, f)
			require.False(t, update)
		}
	})

	t.Run("update", func(t *testing.T) {
		var tr = newTrunkRouteRecorder(time.Second, gaddr, faddr)

		tr.Update(
			netip.MustParseAddrPort("0.0.0.0:111"),
			netip.MustParseAddrPort("1.1.1.1:111"),
		)

		p, f, update := tr.Trunk()
		require.Equal(t, netip.MustParseAddrPort("0.0.0.0:111"), p)
		require.Equal(t, netip.MustParseAddrPort("1.1.1.1:111"), f)
		require.True(t, update)
	})

	t.Run("update2", func(t *testing.T) {
		var tr = newTrunkRouteRecorder(time.Second, gaddr, faddr)

		tr.Update(
			netip.MustParseAddrPort("0.0.0.0:111"),
			netip.MustParseAddrPort("1.1.1.1:111"),
		)

		{
			p, f, update := tr.Trunk()
			require.Equal(t, netip.MustParseAddrPort("0.0.0.0:111"), p)
			require.Equal(t, netip.MustParseAddrPort("1.1.1.1:111"), f)
			require.True(t, update)
		}
		{
			p, f, update := tr.Trunk()
			require.Equal(t, netip.MustParseAddrPort("0.0.0.0:111"), p)
			require.Equal(t, netip.MustParseAddrPort("1.1.1.1:111"), f)
			require.False(t, update)
		}
	})

	t.Run("update timeout scale", func(t *testing.T) {
		var tr = newTrunkRouteRecorder(time.Second, gaddr, faddr)

		tr.Update(
			netip.MustParseAddrPort("0.0.0.0:111"),
			netip.MustParseAddrPort("1.1.1.1:111"),
		)

		{
			p, f, update := tr.Trunk()
			require.Equal(t, netip.MustParseAddrPort("0.0.0.0:111"), p)
			require.Equal(t, netip.MustParseAddrPort("1.1.1.1:111"), f)
			require.True(t, update)
		}

		time.Sleep(time.Second * 2)

		{
			p, f, update := tr.Trunk()
			require.Equal(t, gaddr, p)
			require.Equal(t, faddr, f)
			require.True(t, update)
		}
	})

	t.Run("update timeout scale2", func(t *testing.T) {
		var tr = newTrunkRouteRecorder(time.Second, gaddr, faddr)

		tr.Update(
			netip.MustParseAddrPort("0.0.0.0:111"),
			netip.MustParseAddrPort("1.1.1.1:111"),
		)

		{
			p, f, update := tr.Trunk()
			require.Equal(t, netip.MustParseAddrPort("0.0.0.0:111"), p)
			require.Equal(t, netip.MustParseAddrPort("1.1.1.1:111"), f)
			require.True(t, update)
		}
		time.Sleep(time.Second * 2)
		{
			p, f, update := tr.Trunk()
			require.Equal(t, gaddr, p)
			require.Equal(t, faddr, f)
			require.True(t, update)
		}

		tr.Update(
			netip.MustParseAddrPort("0.0.0.1:111"),
			netip.MustParseAddrPort("2.2.2.2:111"),
		)
		{
			p, f, update := tr.Trunk()
			require.Equal(t, netip.MustParseAddrPort("0.0.0.1:111"), p)
			require.Equal(t, netip.MustParseAddrPort("2.2.2.2:111"), f)
			require.True(t, update)
		}
	})
}
