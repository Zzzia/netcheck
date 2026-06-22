//go:build !linux && !darwin

package probe

import "errors"

func DefaultGateway() (string, error) {
	return "", errors.New("default gateway detection is not implemented on this platform")
}
