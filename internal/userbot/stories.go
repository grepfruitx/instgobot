package userbot

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/gotd/td/tg"

	"github.com/grepfruitx/instgobot/internal/config"
	"github.com/grepfruitx/instgobot/internal/telegramapi"
)

func (h *Handler) genericStoriesFailure(ctx context.Context, chatID int64, loading *loadingHandle, sourceURL string, username *string, err error) bool {
	loading.delete(ctx)
	h.sendText(ctx, chatID, fmt.Sprintf("Ошибка при загрузке сторис. Попробуйте позже.\n%s", config.BotTag))
	if err != nil {
		telegramapi.SendErrorToAdmin(ctx, h.b, err, "telegram stories download", "", &chatID, username)
	}
	h.st.RecordDownloadLogged(chatID, sourceURL, "telegram", "story", false, username, nil)
	return false
}

func (h *Handler) DownloadStoryByID(ctx context.Context, chatID int64, username string, storyID int) bool {
	sourceURL := fmt.Sprintf("t.me/%s/s/%d", username, storyID)
	loading := h.startLoading(ctx, chatID, "Загружаю сторис...")

	peer, err := h.client.Peers().Resolve(ctx, username)
	if err != nil {
		return h.genericStoriesFailure(ctx, chatID, loading, sourceURL, &username, err)
	}

	result, err := h.client.API().StoriesGetStoriesByID(ctx, &tg.StoriesGetStoriesByIDRequest{
		Peer: peer.InputPeer(), ID: []int{storyID},
	})
	if err != nil {
		return h.genericStoriesFailure(ctx, chatID, loading, sourceURL, &username, err)
	}

	if len(result.Stories) == 0 {
		loading.delete(ctx)
		h.sendText(ctx, chatID, fmt.Sprintf("Сторис #%d не найдена у @%s.\n%s", storyID, username, config.BotTag))
		h.st.RecordDownloadLogged(chatID, sourceURL, "telegram", "story", false, &username, nil)
		return false
	}

	media, ok := storyMediaOf(result.Stories[0])
	var loc tg.InputFileLocationClass
	var kind mediaKind
	if ok {
		loc, kind, ok = extractDownloadable(media)
	}
	if !ok {
		loading.delete(ctx)
		h.sendText(ctx, chatID, fmt.Sprintf("Сторис #%d не содержит медиа.\n%s", storyID, config.BotTag))
		h.st.RecordDownloadLogged(chatID, sourceURL, "telegram", "story", false, &username, nil)
		return false
	}

	data, err := downloadMediaBytes(ctx, h.client.API(), loc)
	if err != nil || len(data) == 0 {
		loading.delete(ctx)
		h.sendText(ctx, chatID, fmt.Sprintf("Не удалось скачать сторис #%d.\n%s", storyID, config.BotTag))
		h.st.RecordDownloadLogged(chatID, sourceURL, "telegram", "story", false, &username, nil)
		return false
	}

	if _, err := h.sendDownloadedMedia(ctx, chatID, kind, data, config.BotTag); err != nil {
		return h.genericStoriesFailure(ctx, chatID, loading, sourceURL, &username, err)
	}

	loading.delete(ctx)
	h.st.RecordDownloadLogged(chatID, sourceURL, "telegram", "story", true, &username, nil)
	return true
}

func (h *Handler) DownloadStories(ctx context.Context, chatID int64, username string) bool {
	loading := h.startLoading(ctx, chatID, "Загружаю сторис...")

	peer, err := h.client.Peers().Resolve(ctx, username)
	if err != nil {
		return h.genericStoriesFailure(ctx, chatID, loading, username, &username, err)
	}

	result, err := h.client.API().StoriesGetPeerStories(ctx, peer.InputPeer())
	if err != nil {
		return h.genericStoriesFailure(ctx, chatID, loading, username, &username, err)
	}

	if len(result.Stories.Stories) == 0 {
		loading.delete(ctx)
		h.sendText(ctx, chatID, fmt.Sprintf(
			"Не удалось найти публичные сторис у @%s. Возможно, пользователь скрыл свои сторис или у него нет публичных сторис.\n%s",
			username, config.BotTag,
		))
		h.st.RecordDownloadLogged(chatID, username, "telegram", "story", false, &username, nil)
		return false
	}

	successCount := 0
	var photoBatch, videoBatch [][]byte

	flush := func(batch *[][]byte, kind mediaKind, context string) {
		if len(*batch) == 0 {
			return
		}
		media := make([]models.InputMedia, len(*batch))
		for i, data := range *batch {
			caption := ""
			if i == 0 {
				caption = config.BotTag
			}
			media[i] = inputMediaFor(kind, data, fmt.Sprintf("attach://story%d", i), caption)
		}
		if _, err := telegramapi.SafeSendMediaGroup(ctx, h.b, &bot.SendMediaGroupParams{ChatID: chatID, Media: media, DisableNotification: true}); err != nil {
			telegramapi.SendErrorToAdmin(ctx, h.b, err, context, "", &chatID, &username)
		}
		*batch = nil
	}

	for _, item := range result.Stories.Stories {
		media, ok := storyMediaOf(item)
		if !ok {
			continue
		}
		loc, kind, extractOK := extractDownloadable(media)
		if !extractOK {
			continue
		}

		data, err := downloadMediaBytes(ctx, h.client.API(), loc)
		if err != nil || len(data) == 0 {
			var tooLarge *telegramapi.FileTooLargeError
			if errors.As(err, &tooLarge) {
				h.sendText(ctx, chatID, fmt.Sprintf("Сторис слишком большой для загрузки (максимум 50MB).\n%s", config.BotTag))
			} else if err != nil {
				telegramapi.SendErrorToAdmin(ctx, h.b, err, "telegram stories download", "", &chatID, &username)
			}
			continue
		}

		successCount++
		if kind == mediaPhoto {
			photoBatch = append(photoBatch, data)
			if len(photoBatch) == 10 {
				flush(&photoBatch, mediaPhoto, "sendMediaGroup photos")
			}
		} else {
			videoBatch = append(videoBatch, data)
			if len(videoBatch) == 10 {
				flush(&videoBatch, mediaVideo, "sendMediaGroup videos")
			}
		}
	}

	flush(&photoBatch, mediaPhoto, "sendMediaGroup photos")
	flush(&videoBatch, mediaVideo, "sendMediaGroup videos")

	if successCount == 0 {
		loading.delete(ctx)
		h.sendText(ctx, chatID, fmt.Sprintf("Сторис найдены, но не удалось загрузить медиа. Возможно, они недоступны.\n%s", config.BotTag))
		h.st.RecordDownloadLogged(chatID, username, "telegram", "story", false, &username, nil)
		return false
	}

	loading.delete(ctx)
	h.st.RecordDownloadLogged(chatID, username, "telegram", "story", true, &username, nil)
	return true
}
