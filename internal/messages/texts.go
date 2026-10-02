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

const (
	PlatformDisabled        = "Скачивание с этой платформы временно не работает. Мы уже занимаемся этим. Как только заработает — напишем."
	TelegramLinkUnparsed    = "Не удалось распознать ссылку Telegram."
	TelegramStoriesLimitFmt = "Лимит: 1 запрос раз в 3 минуты. Попробуйте снова через %d мин."

	YouTubeLimitFmt           = "Лимит: 1 загрузка в 3 минуты. Повторите через %d сек."
	YouTubePicker             = "<b>YouTube видео</b>\n\nВыберите формат:\n<i>Максимум в Telegram — 2 ГБ</i>"
	YouTubeAudioButton        = "Аудио"
	YouTubeLoadingButton      = "Загрузка..."
	YouTubeMenuFailed         = "Не удалось отправить меню."
	YouTubeWaitCurrent        = "Дождитесь окончания текущей загрузки."
	YouTubeSessionExpired     = "Сессия истекла. Отправьте ссылку заново."
	YouTubeDownloadFailed     = "Не удалось скачать. Попробуйте ещё раз."
	YouTubeQualityFallbackFmt = "%dp недоступно, скачиваю лучшее: %dp"
	YouTubeServerBusy         = "Сервер сейчас занят обработкой видео в высоком качестве. Попробуйте через минуту."
	YouTubeTooLarge           = "Видео больше 2 ГБ — Telegram не примет такой файл. Выберите качество пониже или аудио."

	SnapsaveFailedFmt   = "Не удалось скачать медиафайл.\nУбедитесь, что медиафайл существует и не является приватным.\nЕсли ошибка возникает многократно, пишите %s"
	MediaDownloadFailed = "Не удалось скачать медиа. Попробуйте еще раз."
	TweetImageFailed    = "Не удалось конвертировать твит в изображение."

	ThreadsDownloadFailed = "Не удалось скачать медиа с Threads. Попробуйте еще раз."
	ThreadsEmptyPost      = "В этом посте нет ни медиафайлов, ни текста."
	ThreadsTextOnly       = "В этом посте нет медиафайлов — только текст."
	ThreadsInvalidURL     = "Не удалось распознать ссылку на пост Threads. Скопируйте полную ссылку."

	StoriesErrorFmt       = "Ошибка при загрузке сторис. Попробуйте позже.\n%s"
	StoriesLoading        = "Загружаю сторис..."
	StoryNotFoundFmt      = "Сторис #%d не найдена у @%s.\n%s"
	StoryNoMediaFmt       = "Сторис #%d не содержит медиа.\n%s"
	StoryTooLargeFmt      = "Сторис слишком большой для загрузки (максимум 50MB).\n%s"
	StoriesNotFoundFmt    = "Не удалось найти публичные сторис у @%s. Возможно, пользователь скрыл свои сторис или у него нет публичных сторис.\n%s"
	StoriesMediaFailedFmt = "Сторис найдены, но не удалось загрузить медиа. Возможно, они недоступны.\n%s"

	ChannelNoAccessFmt    = "Нет доступа к каналу. Возможно, канал приватный или бот не является участником.\n%s"
	PostErrorFmt          = "Ошибка при загрузке поста. Попробуйте позже.\n%s"
	PostLoading           = "Загружаю пост..."
	PostNotFoundByUserFmt = "Пост не найден у @%s.\n%s"
	PostNotFoundFmt       = "Пост не найден.\n%s"
	PostNoMediaFmt        = "Пост не содержит медиа.\n%s"

	MediaURLMissingFmt    = "Не удалось получить URL %s."
	MediaTooLarge         = "Слишком большой файл для загрузки. Максимальный размер: 50MB."
	MediaFetchFailedFmt   = "Не удалось загрузить файл: %s"
	FetchResponseTimeout  = "Превышено время ожидания ответа сервера."
	FetchBodyTimeout      = "Превышено время ожидания загрузки файла."
	FetchTooManyRedirects = "Ссылка содержит слишком много перенаправлений."
	FetchBadStatusFmt     = "Сервер вернул ошибку %d."
)
