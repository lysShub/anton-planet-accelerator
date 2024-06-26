//go:build !windows
// +build !windows

package inject

import "github.com/pkg/errors"

func NewInject() (Inject, error) {
	return nil, errors.New("not implement")
}
