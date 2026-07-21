package ratelimit

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/go-telegram/bot"

	"github.com/grepfruitx/instgobot/internal/telegramapi"
)

func minutesWord(minutes int) string {
	switch {
	case minutes == 1:
		return "минуту"
	case minutes < 5:
		return "минуты"
	default:
		return "минут"
	}
}

func SendGeneralLimitMessage(ctx context.Context, b *bot.Bot, chatID int64, resetTime time.Time) {
	minutesLeft := max(int(math.Ceil(time.Until(resetTime).Minutes())), 1)

	text := fmt.Sprintf(
		"⚠️ Превышен лимит запросов\n\nВы можете отправлять максимум %d запросов в минуту.\nПопробуйте снова через %d %s.\n\nЭто ограничение помогает поддерживать стабильную работу бота для всех пользователей. 🤖",
		GeneralLimit, minutesLeft, minutesWord(minutesLeft),
	)
	_, _ = telegramapi.SafeSendMessage(ctx, b, &bot.SendMessageParams{ChatID: chatID, Text: text})
}
