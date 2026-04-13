package app

import "testing"

func TestBuildUIReadyMessageForWildcardBind(t *testing.T) {
	message := buildUIReadyMessage("0.0.0.0:8765", "0.0.0.0:8765")
	expected := "netcheck UI 已启动，监听地址: 0.0.0.0:8765，本机访问: http://127.0.0.1:8765"
	if message != expected {
		t.Fatalf("期望提示为 %q，实际为 %q", expected, message)
	}
}

func TestBuildUIReadyMessageForFallbackPort(t *testing.T) {
	message := buildUIReadyMessage("0.0.0.0:8765", "0.0.0.0:8766")
	expected := "netcheck UI 已启动，默认监听地址 0.0.0.0:8765 已被占用，已切换到: 0.0.0.0:8766，本机访问: http://127.0.0.1:8766"
	if message != expected {
		t.Fatalf("期望提示为 %q，实际为 %q", expected, message)
	}
}
