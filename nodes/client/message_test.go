package client_test

import (
	"strings"
	"testing"

	"github.com/lysShub/anton-planet-accelerator/nodes/client"
	"github.com/stretchr/testify/require"
)

func Test_NetworkStates(t *testing.T) {
	var stats = client.NetworkStates{}
	str := stats.String()
	require.Equal(t, 6, strings.Count(str, "--.-"))
}
