package admin

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/grepfruitx/instgobot/internal/config"
)

func (h *Handler) handleBan(ctx context.Context, chatID int64, args []string) {
	if len(args) == 0 {
		h.send(ctx, chatID, "Использование: /ban <chat_id>")
		return
	}
	targetID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		h.send(ctx, chatID, "Некорректный chat_id")
		return
	}
	if err := h.st.BanUser(targetID); err != nil {
		h.send(ctx, chatID, fmt.Sprintf("❌ Ошибка при бане: %v", err))
		return
	}
	h.send(ctx, chatID, fmt.Sprintf("🚫 Пользователь %d забанен", targetID))
}

func (h *Handler) handleUnban(ctx context.Context, chatID int64, args []string) {
	if len(args) == 0 {
		h.send(ctx, chatID, "Использование: /unban <chat_id>")
		return
	}
	targetID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		h.send(ctx, chatID, "Некорректный chat_id")
		return
	}
	if err := h.st.UnbanUser(targetID); err != nil {
		h.send(ctx, chatID, fmt.Sprintf("❌ Ошибка при разбане: %v", err))
		return
	}
	h.send(ctx, chatID, fmt.Sprintf("✅ Пользователь %d разбанен", targetID))
}

func (h *Handler) handleBannedList(ctx context.Context, chatID int64) {
	users, err := h.st.GetBannedUsers()
	if err != nil {
		h.send(ctx, chatID, fmt.Sprintf("❌ Ошибка при получении списка забаненных: %v", err))
		return
	}
	if len(users) == 0 {
		h.send(ctx, chatID, "✅ Забаненных нет")
		return
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "🚫 Забанено (%d):\n\n", len(users))
	for _, u := range users {
		fmt.Fprintf(&sb, "%d — %s\n", u.ChatID, config.FormatMoscow(u.BannedAt))
	}
	h.sendChunked(ctx, chatID, sb.String())
}
