package bvvd_test

import (
	"fmt"
	"testing"

	"github.com/lysShub/anton-planet-accelerator/bvvd"
	"github.com/stretchr/testify/require"
)

func Test_LocID(t *testing.T) {

	t.Run("0", func(t *testing.T) {
		var a bvvd.LocID

		require.NoError(t, a.SetID(11))
		a.SetLoc(bvvd.Frankfurt)

		require.Equal(t, uint8(11), a.ID())
		require.Equal(t, bvvd.Frankfurt, a.Loc())
	})

	t.Run("1", func(t *testing.T) {
		var a bvvd.LocID = 1
		fmt.Println(a.String())

		require.NoError(t, a.SetID(1))

		require.Equal(t, "Moscow:1", a.String())
	})

}
