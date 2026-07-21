package userbot

import (
	"bytes"
	"context"
	"log/slog"
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

func recordDownload(st *store.Store, chatID int64, sourceURL, mediaType string, success bool, username *string) {
	if err := st.RecordDownload(chatID, sourceURL, "telegram", mediaType, success, username, nil); err != nil {
		slog.Error("record download failed", "error", err, "chat_id", chatID)
	}
}

// loadingHandle tracks the "Загружаю..." placeholder message so callers can
// delete it once without threading a *models.Message through every branch.
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
	_, _ = telegramapi.SafeSendMessage(ctx, h.b, &bot.SendMessageParams{ChatID: chatID, Text: text})
}

func (h *Handler) sendDownloadedMedia(ctx context.Context, chatID int64, kind mediaKind, data []byte, caption string) (*models.Message, error) {
	if kind == mediaPhoto {
		return telegramapi.SafeSendPhoto(ctx, h.b, &bot.SendPhotoParams{
			ChatID: chatID, Photo: &models.InputFileUpload{Filename: "photo.jpg", Data: bytes.NewReader(data)},
			Caption: caption, DisableNotification: true,
		})
	}
	return telegramapi.SafeSendVideo(ctx, h.b, &bot.SendVideoParams{
		ChatID: chatID, Video: &models.InputFileUpload{Filename: "video.mp4", Data: bytes.NewReader(data)},
		Caption: caption, DisableNotification: true, SupportsStreaming: true,
	})
}

func inputMediaFor(kind mediaKind, data []byte, attachName, caption string) models.InputMedia {
	if kind == mediaPhoto {
		return &models.InputMediaPhoto{Media: attachName, MediaAttachment: bytes.NewReader(data), Caption: caption}
	}
	return &models.InputMediaVideo{Media: attachName, MediaAttachment: bytes.NewReader(data), Caption: caption, SupportsStreaming: true}
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

// storyMediaOf extracts the *tg.StoryItem's downloadable media, returning
// ok=false for deleted/skipped story variants (no media to extract).
func storyMediaOf(item tg.StoryItemClass) (tg.MessageMediaClass, bool) {
	full, ok := item.(*tg.StoryItem)
	if !ok {
		return nil, false
	}
	return full.Media, true
}
