package event

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestXxx(t *testing.T) {

	var b udpTable

	b, err := getExtendedUdpTableWithPid(b)
	require.NoError(t, err)

	n := b.Entries()

	for i := 0; i < int(n); i++ {
		ip := b.Addr(i).String()
		pid := b.Pid(i)
		port := b.Port(i)

		fmt.Println(ip, pid, port)
	}
}
