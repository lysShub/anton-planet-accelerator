package client

import (
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/lysShub/anton-planet-accelerator/bvvd"
	"github.com/stretchr/testify/require"
)

func Test_NetworkStates(t *testing.T) {
	var stats = NetworkStates{}
	str := stats.String()
	require.Equal(t, 6, strings.Count(str, "--.-"))
}

func Test_TrunkRoute(t *testing.T) {
	var (
		paddr = netip.MustParseAddrPort("1.2.3.4:567")
		fid   = bvvd.ForwardID(1)
	)

	t.Run("not update", func(t *testing.T) {
		var tr = newTrunkRouteRecorder(time.Second, paddr, fid)

		p, f, update := tr.Trunk()
		require.Equal(t, paddr, p)
		require.Equal(t, fid, f)
		require.True(t, update)
	})

	t.Run("not update2", func(t *testing.T) {
		var tr = newTrunkRouteRecorder(time.Second, paddr, fid)

		{
			p, f, update := tr.Trunk()
			require.Equal(t, paddr, p)
			require.Equal(t, fid, f)
			require.True(t, update)
		}
		{
			p, f, update := tr.Trunk()
			require.Equal(t, paddr, p)
			require.Equal(t, fid, f)
			require.False(t, update)
		}
	})

	t.Run("update", func(t *testing.T) {
		var tr = newTrunkRouteRecorder(time.Second, paddr, fid)

		tr.Update(
			netip.MustParseAddrPort("0.0.0.0:111"),
			bvvd.ForwardID(2),
		)

		p, f, update := tr.Trunk()
		require.Equal(t, netip.MustParseAddrPort("0.0.0.0:111"), p)
		require.Equal(t, bvvd.ForwardID(2), f)
		require.True(t, update)
	})

	t.Run("update2", func(t *testing.T) {
		var tr = newTrunkRouteRecorder(time.Second, paddr, fid)

		tr.Update(
			netip.MustParseAddrPort("0.0.0.0:111"),
			bvvd.ForwardID(2),
		)

		{
			p, f, update := tr.Trunk()
			require.Equal(t, netip.MustParseAddrPort("0.0.0.0:111"), p)
			require.Equal(t, bvvd.ForwardID(2), f)
			require.True(t, update)
		}
		{
			p, f, update := tr.Trunk()
			require.Equal(t, netip.MustParseAddrPort("0.0.0.0:111"), p)
			require.Equal(t, bvvd.ForwardID(2), f)
			require.False(t, update)
		}
	})

	t.Run("update timeout scale", func(t *testing.T) {
		var tr = newTrunkRouteRecorder(time.Second, paddr, fid)

		tr.Update(
			netip.MustParseAddrPort("0.0.0.0:111"),
			bvvd.ForwardID(2),
		)

		{
			p, f, update := tr.Trunk()
			require.Equal(t, netip.MustParseAddrPort("0.0.0.0:111"), p)
			require.Equal(t, bvvd.ForwardID(2), f)
			require.True(t, update)
		}

		time.Sleep(time.Second * 2)

		{
			p, f, update := tr.Trunk()
			require.Equal(t, paddr, p)
			require.Equal(t, fid, f)
			require.True(t, update)
		}
	})

	t.Run("update timeout scale2", func(t *testing.T) {
		var tr = newTrunkRouteRecorder(time.Second, paddr, fid)

		tr.Update(
			netip.MustParseAddrPort("0.0.0.0:111"),
			bvvd.ForwardID(2),
		)

		{
			p, f, update := tr.Trunk()
			require.Equal(t, netip.MustParseAddrPort("0.0.0.0:111"), p)
			require.Equal(t, bvvd.ForwardID(2), f)
			require.True(t, update)
		}
		time.Sleep(time.Second * 2)
		{
			p, f, update := tr.Trunk()
			require.Equal(t, paddr, p)
			require.Equal(t, fid, f)
			require.True(t, update)
		}

		tr.Update(
			netip.MustParseAddrPort("0.0.0.1:111"),
			bvvd.ForwardID(3),
		)
		{
			p, f, update := tr.Trunk()
			require.Equal(t, netip.MustParseAddrPort("0.0.0.1:111"), p)
			require.Equal(t, bvvd.ForwardID(3), f)
			require.True(t, update)
		}
	})
}
