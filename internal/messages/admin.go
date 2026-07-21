package messages

import (
	"context"

	"github.com/go-telegram/bot"

	"github.com/grepfruitx/instgobot/internal/config"
	"github.com/grepfruitx/instgobot/internal/telegramapi"
)

func NotifyAdmins(ctx context.Context, b *bot.Bot, text string) {
	for _, adminID := range config.AdminUserIDs {
		_, _ = telegramapi.SendText(ctx, b, adminID, text)
	}
}
