package config

import (
	"slices"

	"github.com/caarlos0/env/v11"
)

const BotTag = "@instg_save_bot"

var AdminUserIDs = []int64{324025710, 542142955}

func IsAdmin(userID int64) bool {
	return slices.Contains(AdminUserIDs, userID)
}

type Config struct {
	// Telegram Bot API
	TelegramBotToken string `env:"TELEGRAM_BOT,required"`
	LocalBotAPIURL   string `env:"LOCAL_BOT_API_URL" envDefault:"https://api.telegram.org"`
	AdminUsername    string `env:"ADMIN_USERNAME,required"`

	// MTProto user account (stories/private posts)
	TelegramAPIID      int    `env:"TELEGRAM_API_ID,required"`
	TelegramAPIHash    string `env:"TELEGRAM_API_HASH,required"`
	UserbotSessionPath string `env:"USERBOT_SESSION_PATH" envDefault:"/data/userbot.session"`

	// Storage
	DBPath    string `env:"DB_PATH" envDefault:"/data/bot_data.sqlite"`
	RedisAddr string `env:"REDIS_ADDR" envDefault:"localhost:6379"`

	// External tools
	YtDlpPath string `env:"YT_DLP_PATH" envDefault:"yt-dlp"`
}

func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
