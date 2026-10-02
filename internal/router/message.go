package router

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/grepfruitx/instgobot/internal/config"
	"github.com/grepfruitx/instgobot/internal/messages"
	"github.com/grepfruitx/instgobot/internal/platform"
	"github.com/grepfruitx/instgobot/internal/ratelimit"
	"github.com/grepfruitx/instgobot/internal/snapsave"
	"github.com/grepfruitx/instgobot/internal/telegramapi"
	"github.com/grepfruitx/instgobot/internal/threads"
)

var telegramUsernameRe = regexp.MustCompile(`^@\w{5,32}$`)

func (r *Router) send(ctx context.Context, b *bot.Bot, chatID int64, text string) {
	_, _ = telegramapi.SendText(ctx, b, chatID, text)
}

func (r *Router) handleMessage(ctx context.Context, b *bot.Bot, msg *models.Message) {
	text := msg.Text
	if text == "" {
		return
	}

	chatID := msg.Chat.ID
	var userID int64
	var username, firstName *string
	if msg.From != nil {
		userID = msg.From.ID
		if msg.From.Username != "" {
			username = &msg.From.Username
		}
		if msg.From.FirstName != "" {
			firstName = &msg.From.FirstName
		}
	}

	if !config.IsAdmin(userID) {
		banned, err := r.st.IsBanned(chatID)
		if err != nil {
			slog.Error("ban check failed, dropping message", "chat_id", chatID, "error", err)
			return
		}
		if banned {
			return
		}
	}

	switch {
	case text == "/start":
		r.send(ctx, b, chatID, messages.StartMessage)
		return
	case text == "/help":
		r.send(ctx, b, chatID, messages.HelpMessage)
		return
	case text == "/newsletter":
		messages.ProcessNewsletterToggle(ctx, b, r.st, chatID)
		return
	case strings.HasPrefix(text, "/feat"):
		messages.ProcessFeatureRequest(ctx, b, chatID, text, r.adminUsername, username, firstName)
		return
	}

	isValidURL := strings.Contains(text, "https://") || strings.Contains(text, "http://")
	isAdminCommand := config.IsAdmin(userID) && strings.HasPrefix(text, "/")
	isTelegramUsername := telegramUsernameRe.MatchString(text)

	if !isValidURL && !isAdminCommand && !isTelegramUsername {
		r.send(ctx, b, chatID, messages.HelpMessage)
		return
	}

	isTelegramContent := isTelegramUsername || (isValidURL && platform.IsTelegramLink(text))
	if isTelegramContent {
		if r.rejectIfPlatformDisabled(ctx, b, chatID, text, userID) {
			return
		}
		effectiveUserID := userID
		if effectiveUserID == 0 {
			effectiveUserID = chatID
		}
		r.handleTelegramContent(ctx, b, text, chatID, effectiveUserID)
		return
	}

	r.handleMediaURL(ctx, b, chatID, userID, text, username, firstName)
}

func (r *Router) handleTelegramContent(ctx context.Context, b *bot.Bot, text string, chatID, userID int64) {
	if rl := r.limiter.CheckTelegramStories(userID, config.IsAdmin(userID)); !rl.Allowed {
		_ = r.st.RecordRateLimitHit("tgstories")
		minutesLeft := int(math.Ceil(time.Until(rl.ResetTime).Minutes()))
		r.send(ctx, b, chatID, fmt.Sprintf(messages.TelegramStoriesLimitFmt, minutesLeft))
		return
	}

	if telegramUsernameRe.MatchString(text) {
		r.userHandler.DownloadStories(ctx, chatID, strings.TrimPrefix(text, "@"))
		return
	}

	parsed, ok := platform.ParseTelegramLink(text)
	if !ok {
		r.send(ctx, b, chatID, messages.TelegramLinkUnparsed)
		return
	}

	switch parsed.Type {
	case platform.TelegramLinkPrivatePost:
		r.userHandler.DownloadPrivateTelegramPost(ctx, chatID, parsed.ChannelID, int(parsed.MessageID))
	case platform.TelegramLinkStory:
		r.userHandler.DownloadStoryByID(ctx, chatID, parsed.Username, int(parsed.ID))
	case platform.TelegramLinkPost:
		r.userHandler.DownloadTelegramPost(ctx, chatID, parsed.Username, int(parsed.ID))
	default:
		r.userHandler.DownloadStories(ctx, chatID, parsed.Username)
	}
}

func (r *Router) handleMediaURL(ctx context.Context, b *bot.Bot, chatID int64, userID int64, text string, username, firstName *string) {
	_, _ = r.st.UpsertUser(chatID, username, firstName)

	if config.IsAdmin(userID) {
		if r.adminHandler.HandleCommand(ctx, chatID, text, userID) {
			return
		}
	}

	if !config.IsAdmin(userID) {
		if rl := r.limiter.CheckGeneral(chatID); !rl.Allowed {
			_ = r.st.RecordRateLimitHit("general")
			ratelimit.SendGeneralLimitMessage(ctx, b, chatID, rl.ResetTime)
			return
		}
	}

	if r.rejectIfPlatformDisabled(ctx, b, chatID, text, userID) {
		return
	}

	switch {
	case platform.IsYoutubeShortsLink(text):
		r.ytHandler.ProcessShorts(ctx, chatID, text, username, firstName)
	case platform.IsYoutubeLink(text):
		r.ytHandler.SendQualityPicker(ctx, chatID, userID, text, username)
	case platform.IsThreadsLink(text):
		threads.Process(ctx, b, r.st, chatID, text, username, firstName)
	default:
		snapsave.Process(ctx, b, r.st, chatID, text, r.adminUsername, username, firstName)
	}
}

func (r *Router) rejectIfPlatformDisabled(ctx context.Context, b *bot.Bot, chatID int64, text string, userID int64) bool {
	if config.IsAdmin(userID) {
		return false
	}
	plat := platform.DetectPlatform(text)
	disabled, err := r.st.IsPlatformDisabled(plat)
	if err != nil {
		slog.Error("platform status check failed, allowing request", "platform", plat, "error", err)
		return false
	}
	if !disabled {
		return false
	}
	_ = r.st.JoinWaitlist(chatID, plat)
	r.send(ctx, b, chatID, messages.PlatformDisabled)
	return true
}
