package report

import (
	"os"
	"path/filepath"
	"time"
)

const (
	codexLogSourceText   = "text"
	codexLogSourceSQLite = "sqlite"
)

type codexLogSource struct {
	kind    string
	path    string
	modTime time.Time
}

func defaultCodexLogSource() (codexLogSource, bool) {
	var selected codexLogSource
	if textPath := defaultCodexLogPath(); textPath != "" {
		if stat, err := os.Stat(textPath); err == nil && stat.Size() > 0 {
			selected = codexLogSource{kind: codexLogSourceText, path: textPath, modTime: stat.ModTime()}
		}
	}
	if sqliteSource, ok := latestCodexSQLiteLogSource(); ok && sqliteSource.modTime.After(selected.modTime) {
		selected = sqliteSource
	}
	return selected, selected.path != ""
}

func latestCodexSQLiteLogSource() (codexLogSource, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return codexLogSource{}, false
	}
	matches, err := filepath.Glob(filepath.Join(home, ".codex", "logs*.sqlite"))
	if err != nil {
		return codexLogSource{}, false
	}
	var selected codexLogSource
	for _, path := range matches {
		stat, err := os.Stat(path)
		if err != nil || stat.Size() == 0 {
			continue
		}
		if selected.path == "" || stat.ModTime().After(selected.modTime) {
			selected = codexLogSource{kind: codexLogSourceSQLite, path: path, modTime: stat.ModTime()}
		}
	}
	return selected, selected.path != ""
}
