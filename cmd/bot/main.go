package main

import (
	"log/slog"
	"os"

	"github.com/grepfruitx/instgobot/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "error", err)
		os.Exit(1)
	}
	_ = cfg

	slog.Info("instgobot starting")
}
