package report

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"time"

	"netcheck/internal/i18n"
)

//go:embed report.html.tmpl
var reportTemplate string

//go:embed report.js
var reportScript string

//go:embed report_codex.js
var codexScript string

func Generate(dbPath string, start, end time.Time, output string) error {
	return GenerateForLang(dbPath, start, end, output, i18n.English)
}

func GenerateForLang(dbPath string, start, end time.Time, output string, lang i18n.Lang) error {
	localizer := i18n.New(lang)
	payload, err := LoadDataForLang(dbPath, start, end, localizer.Lang())
	if err != nil {
		return err
	}
	codex := buildCodexReportForLang(start, end, localizer.Lang())
	payload.Codex = &codex
	return writePage(output, templatePageData{
		LiveMode:     false,
		InitialJSON:  mustJSON(payload),
		DefaultMode:  "static",
		Lang:         localizer.Code(),
		Translations: template.JS(i18n.MessagesJSON()),
		ReportScript: template.JS(reportScript),
		CodexScript:  template.JS(codexScript),
	})
}

func RenderLivePage() ([]byte, error) {
	return RenderLivePageForLang(i18n.English)
}

func RenderLivePageForLang(lang i18n.Lang) ([]byte, error) {
	localizer := i18n.New(lang)
	tmpl, err := template.New("report").Parse(reportTemplate)
	if err != nil {
		return nil, fmt.Errorf(localizer.T("report.error.parse_template"), err)
	}
	var builder bytes.Buffer
	if err := tmpl.Execute(&builder, templatePageData{
		LiveMode:     true,
		InitialJSON:  template.JS("null"),
		DefaultMode:  "1h",
		Lang:         localizer.Code(),
		Translations: template.JS(i18n.MessagesJSON()),
		ReportScript: template.JS(reportScript),
		CodexScript:  template.JS(codexScript),
	}); err != nil {
		return nil, fmt.Errorf(localizer.T("report.error.render_live"), err)
	}
	return builder.Bytes(), nil
}

func writePage(output string, page templatePageData) error {
	localizer := i18n.New(i18n.Parse(page.Lang))
	tmpl, err := template.New("report").Parse(reportTemplate)
	if err != nil {
		return fmt.Errorf(localizer.T("report.error.parse_template"), err)
	}
	file, err := os.Create(output)
	if err != nil {
		return fmt.Errorf(localizer.T("report.error.create_file"), err)
	}
	defer file.Close()
	if err := tmpl.Execute(file, page); err != nil {
		return fmt.Errorf(localizer.T("report.error.render_file"), err)
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
