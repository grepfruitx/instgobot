package telegramapi

import (
	"github.com/go-telegram/bot"

	"github.com/grepfruitx/instgobot/internal/config"
)

func New(cfg *config.Config, opts ...bot.Option) (*bot.Bot, error) {
	allOpts := append([]bot.Option{bot.WithServerURL(cfg.LocalBotAPIURL)}, opts...)
	return bot.New(cfg.TelegramBotToken, allOpts...)
}
