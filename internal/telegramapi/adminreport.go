package telegramapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/go-telegram/bot"

	"github.com/grepfruitx/instgobot/internal/config"
)

var contextTitles = map[string]string{
	"youtube download":          "Ошибка загрузки YouTube",
	"youtube video send":        "Ошибка отправки YouTube видео",
	"snapsave download":         "Ошибка скачивания из соцсетей",
	"media check":               "Не найдены медиафайлы в ответе",
	"single video":              "Ошибка обработки одного видео",
	"single photo":              "Ошибка обработки одного фото",
	"sendMediaGroup videos":     "Ошибка отправки группы видео",
	"sendMediaGroup photos":     "Ошибка отправки группы фото",
	"tweet to image":            "Ошибка конвертации твита в изображение",
	"delete loading message":    "Не удалось удалить сообщение 'Загружаю...'",
	"main message handler":      "Общая ошибка обработки сообщения",
	"main function":             "Критическая ошибка бота",
	"telegram stories download": "Ошибка загрузки Telegram сторис",
	"telegram post download":    "Ошибка загрузки Telegram поста",
	"youtube quality picker":    "Ошибка меню выбора качества YouTube",
	"threads download":          "Ошибка скачивания из Threads",
}

func shouldSkipReport(err error) bool {
	if IsBotBlockedError(err) {
		return true
	}
	if strings.Contains(err.Error(), "413 Request Entity Too Large") {
		return true
	}
	var tooLarge *FileTooLargeError
	var fetchErr *MediaFetchError
	return errors.As(err, &tooLarge) || errors.As(err, &fetchErr)
}

func SendErrorToAdmin(ctx context.Context, b *bot.Bot, err error, errContext string, userMessage string, chatID *int64, username *string) {
	if shouldSkipReport(err) {
		return
	}

	contextTitle, ok := contextTitles[errContext]
	if !ok {
		contextTitle = fmt.Sprintf("Ошибка: %s", errContext)
	}

	var userInfo string
	if chatID != nil {
		who := fmt.Sprintf("ID: %d", *chatID)
		if username != nil && *username != "" {
			who = "@" + *username
		}
		userInfo = fmt.Sprintf("У пользователя %s произошла ошибка", who)
		if userMessage != "" {
			userInfo += fmt.Sprintf(" при сообщении %q", userMessage)
		}
	} else {
		userInfo = "Системная ошибка бота"
	}

	lines := []string{
		userInfo,
		"",
		contextTitle,
		"",
		"Детали ошибки:",
		err.Error(),
		"",
	}
	if chatID != nil {
		lines = append(lines, fmt.Sprintf("Chat ID: %d", *chatID), "")
	}
	lines = append(lines, fmt.Sprintf("Время: %s", config.NowMoscowStr()))

	text := strings.Join(lines, "\n")

	for _, adminID := range config.AdminUserIDs {
		if _, sendErr := SafeSendMessage(ctx, b, &bot.SendMessageParams{
			ChatID:              adminID,
			Text:                text,
			DisableNotification: true,
		}); sendErr != nil {
			slog.Warn("failed to send error report to admin", "admin_id", adminID, "error", sendErr)
		}
	}
}
