//go:build windows
// +build windows

package game

import "github.com/pkg/errors"

func New(name string) (game Game, err error) {
	switch name {
	case "warthunder":
		return newWarthundr()
	default:
		return nil, errors.Errorf("not support game %s", name)
	}
}
