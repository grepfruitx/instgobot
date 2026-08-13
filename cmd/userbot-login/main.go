package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/grepfruitx/instgobot/internal/config"
	"github.com/grepfruitx/instgobot/internal/userbot"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "error", err)
		os.Exit(1)
	}

	if err := userbot.Login(context.Background(), cfg); err != nil {
		slog.Error("login failed", "error", err)
		os.Exit(1)
	}

	slog.Info("userbot login successful", "session_path", cfg.UserbotSessionPath)
}
