package admin

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/grepfruitx/instgobot/internal/platform"
)

func (h *Handler) handlePlatforms(ctx context.Context, chatID int64) {
	platforms, err := h.st.GetPlatformStats()
	if err != nil {
		h.send(ctx, chatID, fmt.Sprintf("Ошибка при получении статистики платформ: %v", err))
		return
	}
	if len(platforms) == 0 {
		h.send(ctx, chatID, "Статистика платформ пока пуста")
		return
	}

	var sb strings.Builder
	sb.WriteString("Статистика по платформам:\n\n")
	for _, p := range platforms {
		fmt.Fprintf(&sb, "%s\n", strings.ToUpper(p.Platform))
		fmt.Fprintf(&sb, "   Всего запросов: %d\n", p.TotalRequests)
		fmt.Fprintf(&sb, "   Успешных: %d\n", p.SuccessfulDownloads)
		fmt.Fprintf(&sb, "   Процент успеха: %s%%\n\n", strconv.FormatFloat(p.SuccessRate, 'f', -1, 64))
	}
	h.sendChunked(ctx, chatID, sb.String())
}

func (h *Handler) handlePlatformToggle(ctx context.Context, chatID int64, args []string, disabled bool) {
	cmdName := "poff"
	if !disabled {
		cmdName = "pon"
	}

	var plat string
	if len(args) > 0 {
		plat = strings.ToLower(args[0])
	}
	if plat == "" || !slices.Contains(platform.SupportedPlatforms, plat) {
		h.send(ctx, chatID, fmt.Sprintf(
			"Укажите платформу: %s\n\nПример: /%s instagram",
			strings.Join(platform.SupportedPlatforms, ", "), cmdName,
		))
		return
	}

	if err := h.st.SetPlatformDisabled(plat, disabled); err != nil {
		h.send(ctx, chatID, fmt.Sprintf("Ошибка: %v", err))
		return
	}

	disabledNow, err := h.st.GetDisabledPlatforms()
	statusLine := "Все платформы включены"
	if err == nil && len(disabledNow) > 0 {
		statusLine = fmt.Sprintf("Выключены: %s", strings.Join(disabledNow, ", "))
	}

	action := "выключена"
	if !disabled {
		action = "включена"
	}
	h.send(ctx, chatID, fmt.Sprintf("Платформа %s %s.\n\n%s", plat, action, statusLine))

	if !disabled {
		h.notifyWaitlist(ctx, chatID, plat)
	}
}

func (h *Handler) notifyWaitlist(ctx context.Context, adminChatID int64, plat string) {
	waiting, err := h.st.GetWaitlist(plat)
	if err != nil || len(waiting) == 0 {
		return
	}

	text := fmt.Sprintf("%s снова работает! Отправьте ссылку ещё раз.", strings.ToUpper(plat))
	success, failure, _ := h.broadcast(ctx, waiting, text)
	_ = h.st.ClearWaitlist(plat)

	h.send(ctx, adminChatID, fmt.Sprintf("Уведомил список ожидания (%s): доставлено %d, не удалось %d", plat, success, failure))
}

func (h *Handler) handlePlatformStatus(ctx context.Context, chatID int64) {
	disabledNow, err := h.st.GetDisabledPlatforms()
	if err != nil {
		h.send(ctx, chatID, fmt.Sprintf("Ошибка: %v", err))
		return
	}
	disabledSet := make(map[string]bool, len(disabledNow))
	for _, p := range disabledNow {
		disabledSet[p] = true
	}

	lines := []string{"Статус платформ:", ""}
	for _, p := range platform.SupportedPlatforms {
		status := "включена"
		if disabledSet[p] {
			status = "выключена"
		}
		lines = append(lines, fmt.Sprintf("%s — %s", p, status))
	}
	h.send(ctx, chatID, strings.Join(lines, "\n"))
}
