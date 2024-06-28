package client

import (
	"errors"
	"fmt"
	"testing"

	"github.com/lysShub/netkit/errorx"
)

func TestXxxxx(t *testing.T) {

	err := errorx.WrapTemp(ErrRouteProbe)

	fmt.Println(errors.Is(err, ErrRouteProbe))
}
