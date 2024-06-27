package bvvd_test

import (
	"testing"

	"github.com/lysShub/anton-planet-accelerator/bvvd"
	"github.com/stretchr/testify/require"
)

func Test_LocID(t *testing.T) {

}

func Test_Forwards(t *testing.T) {
	t.Run("SortBy", func(t *testing.T) {
		fs := bvvd.Regions.SortByLoction(bvvd.Tokyo)
		require.Equal(t, fs[0].Location, bvvd.Tokyo)
		require.Equal(t, fs[len(fs)-1].Location, bvvd.NewYork)
	})
}
