//go:build linux

package probe

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

var linuxDefaultRouteRE = regexp.MustCompile(`default via ([^\s]+)`)

func DefaultGateway() (string, error) {
	output, err := exec.Command("ip", "route", "show", "default").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("read default route failed: %w, output: %s", err, strings.TrimSpace(string(output)))
	}
	matches := linuxDefaultRouteRE.FindStringSubmatch(string(output))
	if len(matches) != 2 {
		return "", fmt.Errorf("cannot parse gateway from default route: %s", strings.TrimSpace(string(output)))
	}
	return matches[1], nil
}
