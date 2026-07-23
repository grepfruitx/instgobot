package messages

import (
	"strings"

	"github.com/grepfruitx/instgobot/internal/config"
)

var StartMessage = strings.Join([]string{
	"Привет! ",
	"",
	"Я бот для скачивания медиа из социальных сетей. ",
	"",
	"Я могу:",
	"",
	"• скачивать сторис и посты из Telegram (@username или ссылка t.me/...)",
	"• скачивать рилсы, посты и сторис с Instagram",
	"• скачивать посты, видео и изображения из Twitter (X)",
	"• скачивать посты из Threads (картинки и видео)",
	"• скачивать видео из TikTok",
	"• скачивать видео из Facebook",
	"• скачивать любые видео с YouTube (Shorts, клипы, полные видео)",
	"",
	config.BotTag,
}, "\n")

var HelpMessage = strings.Join([]string{
	"Отправьте ссылку на медиа или юзернейм телеграм пользователя для скачивания контента.",
	"",
	"Поддерживаемые платформы:",
	"",
	"• Telegram (@username, t.me/user/s/300 — сторис, t.me/channel/300 — пост)",
	"• Instagram (рилсы, посты, сторис)",
	"• Threads (картинки и видео)",
	"• Twitter (X) (посты, картинки и видео)",
	"• Facebook (видео)",
	"• TikTok",
	"• YouTube (Shorts, клипы, полные видео)",
	"",
	"Пример: https://www.instagram.com/reel/DKKPO_gyGAg/?igsh=ejVqOTBpNm85OHA0",
	"",
	"Лимит: 5 запросов в минуту на медиа контент по ссылке",
	"Телеграм Сторис лимит: 1 запрос раз в 3 минуты",
	"",
	"/newsletter - управление подпиской на рассылку",
	"/feat [предложение] - предложить новую функцию",
	"",
	config.BotTag,
}, "\n")
