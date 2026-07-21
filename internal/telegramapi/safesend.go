package telegramapi

import (
	"context"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func SafeSendMessage(ctx context.Context, b *bot.Bot, params *bot.SendMessageParams) (*models.Message, error) {
	msg, err := b.SendMessage(ctx, params)
	if err != nil {
		if IsBotBlockedError(err) {
			return nil, nil
		}
		return nil, err
	}
	return msg, nil
}

func SafeSendVideo(ctx context.Context, b *bot.Bot, params *bot.SendVideoParams) (*models.Message, error) {
	msg, err := b.SendVideo(ctx, params)
	if err != nil {
		if IsBotBlockedError(err) {
			return nil, nil
		}
		return nil, err
	}
	return msg, nil
}

func SafeSendPhoto(ctx context.Context, b *bot.Bot, params *bot.SendPhotoParams) (*models.Message, error) {
	msg, err := b.SendPhoto(ctx, params)
	if err != nil {
		if IsBotBlockedError(err) {
			return nil, nil
		}
		return nil, err
	}
	return msg, nil
}

func SafeSendMediaGroup(ctx context.Context, b *bot.Bot, params *bot.SendMediaGroupParams) ([]*models.Message, error) {
	msgs, err := b.SendMediaGroup(ctx, params)
	if err != nil {
		if IsBotBlockedError(err) {
			return nil, nil
		}
		return nil, err
	}
	return msgs, nil
}

func SafeDeleteMessage(ctx context.Context, b *bot.Bot, chatID any, messageID int) bool {
	ok, err := b.DeleteMessage(ctx, &bot.DeleteMessageParams{ChatID: chatID, MessageID: messageID})
	if err != nil {
		return false
	}
	return ok
}

func WithChatAction[T any](ctx context.Context, b *bot.Bot, chatID any, action models.ChatAction, fn func() (T, error)) (T, error) {
	send := func() {
		_, _ = b.SendChatAction(ctx, &bot.SendChatActionParams{ChatID: chatID, Action: action})
	}
	send()

	ticker := time.NewTicker(4 * time.Second)
	defer ticker.Stop()

	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				send()
			}
		}
	}()
	defer close(done)

	return fn()
}

func WithChatActionErr(ctx context.Context, b *bot.Bot, chatID any, action models.ChatAction, fn func() error) error {
	_, err := WithChatAction(ctx, b, chatID, action, func() (struct{}, error) {
		return struct{}{}, fn()
	})
	return err
}
