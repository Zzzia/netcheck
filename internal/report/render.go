package report

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"time"
)

//go:embed report.html.tmpl
var reportTemplate string

//go:embed report.js
var reportScript string

//go:embed report_codex.js
var codexScript string

func Generate(dbPath string, start, end time.Time, output string) error {
	payload, err := LoadData(dbPath, start, end)
	if err != nil {
		return err
	}
	codex := buildCodexReport(start, end)
	payload.Codex = &codex
	return writePage(output, templatePageData{
		LiveMode:     false,
		InitialJSON:  mustJSON(payload),
		DefaultMode:  "static",
		ReportScript: template.JS(reportScript),
		CodexScript:  template.JS(codexScript),
	})
}

func RenderLivePage() ([]byte, error) {
	tmpl, err := template.New("report").Parse(reportTemplate)
	if err != nil {
		return nil, fmt.Errorf("解析报表模板失败: %w", err)
	}
	var builder bytes.Buffer
	if err := tmpl.Execute(&builder, templatePageData{
		LiveMode:     true,
		InitialJSON:  template.JS("null"),
		DefaultMode:  "1h",
		ReportScript: template.JS(reportScript),
		CodexScript:  template.JS(codexScript),
	}); err != nil {
		return nil, fmt.Errorf("渲染动态页面失败: %w", err)
	}
	return builder.Bytes(), nil
}

func writePage(output string, page templatePageData) error {
	tmpl, err := template.New("report").Parse(reportTemplate)
	if err != nil {
		return fmt.Errorf("解析报表模板失败: %w", err)
	}
	file, err := os.Create(output)
	if err != nil {
		return fmt.Errorf("创建报表文件失败: %w", err)
	}
	defer file.Close()
	if err := tmpl.Execute(file, page); err != nil {
		return fmt.Errorf("渲染报表失败: %w", err)
	}
	return nil
}

func mustJSON(value any) template.JS {
	encoded, err := json.Marshal(value)
	if err != nil {
		return template.JS("null")
	}
	return template.JS(encoded)
}
