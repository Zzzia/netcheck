//go:build darwin

package probe

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

var darwinGatewayRE = regexp.MustCompile(`gateway:\s+([^\s]+)`)

func DefaultGateway() (string, error) {
	output, err := exec.Command("route", "-n", "get", "default").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("读取默认路由失败: %w, 输出: %s", err, strings.TrimSpace(string(output)))
	}
	matches := darwinGatewayRE.FindStringSubmatch(string(output))
	if len(matches) != 2 {
		return "", fmt.Errorf("无法从默认路由中解析网关: %s", strings.TrimSpace(string(output)))
	}
	return matches[1], nil
}
