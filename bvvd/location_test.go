package bvvd

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Valid(t *testing.T) {
	require.Equal(t, int(_end)+1, len(infos))
	require.Equal(t, int(_end)-1, len(Locations))
}

func Test_Location(t *testing.T) {

	for _, loc := range Locations {
		fmt.Println(loc.String())
	}

}
