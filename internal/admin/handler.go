package admin

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot"

	"github.com/grepfruitx/instgobot/internal/config"
	"github.com/grepfruitx/instgobot/internal/messages"
	"github.com/grepfruitx/instgobot/internal/store"
	"github.com/grepfruitx/instgobot/internal/telegramapi"
)

type Handler struct {
	b  *bot.Bot
	st *store.Store
}

func New(b *bot.Bot, st *store.Store) *Handler {
	return &Handler{b: b, st: st}
}

func (h *Handler) send(ctx context.Context, chatID int64, text string) {
	_, _ = telegramapi.SafeSendMessage(ctx, h.b, &bot.SendMessageParams{ChatID: chatID, Text: text})
}

func (h *Handler) sendChunked(ctx context.Context, chatID int64, text string) {
	for _, chunk := range messages.SplitMessage(text, messages.DefaultMaxLength) {
		h.send(ctx, chatID, chunk)
	}
}

func nowRu() string {
	return time.Now().Format("02.01.2006, 15:04:05")
}

// formatRuDateTime parses a store timestamp (nowISO's format) and renders it
// like TS's toLocaleString("ru-RU"); falls back to the raw string if
// parsing fails rather than dropping the value.
func formatRuDateTime(iso string) string {
	t, err := time.Parse("2006-01-02T15:04:05.000Z", iso)
	if err != nil {
		return iso
	}
	return t.Format("02.01.2006, 15:04:05")
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

// HandleCommand dispatches an admin command. It re-checks IsAdmin itself
// (matching the TS handler, which does the same even though callers already
// gate on it) and returns whether the message was recognized as one.
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
	default:
		return false
	}
	return true
}
