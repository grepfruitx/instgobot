package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/grepfruitx/instgobot/internal/admin"
	"github.com/grepfruitx/instgobot/internal/cache"
	"github.com/grepfruitx/instgobot/internal/config"
	"github.com/grepfruitx/instgobot/internal/messages"
	"github.com/grepfruitx/instgobot/internal/ratelimit"
	"github.com/grepfruitx/instgobot/internal/router"
	"github.com/grepfruitx/instgobot/internal/store"
	"github.com/grepfruitx/instgobot/internal/telegramapi"
	"github.com/grepfruitx/instgobot/internal/userbot"
	"github.com/grepfruitx/instgobot/internal/youtube"
)

const (
	userbotStartupTimeout = 30 * time.Second
	shutdownDrainTimeout  = 15 * time.Minute
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "error", err)
		os.Exit(1)
	}

	cleanupTmpFiles()

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		slog.Error("store open failed", "error", err)
		os.Exit(1)
	}
	defer st.Close()

	rdb, err := cache.New(cfg.RedisAddr)
	if err != nil {
		slog.Error("redis connect failed", "error", err)
		os.Exit(1)
	}
	defer rdb.Close()

	limiter := ratelimit.New(rdb)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	uc := userbot.NewClient(cfg)
	startUserbot(ctx, uc)

	var rt *router.Router
	b, err := telegramapi.New(cfg, tgbot.WithDefaultHandler(func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
		rt.Handle(ctx, b, update)
	}))
	if err != nil {
		slog.Error("bot init failed", "error", err)
		os.Exit(1)
	}

	go func() {
		sig := <-sigCh
		slog.Info("shutdown signal received", "signal", sig)
		messages.NotifyAdmins(context.Background(), b, fmt.Sprintf("Bot is shutting down due to %s signal", sig))
		cancel()
	}()

	ytHandler := youtube.New(b, st, rdb, limiter, cfg)
	adminHandler := admin.New(b, st)
	userHandler := userbot.New(uc, b, st)
	rt = router.New(st, userHandler, ytHandler, adminHandler, limiter, cfg.AdminUsername)

	slog.Info("instgobot starting")
	b.Start(ctx)

	slog.Info("draining in-flight downloads before exit")
	drained := make(chan struct{})
	go func() {
		youtube.WaitForActiveDownloads()
		close(drained)
	}()
	select {
	case <-drained:
	case <-time.After(shutdownDrainTimeout):
		slog.Warn("shutdown drain timed out, exiting anyway")
	}

	slog.Info("instgobot stopped")
}

// startUserbot connects the MTProto client in the background. A failure
// here degrades Telegram stories/posts gracefully (each request just errors)
// rather than taking down Instagram/TikTok/YouTube/Threads with it, so it
// only logs — never os.Exit's.
func startUserbot(ctx context.Context, uc *userbot.Client) {
	ready := make(chan struct{})
	errCh := make(chan error, 1)
	go func() {
		errCh <- uc.Run(ctx, func() { close(ready) })
	}()

	select {
	case <-ready:
		slog.Info("userbot connected")
		syncCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		if err := uc.SyncDialogs(syncCtx); err != nil {
			slog.Warn("userbot dialog sync failed", "error", err)
		}
	case err := <-errCh:
		slog.Error("userbot failed to start — Telegram stories/posts will be unavailable", "error", err)
	case <-time.After(userbotStartupTimeout):
		slog.Error("userbot startup timed out — Telegram stories/posts will be unavailable")
	}
}

func cleanupTmpFiles() {
	entries, err := os.ReadDir(os.TempDir())
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "yt_") || strings.HasPrefix(name, "tgmedia_") {
			os.RemoveAll(filepath.Join(os.TempDir(), name))
		}
	}
}
