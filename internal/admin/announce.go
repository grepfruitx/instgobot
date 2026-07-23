package admin

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-telegram/bot"

	"github.com/grepfruitx/instgobot/internal/config"
	"github.com/grepfruitx/instgobot/internal/store"
	"github.com/grepfruitx/instgobot/internal/telegramapi"
)

var announceCommandRe = regexp.MustCompile(`^/announce\s*`)

const announceBatchSize = 10

func (h *Handler) handleAnnounce(ctx context.Context, chatID int64, message string) {
	text := strings.TrimSpace(announceCommandRe.ReplaceAllString(message, ""))
	if text == "" {
		h.send(ctx, chatID, "Пожалуйста, добавьте текст объявления после команды /announce\n\nПример: /announce Сегодня мы добавили новую функцию!")
		return
	}

	formatted := fmt.Sprintf("%s\n\nОтписаться от рассылки: /newsletter\n\n%s", text, config.BotTag)

	users, err := h.st.GetAllUsers()
	if err != nil {
		h.send(ctx, chatID, fmt.Sprintf("Ошибка при получении пользователей: %v", err))
		return
	}
	if len(users) == 0 {
		h.send(ctx, chatID, "В базе данных нет пользователей для отправки объявления.")
		return
	}

	h.send(ctx, chatID, fmt.Sprintf("Начинаю отправку объявления %d пользователям...\n\nТекст объявления:\n%s", len(users), formatted))

	successCount, failureCount, failedUsers := h.broadcast(ctx, users, formatted)

	newsletterStats, _ := h.st.GetNewsletterStats()

	var failureLine string
	if failureCount > 0 {
		shown := failedUsers
		suffix := ""
		if len(shown) > 10 {
			suffix = fmt.Sprintf(" и еще %d...", len(shown)-10)
			shown = shown[:10]
		}
		ids := make([]string, len(shown))
		for i, id := range shown {
			ids[i] = fmt.Sprintf("%d", id)
		}
		failureLine = fmt.Sprintf("Не удалось доставить пользователям: %s%s", strings.Join(ids, ", "), suffix)
	} else {
		failureLine = "Все объявления доставлены успешно!"
	}

	h.send(ctx, chatID, strings.Join([]string{
		"Объявление отправлено!",
		"",
		"Статистика рассылки:",
		fmt.Sprintf("Успешно доставлено: %d", successCount),
		fmt.Sprintf("Не удалось доставить: %d", failureCount),
		fmt.Sprintf("Отправлено подписанным: %d", len(users)),
		"",
		"Общая статистика пользователей:",
		fmt.Sprintf("Всего пользователей: %d", newsletterStats.Total),
		fmt.Sprintf("Подписаны на рассылку: %d", newsletterStats.Subscribed),
		fmt.Sprintf("Отписаны от рассылки: %d", newsletterStats.Unsubscribed),
		"",
		failureLine,
	}, "\n"))
}

func (h *Handler) broadcast(ctx context.Context, users []store.NewsletterUser, text string) (success, failure int, failedUsers []int64) {
	for i := 0; i < len(users); i += announceBatchSize {
		end := min(i+announceBatchSize, len(users))
		batch := users[i:end]

		var wg sync.WaitGroup
		var mu sync.Mutex
		for _, u := range batch {
			wg.Add(1)
			go func(u store.NewsletterUser) {
				defer wg.Done()
				msg, err := telegramapi.SafeSendMessage(ctx, h.b, &bot.SendMessageParams{
					ChatID: u.ChatID, Text: text, DisableNotification: true,
				})

				mu.Lock()
				defer mu.Unlock()
				if err == nil && msg != nil {
					success++
				} else {
					failure++
					failedUsers = append(failedUsers, u.ChatID)
				}
			}(u)
		}
		wg.Wait()

		if end < len(users) {
			time.Sleep(500 * time.Millisecond)
		}
	}
	return success, failure, failedUsers
}
