package main

import (
	"flag"
	"fmt"
	"os"

	"aurum/internal/common/config"
	"aurum/internal/common/logging"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	// Parse flags
	flag.Parse()
	args := flag.Args()

	if len(args) < 1 {
		fmt.Println("Usage: migrate <command>")
		fmt.Println("Commands:")
		fmt.Println("  up       Apply all pending migrations")
		fmt.Println("  down     Rollback the last migration")
		fmt.Println("  drop     Drop all tables (DANGEROUS)")
		fmt.Println("  version  Show current migration version")
		os.Exit(1)
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Setup logging
	logging.Setup(logging.Config{
		Level:  cfg.LogLevel,
		Format: cfg.LogFormat,
	})

	// Create migrator
	m, err := migrate.New("file://migrations", cfg.DatabaseURL)
	if err != nil {
		logging.Error("Failed to create migrator", "error", err)
		os.Exit(1)
	}
	defer m.Close()

	command := args[0]

	switch command {
	case "up":
		logging.Info("Applying migrations")
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			logging.Error("Migration failed", "error", err)
			os.Exit(1)
		}
		logging.Info("Migrations applied successfully")

	case "down":
		logging.Info("Rolling back last migration")
		if err := m.Steps(-1); err != nil {
			logging.Error("Rollback failed", "error", err)
			os.Exit(1)
		}
		logging.Info("Rollback completed")

	case "drop":
		logging.Warn("Dropping all tables")
		if err := m.Drop(); err != nil {
			logging.Error("Drop failed", "error", err)
			os.Exit(1)
		}
		logging.Info("All tables dropped")

	case "version":
		version, dirty, err := m.Version()
		if err != nil {
			logging.Error("Failed to get version", "error", err)
			os.Exit(1)
		}
		fmt.Printf("Version: %d, Dirty: %v\n", version, dirty)

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		os.Exit(1)
	}
}
