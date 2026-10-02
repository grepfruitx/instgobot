package admin

import (
	"context"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"

	"github.com/grepfruitx/instgobot/internal/config"
	"github.com/grepfruitx/instgobot/internal/messages"
	"github.com/grepfruitx/instgobot/internal/store"
	"github.com/grepfruitx/instgobot/internal/telegramapi"
	"github.com/grepfruitx/instgobot/internal/userbot"
)

type Handler struct {
	b         *bot.Bot
	st        *store.Store
	uc        *userbot.Client
	restart   func()
	ytDlpPath string
}

func New(b *bot.Bot, st *store.Store, uc *userbot.Client, restart func(), ytDlpPath string) *Handler {
	return &Handler{b: b, st: st, uc: uc, restart: restart, ytDlpPath: ytDlpPath}
}

func (h *Handler) send(ctx context.Context, chatID int64, text string) {
	_, _ = telegramapi.SendText(ctx, h.b, chatID, text)
}

func (h *Handler) sendChunked(ctx context.Context, chatID int64, text string) {
	for _, chunk := range messages.SplitMessage(text, messages.DefaultMaxLength) {
		h.send(ctx, chatID, chunk)
	}
}

func displayName(username, firstName *string) string {
	if username != nil && *username != "" {
		return "@" + *username
	}
	if firstName != nil && *firstName != "" {
		return *firstName
	}
	return "Без имени"
}

func parseLimit(args []string, fallback int) int {
	if len(args) == 0 {
		return fallback
	}
	if n, err := strconv.Atoi(args[0]); err == nil {
		return n
	}
	return fallback
}

func (h *Handler) HandleCommand(ctx context.Context, chatID int64, message string, userID int64) bool {
	if !config.IsAdmin(userID) {
		return false
	}

	fields := strings.Fields(message)
	if len(fields) == 0 {
		return false
	}
	command := strings.ToLower(fields[0])
	args := fields[1:]

	switch command {
	case "/users":
		h.handleUsers(ctx, chatID, args)
	case "/stats":
		h.handleStats(ctx, chatID)
	case "/top":
		h.handleTopUsers(ctx, chatID, args)
	case "/errors":
		h.handleErrors(ctx, chatID, args)
	case "/platforms":
		h.handlePlatforms(ctx, chatID)
	case "/announcecount":
		h.handleAnnounceCount(ctx, chatID)
	case "/ah":
		h.handleHelp(ctx, chatID)
	case "/announce":
		h.handleAnnounce(ctx, chatID, message)
	case "/poff":
		h.handlePlatformToggle(ctx, chatID, args, true)
	case "/pon":
		h.handlePlatformToggle(ctx, chatID, args, false)
	case "/pstatus":
		h.handlePlatformStatus(ctx, chatID)
	case "/errortop":
		h.handleErrorTop(ctx, chatID, args)
	case "/cachestats":
		h.handleCacheStats(ctx, chatID)
	case "/retention":
		h.handleRetention(ctx, chatID)
	case "/activity":
		h.handleActivity(ctx, chatID)
	case "/clearcache":
		h.handleClearCache(ctx, chatID, args)
	case "/ratelimits":
		h.handleRateLimitHits(ctx, chatID)
	case "/ban":
		h.handleBan(ctx, chatID, args)
	case "/unban":
		h.handleUnban(ctx, chatID, args)
	case "/banned":
		h.handleBannedList(ctx, chatID)
	case "/health":
		h.handleHealth(ctx, chatID)
	case "/restart":
		h.handleRestart(ctx, chatID)
	case "/ytdlp":
		h.handleYtDlp(ctx, chatID, args)
	case "/logs":
		h.handleLogs(ctx, chatID, args)
	default:
		return false
	}
	return true
}
