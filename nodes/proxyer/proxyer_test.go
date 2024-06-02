package proxyer

import (
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestXxxx(t *testing.T) {

	conn, err := net.ListenUDP("udp4", nil)
	require.NoError(t, err)
	defer conn.Close()

	fmt.Println(conn.LocalAddr().String())

}
