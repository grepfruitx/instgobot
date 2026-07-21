package userbot

import (
	"context"
	"io"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/gotd/td/tg"

	"github.com/grepfruitx/instgobot/internal/store"
	"github.com/grepfruitx/instgobot/internal/telegramapi"
)

type Handler struct {
	client *Client
	b      *bot.Bot
	st     *store.Store
}

func New(client *Client, b *bot.Bot, st *store.Store) *Handler {
	return &Handler{client: client, b: b, st: st}
}

type loadingHandle struct {
	h      *Handler
	chatID int64
	msg    *models.Message
}

func (h *Handler) startLoading(ctx context.Context, chatID int64, text string) *loadingHandle {
	msg, _ := telegramapi.SafeSendMessage(ctx, h.b, &bot.SendMessageParams{ChatID: chatID, Text: text, DisableNotification: true})
	return &loadingHandle{h: h, chatID: chatID, msg: msg}
}

func (l *loadingHandle) delete(ctx context.Context) {
	if l.msg != nil {
		telegramapi.SafeDeleteMessage(ctx, l.h.b, l.chatID, l.msg.ID)
	}
}

func (h *Handler) sendText(ctx context.Context, chatID int64, text string) {
	_, _ = telegramapi.SendText(ctx, h.b, chatID, text)
}

func (h *Handler) sendDownloadedMedia(ctx context.Context, chatID int64, kind mediaKind, body io.Reader, caption string) (*models.Message, error) {
	if kind == mediaPhoto {
		return telegramapi.SafeSendPhoto(ctx, h.b, &bot.SendPhotoParams{
			ChatID: chatID, Photo: &models.InputFileUpload{Filename: "photo.jpg", Data: body},
			Caption: caption, DisableNotification: true,
		})
	}
	return telegramapi.SafeSendVideo(ctx, h.b, &bot.SendVideoParams{
		ChatID: chatID, Video: &models.InputFileUpload{Filename: "video.mp4", Data: body},
		Caption: caption, DisableNotification: true, SupportsStreaming: true,
	})
}

func inputMediaFor(kind mediaKind, body io.Reader, attachName, caption string) models.InputMedia {
	if kind == mediaPhoto {
		return &models.InputMediaPhoto{Media: attachName, MediaAttachment: body, Caption: caption}
	}
	return &models.InputMediaVideo{Media: attachName, MediaAttachment: body, Caption: caption, SupportsStreaming: true}
}

func isNoAccessError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, substr := range []string{
		"CHANNEL_PRIVATE", "USER_NOT_PARTICIPANT", "CHAT_ADMIN_REQUIRED",
		"CHANNEL_INVALID", "Cannot find any entity", "Could not find the input entity",
	} {
		if strings.Contains(msg, substr) {
			return true
		}
	}
	return false
}

func storyMediaOf(item tg.StoryItemClass) (tg.MessageMediaClass, bool) {
	// ok=false also covers deleted/skipped story variants, which carry no media
	full, ok := item.(*tg.StoryItem)
	if !ok {
		return nil, false
	}
	return full.Media, true
}
