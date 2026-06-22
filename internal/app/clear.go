package app

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/Zzzia/netcheck/internal/config"
	"github.com/Zzzia/netcheck/internal/i18n"
)

func runClear(args []string) error {
	return runClearForLang(args, i18n.English)
}

func runClearForLang(args []string, lang i18n.Lang) error {
	localizer := i18n.New(lang)
	fs := flag.NewFlagSet("clear", flag.ContinueOnError)
	configPath := fs.String("config", "", localizer.T("cli.flag.config"))
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
		fmt.Printf(localizer.T("clear.no_database"), cfg.DBPath)
		return nil
	}
	fmt.Printf(localizer.T("clear.removed"), removedFiles, cfg.DBPath)
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
			return removedFiles, fmt.Errorf("remove database file %s failed: %w", path, err)
		}
		removedFiles++
	}
	return removedFiles, nil
}
