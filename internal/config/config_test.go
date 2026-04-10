package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigIsValid(t *testing.T) {
	cfg, err := Default()
	if err != nil {
		t.Fatalf("Default 返回错误: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("默认配置校验失败: %v", err)
	}
	if cfg.Sampling.InternationalDownloadSec != 180 {
		t.Fatalf("期望国外下载默认周期为 180 秒，实际为 %d", cfg.Sampling.InternationalDownloadSec)
	}
	if cfg.Thresholds.DomesticDownloadWarnMbps != 10 || cfg.Thresholds.InternationalDownloadWarnMbps != 10 {
		t.Fatalf("期望国内/国外下载异常阈值均为 10Mbps，实际为 %.1f/%.1f", cfg.Thresholds.DomesticDownloadWarnMbps, cfg.Thresholds.InternationalDownloadWarnMbps)
	}
	if len(cfg.Targets.InternationalLatency) != 2 {
		t.Fatalf("期望国外默认延迟目标为 2 个，实际为 %d", len(cfg.Targets.InternationalLatency))
	}
	if cfg.Targets.InternationalLatency[0].Name != "openai-status" || cfg.Targets.InternationalLatency[1].Name != "claude-status" {
		t.Fatalf("期望国外默认延迟目标为 openai-status/claude-status，实际为 %s/%s", cfg.Targets.InternationalLatency[0].Name, cfg.Targets.InternationalLatency[1].Name)
	}
}

func TestValidateRejectsEmptyTargets(t *testing.T) {
	cfg, err := Default()
	if err != nil {
		t.Fatalf("Default 返回错误: %v", err)
	}
	cfg.Targets.DomesticDownloads = nil
	if err := cfg.Validate(); err == nil {
		t.Fatal("期望配置校验失败，但实际成功")
	}
}

func TestLoadMigratesLegacyInternationalLatencyTargets(t *testing.T) {
	cfg, err := Default()
	if err != nil {
		t.Fatalf("Default 返回错误: %v", err)
	}
	cfg.Targets.InternationalLatency = []Target{
		{Name: "cloudflare-speed", URL: "https://speed.cloudflare.com/__down?bytes=1"},
		{Name: "github-home", URL: "https://github.com/"},
	}
	configPath := filepath.Join(t.TempDir(), "config.json")
	payload, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("序列化配置失败: %v", err)
	}
	if err := os.WriteFile(configPath, payload, 0o644); err != nil {
		t.Fatalf("写入配置失败: %v", err)
	}

	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load 返回错误: %v", err)
	}
	if loaded.Targets.InternationalLatency[0].Name != "openai-status" || loaded.Targets.InternationalLatency[1].Name != "claude-status" {
		t.Fatalf("期望自动迁移为 openai-status/claude-status，实际为 %s/%s", loaded.Targets.InternationalLatency[0].Name, loaded.Targets.InternationalLatency[1].Name)
	}
}

func TestLoadKeepsCustomInternationalLatencyTargets(t *testing.T) {
	cfg, err := Default()
	if err != nil {
		t.Fatalf("Default 返回错误: %v", err)
	}
	cfg.Targets.InternationalLatency = []Target{
		{Name: "custom-openai", URL: "https://openai.com/"},
		{Name: "custom-claude", URL: "https://claude.ai/"},
	}
	configPath := filepath.Join(t.TempDir(), "config.json")
	payload, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("序列化配置失败: %v", err)
	}
	if err := os.WriteFile(configPath, payload, 0o644); err != nil {
		t.Fatalf("写入配置失败: %v", err)
	}

	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load 返回错误: %v", err)
	}
	if loaded.Targets.InternationalLatency[0].Name != "custom-openai" || loaded.Targets.InternationalLatency[1].Name != "custom-claude" {
		t.Fatalf("期望保留自定义目标，实际为 %s/%s", loaded.Targets.InternationalLatency[0].Name, loaded.Targets.InternationalLatency[1].Name)
	}
}
