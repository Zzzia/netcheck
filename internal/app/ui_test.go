package app

import (
	"testing"

	"netcheck/internal/i18n"
)

func TestBuildUIReadyMessageForWildcardBind(t *testing.T) {
	message := buildUIReadyMessage("0.0.0.0:8765", "0.0.0.0:8765")
	expected := "netcheck UI started, listening on: 0.0.0.0:8765, local access: http://127.0.0.1:8765"
	if message != expected {
		t.Fatalf("期望提示为 %q，实际为 %q", expected, message)
	}
}

func TestBuildUIReadyMessageForFallbackPort(t *testing.T) {
	message := buildUIReadyMessage("0.0.0.0:8765", "0.0.0.0:8766")
	expected := "netcheck UI started, default listen address 0.0.0.0:8765 is in use, switched to: 0.0.0.0:8766, local access: http://127.0.0.1:8766"
	if message != expected {
		t.Fatalf("期望提示为 %q，实际为 %q", expected, message)
	}
}

func TestBuildUIReadyMessageSupportsChinese(t *testing.T) {
	message := buildUIReadyMessageForLang("0.0.0.0:8765", "0.0.0.0:8765", i18n.Chinese)
	expected := "netcheck UI 已启动，监听地址: 0.0.0.0:8765，本机访问: http://127.0.0.1:8765"
	if message != expected {
		t.Fatalf("期望中文提示为 %q，实际为 %q", expected, message)
	}
}
