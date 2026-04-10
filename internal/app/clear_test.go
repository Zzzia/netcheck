package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClearDatabaseFilesRemovesMainAndWalFiles(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "netcheck.sqlite")
	for _, path := range []string{dbPath, dbPath + "-wal", dbPath + "-shm"} {
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatalf("准备测试文件失败: %v", err)
		}
	}

	removedFiles, err := clearDatabaseFiles(dbPath)
	if err != nil {
		t.Fatalf("清理数据库文件失败: %v", err)
	}
	if removedFiles != 3 {
		t.Fatalf("期望删除 3 个文件，实际为 %d", removedFiles)
	}
	for _, path := range []string{dbPath, dbPath + "-wal", dbPath + "-shm"} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("期望文件已被删除: %s", path)
		}
	}
}

func TestClearDatabaseFilesIgnoresMissingFiles(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "missing.sqlite")

	removedFiles, err := clearDatabaseFiles(dbPath)
	if err != nil {
		t.Fatalf("清理缺失文件时不应报错: %v", err)
	}
	if removedFiles != 0 {
		t.Fatalf("期望未删除任何文件，实际为 %d", removedFiles)
	}
}
