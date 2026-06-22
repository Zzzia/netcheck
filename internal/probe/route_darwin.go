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
		return "", fmt.Errorf("read default route failed: %w, output: %s", err, strings.TrimSpace(string(output)))
	}
	matches := darwinGatewayRE.FindStringSubmatch(string(output))
	if len(matches) != 2 {
		return "", fmt.Errorf("cannot parse gateway from default route: %s", strings.TrimSpace(string(output)))
	}
	return matches[1], nil
}
