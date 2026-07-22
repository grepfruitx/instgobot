package config

import (
	"slices"
	"time"

	"github.com/caarlos0/env/v11"
)

const BotTag = "@instg_save_bot"

var AdminUserIDs = []int64{324025710, 542142955}

func IsAdmin(userID int64) bool {
	return slices.Contains(AdminUserIDs, userID)
}

const ruTimeLayout = "02.01.2006, 15:04:05"

var moscowLoc = time.FixedZone("MSK", 3*60*60)

func NowMoscowStr() string {
	return time.Now().In(moscowLoc).Format(ruTimeLayout)
}

func FormatMoscow(iso string) string {
	t, err := time.Parse("2006-01-02T15:04:05.000Z", iso)
	if err != nil {
		return iso
	}
	return t.In(moscowLoc).Format(ruTimeLayout)
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
