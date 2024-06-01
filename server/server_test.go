package server_test

import (
	"testing"

	"github.com/lysShub/anton-planet-accelerator/server"
	"github.com/stretchr/testify/require"
)

func TestXxxx(t *testing.T) {

	s, err := server.New(":443", &server.Config{})
	require.NoError(t, err)
	defer s.Close()

	require.NoError(t, s.Serve())

}
