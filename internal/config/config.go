package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	DBPath     string      `json:"db_path"`
	Sampling   Sampling    `json:"sampling"`
	Thresholds Thresholds  `json:"thresholds"`
	Targets    Targets     `json:"targets"`
	Alerts     AlertConfig `json:"alerts"`
}

type Sampling struct {
	GatewayIntervalSec          int   `json:"gateway_interval_sec"`
	DomesticLatencyIntervalSec  int   `json:"domestic_latency_interval_sec"`
	InternationalLatencySeconds int   `json:"international_latency_interval_sec"`
	DomesticDownloadIntervalSec int   `json:"domestic_download_interval_sec"`
	InternationalDownloadSec    int   `json:"international_download_interval_sec"`
	DomesticDownloadBytes       int64 `json:"domestic_download_bytes"`
	InternationalDownloadBytes  int64 `json:"international_download_bytes"`
	PingTimeoutSec              int   `json:"ping_timeout_sec"`
	ConnectTimeoutMs            int   `json:"connect_timeout_ms"`
	HTTPTimeoutMs               int   `json:"http_timeout_ms"`
}

type Thresholds struct {
	GatewayRTTWarnMs              float64 `json:"gateway_rtt_warn_ms"`
	GatewayJitterWarnMs           float64 `json:"gateway_jitter_warn_ms"`
	GatewayLossWarnRatio          float64 `json:"gateway_loss_warn_ratio"`
	DomesticLatencyWarnMs         float64 `json:"domestic_latency_warn_ms"`
	InternationalLatencyWarnMs    float64 `json:"international_latency_warn_ms"`
	DomesticDownloadWarnMbps      float64 `json:"domestic_download_warn_mbps"`
	InternationalDownloadWarnMbps float64 `json:"international_download_warn_mbps"`
	RemoteFailureWarnRatio        float64 `json:"remote_failure_warn_ratio"`
	EnterConsecutive              int     `json:"enter_consecutive"`
	RecoverConsecutive            int     `json:"recover_consecutive"`
}

type Targets struct {
	DomesticLatency        []Target       `json:"domestic_latency"`
	InternationalLatency   []Target       `json:"international_latency"`
	DomesticDownloads      []DownloadItem `json:"domestic_downloads"`
	InternationalDownloads []DownloadItem `json:"international_downloads"`
}

type Target struct {
	Name    string `json:"name"`
	Address string `json:"address,omitempty"`
	URL     string `json:"url,omitempty"`
}

type DownloadItem struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type AlertConfig struct {
	PrintStateChanges bool `json:"print_state_changes"`
}

func DefaultPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("get user config directory failed: %w", err)
	}
	return filepath.Join(base, "netcheck", "config.json"), nil
}

func DefaultDBPath() (string, error) {
	cfg, err := DefaultPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(cfg), "netcheck.sqlite"), nil
}

func Default() (Config, error) {
	dbPath, err := DefaultDBPath()
	if err != nil {
		return Config{}, err
	}
	cfg := Config{
		DBPath: dbPath,
		Sampling: Sampling{
			GatewayIntervalSec:          1,
			DomesticLatencyIntervalSec:  15,
			InternationalLatencySeconds: 15,
			DomesticDownloadIntervalSec: 30,
			InternationalDownloadSec:    180,
			DomesticDownloadBytes:       1024 * 1024,
			InternationalDownloadBytes:  1024 * 1024,
			PingTimeoutSec:              2,
			ConnectTimeoutMs:            2500,
			HTTPTimeoutMs:               15000,
		},
		Thresholds: Thresholds{
			GatewayRTTWarnMs:              180,
			GatewayJitterWarnMs:           60,
			GatewayLossWarnRatio:          0.2,
			DomesticLatencyWarnMs:         180,
			InternationalLatencyWarnMs:    450,
			DomesticDownloadWarnMbps:      10,
			InternationalDownloadWarnMbps: 10,
			RemoteFailureWarnRatio:        0.34,
			EnterConsecutive:              3,
			RecoverConsecutive:            3,
		},
		Targets: Targets{
			DomesticLatency: []Target{
				{Name: "tuna-mirror", URL: "https://mirrors.tuna.tsinghua.edu.cn/ubuntu-releases/24.04/ubuntu-24.04.4-live-server-amd64.iso"},
				{Name: "qq-home", URL: "https://www.qq.com/"},
			},
			InternationalLatency: []Target{
				{Name: "openai-status", URL: "https://status.openai.com/api/v2/status.json"},
				{Name: "claude-status", URL: "https://status.claude.com/api/v2/status.json"},
			},
			DomesticDownloads: []DownloadItem{
				{
					Name: "tuna-ubuntu-iso",
					URL:  "https://mirrors.tuna.tsinghua.edu.cn/ubuntu-releases/24.04/ubuntu-24.04.4-live-server-amd64.iso",
				},
			},
			InternationalDownloads: []DownloadItem{
				{
					Name: "cloudflare-speed",
					URL:  "https://speed.cloudflare.com/__down?bytes=1048576",
				},
			},
		},
		Alerts: AlertConfig{
			PrintStateChanges: true,
		},
	}
	return cfg, nil
}

func Load(path string) (Config, error) {
	defaults, err := Default()
	if err != nil {
		return Config{}, err
	}
	if path == "" {
		path, err = DefaultPath()
		if err != nil {
			return Config{}, err
		}
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		if errors.Is(readErr, os.ErrNotExist) {
			if err := defaults.Validate(); err != nil {
				return Config{}, err
			}
			return defaults, nil
		}
		return Config{}, fmt.Errorf("read config failed: %w", readErr)
	}
	cfg := cloneConfig(defaults)
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config failed: %w", err)
	}
	applyLegacyDefaults(&cfg, defaults)
	if strings.TrimSpace(cfg.DBPath) == "" {
		cfg.DBPath = defaults.DBPath
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func WriteDefault(path string, force bool) error {
	cfg, err := Default()
	if err != nil {
		return err
	}
	if path == "" {
		path, err = DefaultPath()
		if err != nil {
			return err
		}
	}
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("config file already exists: %s", path)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config directory failed: %w", err)
	}
	encoded, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("serialize default config failed: %w", err)
	}
	if err := os.WriteFile(path, append(encoded, '\n'), 0o644); err != nil {
		return fmt.Errorf("write default config failed: %w", err)
	}
	return nil
}

func applyLegacyDefaults(cfg *Config, defaults Config) {
	if usesLegacyInternationalLatencyDefaults(cfg.Targets.InternationalLatency) {
		cfg.Targets.InternationalLatency = append([]Target(nil), defaults.Targets.InternationalLatency...)
	}
}

func cloneConfig(source Config) Config {
	cloned := source
	cloned.Targets.DomesticLatency = append([]Target(nil), source.Targets.DomesticLatency...)
	cloned.Targets.InternationalLatency = append([]Target(nil), source.Targets.InternationalLatency...)
	cloned.Targets.DomesticDownloads = append([]DownloadItem(nil), source.Targets.DomesticDownloads...)
	cloned.Targets.InternationalDownloads = append([]DownloadItem(nil), source.Targets.InternationalDownloads...)
	return cloned
}

func usesLegacyInternationalLatencyDefaults(targets []Target) bool {
	if len(targets) != 2 {
		return false
	}
	legacyTargets := map[string]string{
		"cloudflare-speed": "https://speed.cloudflare.com/__down?bytes=1",
		"github-home":      "https://github.com/",
	}
	for _, target := range targets {
		expectedURL, ok := legacyTargets[target.Name]
		if !ok || target.Address != "" || target.URL != expectedURL {
			return false
		}
		delete(legacyTargets, target.Name)
	}
	return len(legacyTargets) == 0
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.DBPath) == "" {
		return errors.New("db_path cannot be empty")
	}
	if c.Sampling.GatewayIntervalSec <= 0 || c.Sampling.DomesticLatencyIntervalSec <= 0 || c.Sampling.InternationalLatencySeconds <= 0 {
		return errors.New("latency sampling intervals must be greater than 0")
	}
	if c.Sampling.DomesticDownloadIntervalSec <= 0 || c.Sampling.InternationalDownloadSec <= 0 {
		return errors.New("download sampling intervals must be greater than 0")
	}
	if c.Sampling.DomesticDownloadBytes <= 0 || c.Sampling.InternationalDownloadBytes <= 0 {
		return errors.New("download byte limits must be greater than 0")
	}
	if c.Sampling.PingTimeoutSec <= 0 || c.Sampling.ConnectTimeoutMs <= 0 || c.Sampling.HTTPTimeoutMs <= 0 {
		return errors.New("timeout settings must be greater than 0")
	}
	if len(c.Targets.DomesticLatency) == 0 || len(c.Targets.InternationalLatency) == 0 {
		return errors.New("latency probe targets cannot be empty")
	}
	if len(c.Targets.DomesticDownloads) == 0 || len(c.Targets.InternationalDownloads) == 0 {
		return errors.New("download probe targets cannot be empty")
	}
	for _, target := range append(append([]Target{}, c.Targets.DomesticLatency...), c.Targets.InternationalLatency...) {
		if strings.TrimSpace(target.Address) == "" && strings.TrimSpace(target.URL) == "" {
			return fmt.Errorf("latency probe target %s must configure at least address or url", target.Name)
		}
	}
	if c.Thresholds.EnterConsecutive <= 0 || c.Thresholds.RecoverConsecutive <= 0 {
		return errors.New("state debounce settings must be greater than 0")
	}
	return nil
}
