package report

import "testing"

func TestNormalizeListenerAddrKeepsWildcardHost(t *testing.T) {
	if got := normalizeListenerAddr("0.0.0.0:8765"); got != "0.0.0.0:8765" {
		t.Fatalf("期望保留全网卡监听地址，实际为 %s", got)
	}
}

func TestNormalizeListenerAddrFillsEmptyHost(t *testing.T) {
	if got := normalizeListenerAddr(":8765"); got != "0.0.0.0:8765" {
		t.Fatalf("期望空 host 归一化为 0.0.0.0，实际为 %s", got)
	}
}
