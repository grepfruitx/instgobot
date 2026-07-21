package messages

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/go-telegram/bot"

	"github.com/grepfruitx/instgobot/internal/config"
	"github.com/grepfruitx/instgobot/internal/store"
	"github.com/grepfruitx/instgobot/internal/telegramapi"
)

func ProcessNewsletterToggle(ctx context.Context, b *bot.Bot, st *store.Store, chatID int64) {
	subscribed, err := st.ToggleNewsletterSubscription(chatID)
	if err != nil {
		_, _ = telegramapi.SendText(ctx, b, chatID, "Произошла ошибка при изменении настроек рассылки. Попробуйте позже.")
		return
	}

	text := "❌ Подписка на рассылку отключена.\n\nВы больше не будете получать:\n• Объявления о новых функциях\n• Уведомления от бота\n\nВключить рассылку: /newsletter"
	if subscribed {
		text = "✅ Подписка на рассылку включена!\n\nТеперь вы будете получать:\n• Объявления о новых функциях\n• Важные уведомления от бота\n\nОтключить рассылку: /newsletter"
	}
	_, _ = telegramapi.SendText(ctx, b, chatID, text)
}

var featureCommandRe = regexp.MustCompile(`^/feat\s*`)

func ProcessFeatureRequest(ctx context.Context, b *bot.Bot, chatID int64, message string, adminUsername string, username, firstName *string) {
	featureText := strings.TrimSpace(featureCommandRe.ReplaceAllString(message, ""))
	if featureText == "" {
		_, _ = telegramapi.SendText(ctx, b, chatID, "💡 Расскажите нам о своей идее!\n\nИспользуйте команду так:\n/feat добавьте поддержку Pinterest\n\nМы рассмотрим ваше предложение и возможно добавим эту функцию в бот! ✨")
		return
	}

	userInfo := fmt.Sprintf("User ID: %d", chatID)
	if username != nil && *username != "" {
		userInfo = "@" + *username
	} else if firstName != nil && *firstName != "" {
		userInfo = *firstName
	}

	adminMessage := fmt.Sprintf(
		"💡 Новое предложение функции!\n\n👤 От пользователя: %s\n🆔 Chat ID: %d\n\n📝 Предложение:\n%s\n\n⏰ Время: %s",
		userInfo, chatID, featureText, time.Now().Format("02.01.2006, 15:04:05"),
	)

	successCount := 0
	for _, adminID := range config.AdminUserIDs {
		if _, err := telegramapi.SafeSendMessage(ctx, b, &bot.SendMessageParams{
			ChatID: adminID, Text: adminMessage, DisableNotification: true,
		}); err == nil {
			successCount++
		}
	}

	if successCount > 0 {
		_, _ = telegramapi.SendText(ctx, b, chatID, "✅ Спасибо за предложение!\n\nВаша идея отправлена разработчикам.\nМы рассмотрим её и, возможно, добавим в будущих обновлениях! 🚀")
		return
	}
	_, _ = telegramapi.SendText(ctx, b, chatID, fmt.Sprintf("❌ Произошла ошибка при отправке предложения.\nПопробуйте позже или обратитесь к администратору.\n%s", adminUsername))
}
