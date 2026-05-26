package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"yaran-go/internal/tui"
	"yaran-go/internal/yaran"
)

func main() {
	dbPath := defaultDBPath()
	showVersion := false

	flag.StringVar(&dbPath, "db-path", dbPath, "Path to the SQLite database file.")
	flag.BoolVar(&showVersion, "version", false, "Print the current Yaran version and exit.")
	flag.Parse()

	if showVersion {
		fmt.Printf("Current version: %s\n", yaran.Version)
		return
	}

	service, err := yaran.NewService(dbPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer service.Close()

	if err := service.InitializeDatabase(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	app := tui.New(service)
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func defaultDBPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".yaran.db"
	}
	return filepath.Join(home, ".yaran.db")
}
