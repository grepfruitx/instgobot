package admin

import (
	"context"
	"fmt"
	"strings"

	"github.com/grepfruitx/instgobot/internal/config"
)

func (h *Handler) handleUsers(ctx context.Context, chatID int64, args []string) {
	users, err := h.st.GetUsers(parseLimit(args, 20))
	if err != nil {
		h.send(ctx, chatID, fmt.Sprintf("❌ Ошибка при получении пользователей: %v", err))
		return
	}
	if len(users) == 0 {
		h.send(ctx, chatID, "📭 Пользователей пока нет")
		return
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "👥 Статистика пользователей (показано %d):\n\n", len(users))
	for i, u := range users {
		fmt.Fprintf(&sb, "%d. %s\n", i+1, displayName(u.Username, u.FirstName))
		fmt.Fprintf(&sb, "   📊 Скачивания: %d\n", u.DownloadCount)
		fmt.Fprintf(&sb, "   ❌ Ошибки: %d\n", u.ErrorCount)
		fmt.Fprintf(&sb, "   🕐 Последняя активность: %s\n", config.FormatMoscow(u.LastActivity))
		fmt.Fprintf(&sb, "   🆔 ID: %d\n\n", u.ChatID)
	}
	h.sendChunked(ctx, chatID, sb.String())
}

func (h *Handler) handleStats(ctx context.Context, chatID int64) {
	stats, err := h.st.GetStats()
	if err != nil {
		h.send(ctx, chatID, fmt.Sprintf("❌ Ошибка при получении статистики: %v", err))
		return
	}
	h.send(ctx, chatID, fmt.Sprintf(
		"📊 Общая статистика бота:\n\n👥 Всего пользователей: %d\n📱 Активных за 24ч: %d\n✅ Успешных скачиваний: %d\n❌ Всего ошибок: %d\n\n⏰ Обновлено: %s",
		stats.TotalUsers, stats.ActiveUsers24h, stats.TotalDownloads, stats.TotalErrors, config.NowMoscowStr(),
	))
}

func (h *Handler) handleTopUsers(ctx context.Context, chatID int64, args []string) {
	users, err := h.st.GetTopUsers(parseLimit(args, 10))
	if err != nil {
		h.send(ctx, chatID, fmt.Sprintf("❌ Ошибка при получении топа: %v", err))
		return
	}
	if len(users) == 0 {
		h.send(ctx, chatID, "📭 Активных пользователей пока нет")
		return
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "🏆 ТОП-%d пользователей по скачиваниям:\n\n", len(users))
	for i, u := range users {
		medal := fmt.Sprintf("%d.", i+1)
		switch i {
		case 0:
			medal = "🥇"
		case 1:
			medal = "🥈"
		case 2:
			medal = "🥉"
		}
		fmt.Fprintf(&sb, "%s %s\n", medal, displayName(u.Username, u.FirstName))
		fmt.Fprintf(&sb, "   📊 %d скачиваний\n", u.DownloadCount)
		fmt.Fprintf(&sb, "   🆔 ID: %d\n\n", u.ChatID)
	}
	h.sendChunked(ctx, chatID, sb.String())
}

func (h *Handler) handleErrors(ctx context.Context, chatID int64, args []string) {
	errs, err := h.st.GetRecentErrors(parseLimit(args, 5))
	if err != nil {
		h.send(ctx, chatID, fmt.Sprintf("❌ Ошибка при получении ошибок: %v", err))
		return
	}
	if len(errs) == 0 {
		h.send(ctx, chatID, "✅ Недавних ошибок нет")
		return
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "🚨 Последние %d ошибок:\n\n", len(errs))
	for i, e := range errs {
		originalMessage := "Не указано"
		if e.OriginalMessage != nil && *e.OriginalMessage != "" {
			originalMessage = *e.OriginalMessage
		}
		errMsg := e.ErrorMessage
		if len(errMsg) > 100 {
			errMsg = errMsg[:100] + "..."
		}
		fmt.Fprintf(&sb, "%d. %s (ID: %d)\n", i+1, displayName(e.Username, e.FirstName), e.ChatID)
		fmt.Fprintf(&sb, "   🏷️ Контекст: %s\n", e.ErrorContext)
		fmt.Fprintf(&sb, "   💬 Сообщение: %s\n", originalMessage)
		fmt.Fprintf(&sb, "   ⚠️ Ошибка: %s\n", errMsg)
		fmt.Fprintf(&sb, "   🕐 Время: %s\n\n", config.FormatMoscow(e.Timestamp))
	}
	h.sendChunked(ctx, chatID, sb.String())
}

func (h *Handler) handleAnnounceCount(ctx context.Context, chatID int64) {
	stats, err := h.st.GetNewsletterStats()
	if err != nil {
		h.send(ctx, chatID, "Произошла ошибка при получении статистики подписок.")
		return
	}
	percent := 0
	if stats.Total > 0 {
		percent = int(float64(stats.Subscribed) / float64(stats.Total) * 100)
	}
	h.send(ctx, chatID, fmt.Sprintf(
		"📊 Статистика подписок на рассылку:\n\n👥 Всего пользователей в базе: %d\n🔔 Подписаны на рассылку: %d\n🔕 Отписались от рассылки: %d\n\n📈 Процент подписчиков: %d%%\n\n⏰ Проверено: %s",
		stats.Total, stats.Subscribed, stats.Unsubscribed, percent, config.NowMoscowStr(),
	))
}
