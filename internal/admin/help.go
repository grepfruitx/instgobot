package admin

import (
	"context"
	"fmt"
	"strings"

	"github.com/grepfruitx/instgobot/internal/platform"
)

func (h *Handler) handleHelp(ctx context.Context, chatID int64) {
	h.send(ctx, chatID, strings.Join([]string{
		"🔧 Админские команды:",
		"",
		"👤 /users [количество] - список пользователей",
		"📊 /stats - общая статистика",
		"🏆 /top [количество] - топ пользователей",
		"🚨 /errors [количество] - последние ошибки",
		"📱 /platforms - статистика по платформам",
		"📢 /announce [сообщение] - отправить объявление подписанным пользователям",
		"📈 /announceCount - статистика подписок на рассылку",
		"🔴 /poff <платформа> - выключить платформу",
		"🟢 /pon <платформа> - включить платформу обратно",
		"📡 /pstatus - статус всех платформ",
		"🚨 /errortop [количество] - топ повторяющихся ошибок",
		"⚡ /cachestats - процент попаданий в кэш по платформам",
		"📈 /retention - новые/неактивные юзеры",
		"🕐 /activity - активность по часам и дням недели",
		"🗑️ /clearcache <url> - очистить кэш file_id для ссылки",
		"⛔ /ratelimits - сколько раз срабатывал рейт-лимит",
		"🚫 /ban <chat_id> - забанить пользователя",
		"✅ /unban <chat_id> - разбанить пользователя",
		"📋 /banned - список забаненных",
		"🩺 /health - redis/диск/память/userbot/платформы",
		"❓ /ah - эта справка",
		"",
		"Примеры:",
		"• /users 10 - показать 10 пользователей",
		"• /top 5 - топ 5 пользователей",
		"• /errors 3 - последние 3 ошибки",
		"• /poff instagram - выключить Instagram",
		"• /pon instagram - включить Instagram обратно",
		"",
		fmt.Sprintf("Доступные платформы: %s", strings.Join(platform.SupportedPlatforms, ", ")),
		"",
		"ℹ️ Рассылка отправляется только пользователям с включенной подпиской (/newsletter)",
	}, "\n"))
}
