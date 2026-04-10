//go:build !linux && !darwin

package probe

import "errors"

func DefaultGateway() (string, error) {
	return "", errors.New("当前平台暂未实现默认网关探测")
}
