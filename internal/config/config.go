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
	TelegramBotToken string `env:"TELEGRAM_BOT,required"`
	LocalBotAPIURL   string `env:"LOCAL_BOT_API_URL" envDefault:"https://api.telegram.org"`
	AdminUsername    string `env:"ADMIN_USERNAME,required"`

	TelegramAPIID      int    `env:"TELEGRAM_API_ID,required"`
	TelegramAPIHash    string `env:"TELEGRAM_API_HASH,required"`
	UserbotSessionPath string `env:"USERBOT_SESSION_PATH" envDefault:"/root/instgobot/userbot.session"`

	DBPath    string `env:"DB_PATH" envDefault:"/root/instgobot/bot_data.sqlite"`
	RedisAddr string `env:"REDIS_ADDR" envDefault:"localhost:6379"`

	YtDlpPath string `env:"YT_DLP_PATH" envDefault:"yt-dlp"`
}

func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
