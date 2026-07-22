package admin

import (
	"context"
	"fmt"
	"strings"
)

func (h *Handler) handleClearCache(ctx context.Context, chatID int64, args []string) {
	if len(args) == 0 {
		h.send(ctx, chatID, "Использование: /clearcache <url>")
		return
	}
	rows, err := h.st.ClearCache(args[0])
	if err != nil {
		h.send(ctx, chatID, fmt.Sprintf("❌ Ошибка при очистке кэша: %v", err))
		return
	}
	h.send(ctx, chatID, fmt.Sprintf("🗑️ Удалено записей из кэша: %d", rows))
}

func (h *Handler) handleRateLimitHits(ctx context.Context, chatID int64) {
	hits, err := h.st.GetRateLimitHits()
	if err != nil {
		h.send(ctx, chatID, fmt.Sprintf("❌ Ошибка при получении статистики рейт-лимитов: %v", err))
		return
	}
	if len(hits) == 0 {
		h.send(ctx, chatID, "📭 Срабатываний рейт-лимита пока не было")
		return
	}

	var sb strings.Builder
	sb.WriteString("⛔ Срабатывания рейт-лимита:\n\n")
	for _, hit := range hits {
		fmt.Fprintf(&sb, "%s — %d\n", hit.Kind, hit.Count)
	}
	h.send(ctx, chatID, sb.String())
}

func (h *Handler) handleErrorTop(ctx context.Context, chatID int64, args []string) {
	clusters, err := h.st.GetTopErrorMessages(parseLimit(args, 10))
	if err != nil {
		h.send(ctx, chatID, fmt.Sprintf("❌ Ошибка при получении статистики ошибок: %v", err))
		return
	}
	if len(clusters) == 0 {
		h.send(ctx, chatID, "✅ Ошибок пока нет")
		return
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "🚨 ТОП-%d повторяющихся ошибок:\n\n", len(clusters))
	for i, c := range clusters {
		msg := c.ErrorMessage
		if len(msg) > 150 {
			msg = msg[:150] + "..."
		}
		fmt.Fprintf(&sb, "%d. (%d раз)\n   %s\n\n", i+1, c.Count, msg)
	}
	h.sendChunked(ctx, chatID, sb.String())
}

func (h *Handler) handleCacheStats(ctx context.Context, chatID int64) {
	stats, err := h.st.GetCacheStats()
	if err != nil {
		h.send(ctx, chatID, fmt.Sprintf("❌ Ошибка при получении статистики кэша: %v", err))
		return
	}
	if len(stats) == 0 {
		h.send(ctx, chatID, "📭 Статистика кэша пока пуста")
		return
	}

	var sb strings.Builder
	sb.WriteString("⚡ Попадания в кэш по платформам:\n\n")
	for _, s := range stats {
		total := s.Hits + s.Misses
		percent := 0.0
		if total > 0 {
			percent = float64(s.Hits) / float64(total) * 100
		}
		fmt.Fprintf(&sb, "🌐 %s\n   ⚡ Из кэша: %d\n   📥 Свежих закачек: %d\n   📈 Процент: %.1f%%\n\n", strings.ToUpper(s.Platform), s.Hits, s.Misses, percent)
	}
	h.send(ctx, chatID, sb.String())
}

func (h *Handler) handleRetention(ctx context.Context, chatID int64) {
	r, err := h.st.GetRetentionStats()
	if err != nil {
		h.send(ctx, chatID, fmt.Sprintf("❌ Ошибка при получении retention-статистики: %v", err))
		return
	}
	h.send(ctx, chatID, fmt.Sprintf(
		"📈 Retention:\n\n🆕 Новых юзеров сегодня: %d\n🆕 Новых юзеров за неделю: %d\n\n😴 Неактивны >7 дней: %d\n😴 Неактивны >30 дней: %d",
		r.NewUsersToday, r.NewUsersThisWeek, r.InactiveOver7d, r.InactiveOver30d,
	))
}

var ruWeekdays = []string{"Воскресенье", "Понедельник", "Вторник", "Среда", "Четверг", "Пятница", "Суббота"}

func (h *Handler) handleActivity(ctx context.Context, chatID int64) {
	hours, err := h.st.GetActivityByHour()
	if err != nil {
		h.send(ctx, chatID, fmt.Sprintf("❌ Ошибка при получении активности по часам: %v", err))
		return
	}
	weekdays, err := h.st.GetActivityByWeekday()
	if err != nil {
		h.send(ctx, chatID, fmt.Sprintf("❌ Ошибка при получении активности по дням недели: %v", err))
		return
	}

	var sb strings.Builder
	sb.WriteString("🕐 Активность по часам (МСК):\n\n")
	for _, hr := range hours {
		fmt.Fprintf(&sb, "%02d:00 — %d\n", hr.Hour, hr.Count)
	}
	sb.WriteString("\n📅 Активность по дням недели:\n\n")
	for _, w := range weekdays {
		name := "?"
		if w.Weekday >= 0 && w.Weekday < len(ruWeekdays) {
			name = ruWeekdays[w.Weekday]
		}
		fmt.Fprintf(&sb, "%s — %d\n", name, w.Count)
	}
	h.sendChunked(ctx, chatID, sb.String())
}
