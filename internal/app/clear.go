package app

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"netcheck/internal/config"
)

func runClear(args []string) error {
	fs := flag.NewFlagSet("clear", flag.ContinueOnError)
	configPath := fs.String("config", "", "配置文件路径")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	removedFiles, err := clearDatabaseFiles(cfg.DBPath)
	if err != nil {
		return err
	}
	if removedFiles == 0 {
		fmt.Printf("数据库文件不存在，无需清理: %s\n", cfg.DBPath)
		return nil
	}
	fmt.Printf("已清理数据库文件 %d 个: %s\n", removedFiles, cfg.DBPath)
	return nil
}

func clearDatabaseFiles(dbPath string) (int, error) {
	paths := []string{
		dbPath,
		dbPath + "-wal",
		dbPath + "-shm",
	}
	removedFiles := 0
	for _, path := range paths {
		if err := os.Remove(path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return removedFiles, fmt.Errorf("删除数据库文件失败 %s: %w", path, err)
		}
		removedFiles++
	}
	return removedFiles, nil
}
