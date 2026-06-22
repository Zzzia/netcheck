package i18n

import (
	"encoding/json"
	"os"
	"strings"
)

type Lang string

const (
	English Lang = "en"
	Chinese Lang = "zh-CN"
)

func Parse(raw string) Lang {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "zh", "zh-cn", "zh_cn", "cn", "chinese":
		return Chinese
	default:
		return English
	}
}

func FromEnv() Lang {
	return Parse(os.Getenv("NETCHECK_LANG"))
}

func (lang Lang) Code() string {
	if lang == Chinese {
		return string(Chinese)
	}
	return string(English)
}

func (lang Lang) HTMLLang() string {
	return lang.Code()
}

type Localizer struct {
	lang Lang
}

func New(lang Lang) Localizer {
	return Localizer{lang: Parse(lang.Code())}
}

func (l Localizer) Lang() Lang {
	return l.lang
}

func (l Localizer) Code() string {
	return l.lang.Code()
}

func (l Localizer) T(key string) string {
	if values, ok := messages[l.lang]; ok {
		if value, ok := values[key]; ok {
			return value
		}
	}
	if value, ok := messages[English][key]; ok {
		return value
	}
	return key
}

func MessagesJSON() string {
	encoded, err := json.Marshal(messages)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

var messages = map[Lang]map[string]string{
	English: {
		"app.usage": `netcheck usage:

  netcheck [--lang en|zh-CN]
  netcheck monitor [--config path] [--duration 5m] [--lang en|zh-CN]
  netcheck report [--config path] [--since 24h] [--start 2026-04-10T09:00 --end 2026-04-10T18:00] [--output report.html] [--lang en|zh-CN]
  netcheck ui [--config path] [--addr 0.0.0.0:8765] [--lang en|zh-CN]
  netcheck clear [--config path] [--lang en|zh-CN]
  netcheck init-config [--output path] [--force] [--lang en|zh-CN]`,
		"cli.unknown_command":                  "unknown command",
		"cli.flag.config":                      "configuration file path",
		"cli.flag.duration":                    "run duration; defaults to running until an exit signal is received",
		"cli.flag.since":                       "time range, for example 24h or 7d",
		"cli.flag.start":                       "start time; supports RFC3339 or 2006-01-02T15:04",
		"cli.flag.end":                         "end time; supports RFC3339 or 2006-01-02T15:04",
		"cli.flag.output":                      "report output path",
		"cli.flag.addr":                        "listen address",
		"cli.flag.force":                       "overwrite existing configuration",
		"cli.flag.lang":                        "language: en or zh-CN",
		"cli.error.missing_lang_value":         "missing value for --lang",
		"cli.error.invalid_since":              "cannot parse since value: %s",
		"cli.error.invalid_since_duration":     "cannot parse since value: %v",
		"cli.error.positive_since":             "since must be greater than 0",
		"cli.error.custom_range_pair":          "custom ranges require both --start and --end",
		"cli.error.parse_start":                "parse start time failed: %v",
		"cli.error.parse_end":                  "parse end time failed: %v",
		"cli.error.end_before_start":           "end time cannot be before start time",
		"cli.error.unsupported_time_format":    "unsupported time format: %s",
		"cli.error.create_report_dir":          "create report directory failed: %v",
		"cli.config_written":                   "default configuration written: %s\n",
		"cli.ui_ready":                         "netcheck UI started, listening on: %s",
		"cli.ui_ready_local":                   "netcheck UI started, listening on: %s, local access: %s",
		"cli.ui_port_changed":                  "netcheck UI started, default listen address %s is in use, switched to: %s",
		"cli.ui_port_changed_local":            "netcheck UI started, default listen address %s is in use, switched to: %s, local access: %s",
		"clear.no_database":                    "database file does not exist, nothing to clear: %s\n",
		"clear.removed":                        "removed %d database files: %s\n",
		"monitor.started":                      "netcheck started, database: %s\n",
		"monitor.sampling":                     "gateway sample: every %ds, domestic download: every %ds, international download: every %ds\n",
		"monitor.download_budget":              "estimated daily download traffic: domestic %.1fMB, international %.1fMB\n",
		"monitor.background_error":             "[%s] background error: %v\n",
		"monitor.degraded_start":               "[%s] degradation started [%s] %s (%s)\n",
		"monitor.degraded_resolved":            "[%s] degradation resolved [%s] %s\n",
		"monitor.no_evidence":                  "no extra evidence",
		"monitor.speedtest_failed":             "[%s] speed test [%s] failed: %s\n",
		"monitor.speedtest":                    "[%s] speed test [%s] %.2f Mbps\n",
		"state.local_insufficient":             "not enough gateway samples",
		"state.local_summary":                  "gateway avg=%.1fms p95=%.1fms jitter=%.1fms loss=%.0f%%",
		"state.remote_summary":                 "%s latency avg=%.1fms failure=%.0f%% dl=%.2fMbps",
		"evidence.jitter":                      "jitter %.1fms > %.1fms",
		"evidence.loss":                        "packet loss %.0f%% > %.0f%%",
		"evidence.avg_latency":                 "average latency %.1fms > %.1fms",
		"evidence.failure_rate":                "failure rate %.0f%% > %.0f%%",
		"evidence.avg_download":                "average download %.2fMbps < %.2fMbps",
		"common.no_data":                       "No data",
		"ui.title":                             "netcheck network dashboard",
		"ui.range":                             "Time range",
		"ui.range.30m":                         "Last 30 minutes",
		"ui.range.1h":                          "Last 1 hour",
		"ui.range.6h":                          "Last 6 hours",
		"ui.range.24h":                         "Last 24 hours",
		"ui.range.7d":                          "Last 7 days",
		"ui.apply_custom":                      "Apply custom range",
		"ui.incident_overview":                 "Incident overview",
		"ui.link_incidents":                    "Link incident totals",
		"ui.event_timeline":                    "Event timeline (latest 10)",
		"ui.updated_at":                        "Updated at: %s",
		"ui.alert.custom_range":                "Please fill both start and end time.",
		"ui.incident_count":                    "Incident count",
		"ui.longest_duration":                  "Longest duration",
		"ui.empty_causes":                      "No incident cause data in the selected range.",
		"ui.empty_events":                      "No events in the selected range.",
		"ui.table.category":                    "Category",
		"ui.table.total_duration":              "Total duration",
		"ui.table.status":                      "Status",
		"ui.table.summary":                     "Summary",
		"ui.table.evidence":                    "Evidence",
		"ui.table.window_start":                "Window start",
		"ui.table.window_end":                  "Window end",
		"ui.table.window_duration":             "Window duration",
		"ui.tooltip.time":                      "Time",
		"ui.tooltip.value":                     "Value",
		"layer.gateway":                        "Gateway",
		"layer.domestic":                       "Domestic",
		"layer.international":                  "International",
		"cause.root":                           "Cause",
		"cause.local":                          "Local link",
		"cause.domestic":                       "Domestic link",
		"cause.international":                  "International link",
		"cause.global":                         "Global issue",
		"cause.degraded":                       "Degraded",
		"summary.gateway":                      "Gateway",
		"summary.domestic":                     "Domestic",
		"summary.international":                "International",
		"metric.rtt":                           "RTT",
		"metric.average":                       "Average",
		"metric.average_jitter":                "Average jitter",
		"metric.loss_rate":                     "Packet loss",
		"metric.latency":                       "Latency",
		"metric.download":                      "Download",
		"metric.failure_rate":                  "Failure rate",
		"group.gateway_quality":                "Gateway quality",
		"group.domestic_quality":               "Domestic quality",
		"group.international_quality":          "International quality",
		"chart.gateway_rtt":                    "Gateway RTT",
		"chart.gateway_loss":                   "Gateway packet loss",
		"chart.domestic_rtt":                   "Domestic latency",
		"chart.domestic_failure":               "Domestic failure rate",
		"chart.domestic_download":              "Domestic download speed",
		"chart.international_rtt":              "International latency",
		"chart.international_failure":          "International failure rate",
		"chart.international_download":         "International download speed",
		"unit.ratio":                           "ratio",
		"event.before_window":                  " (started before window)",
		"event.ongoing":                        " (ongoing)",
		"event.after_window":                   " (ends after window)",
		"codex.error.empty_path":               "Codex log path is empty",
		"codex.error.missing_log":              "Codex log was not found",
		"codex.error.read_log":                 "read Codex log failed: %v",
		"codex.error.scan_log":                 "scan Codex log failed: %v",
		"codex.kind.stream_retry":              "Stream retry",
		"codex.kind.tool_error":                "Local tool error",
		"codex.kind.rollout_record_error":      "Session record error",
		"codex.kind.apps_or_tool_403":          "Apps/tool suggestion 403",
		"codex.kind.network_candidate":         "Network error",
		"codex.kind.unknown_error":             "Unknown error",
		"codex.kind.unknown_warning":           "Unknown warning",
		"codex.noise.plugin_icon":              "Plugin/skill icon warning",
		"codex.noise.default_prompt":           "Plugin default prompt warning",
		"codex.noise.file_watcher":             "File watcher release warning",
		"codex.noise.clipboard":                "Clipboard image warning",
		"codex.noise.other":                    "Other suppressed warning",
		"codex.title":                          "Codex network stability",
		"codex.empty":                          "Codex log was not found.",
		"codex.loading":                        "Analyzing local Codex logs...",
		"codex.clamped":                        "clamped to the latest 24 hours",
		"codex.metric.retry":                   "Stream retries",
		"codex.metric.affected_turns":          "Affected turns",
		"codex.metric.network_errors":          "Network errors",
		"codex.metric.max_retry":               "Max retry depth",
		"codex.hint.sampling_requests":         "%s / sampling requests",
		"codex.hint.completed_turns":           "%s / completed turns",
		"codex.hint.network_errors":            "timeout / DNS / TLS / 5xx",
		"codex.hint.recovery_depth":            "automatic recovery depth",
		"codex.timeline_title":                 "Codex network issue timeline",
		"codex.timeline_unit":                  "Count",
		"codex.series.retry":                   "Stream retries",
		"codex.series.network":                 "Network errors",
		"codex.tooltip.retry":                  "%d events / %d sampling requests",
		"report.error.end_before_start":        "end time is before start time: start=%s end=%s",
		"report.error.parse_template":          "parse report template failed: %v",
		"report.error.render_live":             "render live page failed: %v",
		"report.error.create_file":             "create report file failed: %v",
		"report.error.render_file":             "render report failed: %v",
		"report.error.start_ui":                "start web UI failed: %v",
		"report.error.encode_response":         "encode response failed: %v",
		"report.error.split_addr":              "parse listen address failed: %v",
		"report.error.parse_port":              "parse listen port failed: %v",
		"report.error.listen_addr":             "listen on %s failed: %v",
		"report.error.no_fallback_port":        "default address %s is in use and the next 20 ports are unavailable",
		"report.error.parse_start":             "parse start failed: %v",
		"report.error.missing_end":             "missing end parameter",
		"report.error.parse_end":               "parse end failed: %v",
		"report.error.invalid_range":           "cannot parse time range: %s",
		"report.error.invalid_range_duration":  "cannot parse time range: %v",
		"report.error.positive_range":          "time range must be greater than 0",
		"report.error.unsupported_time_format": "unsupported time format: %s",
	},
	Chinese: {
		"app.usage": `netcheck 用法:

  netcheck [--lang en|zh-CN]
  netcheck monitor [--config path] [--duration 5m] [--lang en|zh-CN]
  netcheck report [--config path] [--since 24h] [--start 2026-04-10T09:00 --end 2026-04-10T18:00] [--output report.html] [--lang en|zh-CN]
  netcheck ui [--config path] [--addr 0.0.0.0:8765] [--lang en|zh-CN]
  netcheck clear [--config path] [--lang en|zh-CN]
  netcheck init-config [--output path] [--force] [--lang en|zh-CN]`,
		"cli.unknown_command":                  "未知命令",
		"cli.flag.config":                      "配置文件路径",
		"cli.flag.duration":                    "运行时长，默认持续运行直到收到退出信号",
		"cli.flag.since":                       "时间范围，例如 24h、7d",
		"cli.flag.start":                       "开始时间，支持 RFC3339 或 2006-01-02T15:04",
		"cli.flag.end":                         "结束时间，支持 RFC3339 或 2006-01-02T15:04",
		"cli.flag.output":                      "报表输出路径",
		"cli.flag.addr":                        "监听地址",
		"cli.flag.force":                       "覆盖已有配置",
		"cli.flag.lang":                        "语言：en 或 zh-CN",
		"cli.error.missing_lang_value":         "缺少 --lang 参数值",
		"cli.error.invalid_since":              "无法解析 since 参数: %s",
		"cli.error.invalid_since_duration":     "无法解析 since 参数: %v",
		"cli.error.positive_since":             "since 参数必须大于 0",
		"cli.error.custom_range_pair":          "使用自定义时间时，必须同时提供 --start 和 --end",
		"cli.error.parse_start":                "解析开始时间失败: %v",
		"cli.error.parse_end":                  "解析结束时间失败: %v",
		"cli.error.end_before_start":           "结束时间不能早于开始时间",
		"cli.error.unsupported_time_format":    "不支持的时间格式: %s",
		"cli.error.create_report_dir":          "创建报表目录失败: %v",
		"cli.config_written":                   "已写入默认配置: %s\n",
		"cli.ui_ready":                         "netcheck UI 已启动，监听地址: %s",
		"cli.ui_ready_local":                   "netcheck UI 已启动，监听地址: %s，本机访问: %s",
		"cli.ui_port_changed":                  "netcheck UI 已启动，默认监听地址 %s 已被占用，已切换到: %s",
		"cli.ui_port_changed_local":            "netcheck UI 已启动，默认监听地址 %s 已被占用，已切换到: %s，本机访问: %s",
		"clear.no_database":                    "数据库文件不存在，无需清理: %s\n",
		"clear.removed":                        "已清理数据库文件 %d 个: %s\n",
		"monitor.started":                      "netcheck 已启动，数据库: %s\n",
		"monitor.sampling":                     "网关采样: %ds 一次，国内下载: %ds 一次，国外下载: %ds 一次\n",
		"monitor.download_budget":              "估算日下载流量: 国内 %.1fMB，国外 %.1fMB\n",
		"monitor.background_error":             "[%s] 背景错误: %v\n",
		"monitor.degraded_start":               "[%s] 异常开始 [%s] %s (%s)\n",
		"monitor.degraded_resolved":            "[%s] 异常恢复 [%s] %s\n",
		"monitor.no_evidence":                  "无额外证据",
		"monitor.speedtest_failed":             "[%s] 测速 [%s] 失败: %s\n",
		"monitor.speedtest":                    "[%s] 测速 [%s] %.2f Mbps\n",
		"state.local_insufficient":             "本地链路样本不足",
		"state.local_summary":                  "网关 avg=%.1fms p95=%.1fms jitter=%.1fms loss=%.0f%%",
		"state.remote_summary":                 "%s 延迟 avg=%.1fms 失败率=%.0f%% dl=%.2fMbps",
		"evidence.jitter":                      "抖动 %.1fms > %.1fms",
		"evidence.loss":                        "丢包率 %.0f%% > %.0f%%",
		"evidence.avg_latency":                 "平均延迟 %.1fms > %.1fms",
		"evidence.failure_rate":                "失败率 %.0f%% > %.0f%%",
		"evidence.avg_download":                "平均下载速率 %.2fMbps < %.2fMbps",
		"common.no_data":                       "无数据",
		"ui.title":                             "netcheck 网络质量面板",
		"ui.range":                             "时间范围",
		"ui.range.30m":                         "最近 30 分钟",
		"ui.range.1h":                          "最近 1 小时",
		"ui.range.6h":                          "最近 6 小时",
		"ui.range.24h":                         "最近 24 小时",
		"ui.range.7d":                          "最近 7 天",
		"ui.apply_custom":                      "应用自定义范围",
		"ui.incident_overview":                 "异常概览",
		"ui.link_incidents":                    "链路异常统计",
		"ui.event_timeline":                    "事件时间轴（最近 10 条）",
		"ui.updated_at":                        "更新时间：%s",
		"ui.alert.custom_range":                "请同时填写开始和结束时间",
		"ui.incident_count":                    "异常次数",
		"ui.longest_duration":                  "最长持续",
		"ui.empty_causes":                      "当前时间范围内没有异常归因数据。",
		"ui.empty_events":                      "当前时间范围内没有事件记录。",
		"ui.table.category":                    "类别",
		"ui.table.total_duration":              "累计时长",
		"ui.table.status":                      "状态",
		"ui.table.summary":                     "摘要",
		"ui.table.evidence":                    "证据",
		"ui.table.window_start":                "窗口内开始",
		"ui.table.window_end":                  "窗口内结束",
		"ui.table.window_duration":             "窗口内持续",
		"ui.tooltip.time":                      "时间",
		"ui.tooltip.value":                     "数值",
		"layer.gateway":                        "网关",
		"layer.domestic":                       "国内",
		"layer.international":                  "国外",
		"cause.root":                           "归因",
		"cause.local":                          "本地链路",
		"cause.domestic":                       "国内链路",
		"cause.international":                  "国外链路",
		"cause.global":                         "全局异常",
		"cause.degraded":                       "异常",
		"summary.gateway":                      "网关",
		"summary.domestic":                     "国内",
		"summary.international":                "国外",
		"metric.rtt":                           "RTT",
		"metric.average":                       "平均",
		"metric.average_jitter":                "平均抖动",
		"metric.loss_rate":                     "丢包率",
		"metric.latency":                       "延迟",
		"metric.download":                      "下载",
		"metric.failure_rate":                  "失败率",
		"group.gateway_quality":                "网关质量",
		"group.domestic_quality":               "国内质量",
		"group.international_quality":          "国外质量",
		"chart.gateway_rtt":                    "网关 RTT",
		"chart.gateway_loss":                   "网关丢包率",
		"chart.domestic_rtt":                   "国内访问延迟",
		"chart.domestic_failure":               "国内失败率",
		"chart.domestic_download":              "国内下载速率",
		"chart.international_rtt":              "国外访问延迟",
		"chart.international_failure":          "国外失败率",
		"chart.international_download":         "国外下载速率",
		"unit.ratio":                           "比例",
		"event.before_window":                  "（窗口前开始）",
		"event.ongoing":                        "（进行中）",
		"event.after_window":                   "（窗口后结束）",
		"codex.error.empty_path":               "Codex 日志路径为空",
		"codex.error.missing_log":              "未检测到 Codex 日志",
		"codex.error.read_log":                 "读取 Codex 日志失败: %v",
		"codex.error.scan_log":                 "扫描 Codex 日志失败: %v",
		"codex.kind.stream_retry":              "响应流断开重试",
		"codex.kind.tool_error":                "本地工具错误",
		"codex.kind.rollout_record_error":      "会话记录错误",
		"codex.kind.apps_or_tool_403":          "Apps/工具建议 403",
		"codex.kind.network_candidate":         "网络错误",
		"codex.kind.unknown_error":             "未知错误",
		"codex.kind.unknown_warning":           "未知警告",
		"codex.noise.plugin_icon":              "插件/Skill 图标配置告警",
		"codex.noise.default_prompt":           "插件默认 Prompt 配置告警",
		"codex.noise.file_watcher":             "文件监听释放告警",
		"codex.noise.clipboard":                "剪贴板图片读取告警",
		"codex.noise.other":                    "其他降噪告警",
		"codex.title":                          "Codex 网络稳定性",
		"codex.empty":                          "未检测到 Codex 日志。",
		"codex.loading":                        "正在分析 Codex 本地日志...",
		"codex.clamped":                        "已按最近 24 小时统计",
		"codex.metric.retry":                   "断流重试",
		"codex.metric.affected_turns":          "受影响 turn",
		"codex.metric.network_errors":          "网络错误",
		"codex.metric.max_retry":               "最大重试深度",
		"codex.hint.sampling_requests":         "%s / 采样请求",
		"codex.hint.completed_turns":           "%s / 完成 turn",
		"codex.hint.network_errors":            "timeout / DNS / TLS / 5xx",
		"codex.hint.recovery_depth":            "自动恢复深度",
		"codex.timeline_title":                 "Codex 网络异常时间轴",
		"codex.timeline_unit":                  "次数",
		"codex.series.retry":                   "断流重试",
		"codex.series.network":                 "网络错误",
		"codex.tooltip.retry":                  "%d 次 / %d 次采样",
		"report.error.end_before_start":        "结束时间早于开始时间: start=%s end=%s",
		"report.error.parse_template":          "解析报表模板失败: %v",
		"report.error.render_live":             "渲染动态页面失败: %v",
		"report.error.create_file":             "创建报表文件失败: %v",
		"report.error.render_file":             "渲染报表失败: %v",
		"report.error.start_ui":                "启动 Web UI 失败: %v",
		"report.error.encode_response":         "序列化响应失败: %v",
		"report.error.split_addr":              "解析监听地址失败: %v",
		"report.error.parse_port":              "解析监听端口失败: %v",
		"report.error.listen_addr":             "监听地址 %s 失败: %v",
		"report.error.no_fallback_port":        "默认地址 %s 已被占用，且后续 20 个端口也不可用",
		"report.error.parse_start":             "解析 start 失败: %v",
		"report.error.missing_end":             "缺少 end 参数",
		"report.error.parse_end":               "解析 end 失败: %v",
		"report.error.invalid_range":           "无法解析时间范围: %s",
		"report.error.invalid_range_duration":  "无法解析时间范围: %v",
		"report.error.positive_range":          "时间范围必须大于 0",
		"report.error.unsupported_time_format": "不支持的时间格式: %s",
	},
}
