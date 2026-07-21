package userbot

import (
	"context"
	"errors"
	"fmt"
	"os"

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

	storyMedia, ok := storyMediaOf(result.Stories[0])
	var loc tg.InputFileLocationClass
	var kind mediaKind
	var size int64
	if ok {
		loc, kind, size, ok = extractDownloadable(storyMedia)
	}
	if !ok {
		loading.delete(ctx)
		h.sendText(ctx, chatID, fmt.Sprintf("Сторис #%d не содержит медиа.\n%s", storyID, config.BotTag))
		h.st.RecordDownloadLogged(chatID, sourceURL, "telegram", "story", false, &username, nil)
		return false
	}

	if err := checkSize(size); err != nil {
		loading.delete(ctx)
		h.sendText(ctx, chatID, fmt.Sprintf("Сторис слишком большой для загрузки (максимум 50MB).\n%s", config.BotTag))
		h.st.RecordDownloadLogged(chatID, sourceURL, "telegram", "story", false, &username, nil)
		return false
	}

	stream := downloadMediaStream(ctx, h.client.API(), loc)
	_, sendErr := h.sendDownloadedMedia(ctx, chatID, kind, stream, config.BotTag)
	stream.Close()
	if sendErr != nil {
		return h.genericStoriesFailure(ctx, chatID, loading, sourceURL, &username, sendErr)
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
	var photoBatch, videoBatch []string

	flush := func(batch *[]string, kind mediaKind, context string) {
		if len(*batch) == 0 {
			return
		}
		files := make([]*os.File, 0, len(*batch))
		media := make([]models.InputMedia, 0, len(*batch))
		for i, path := range *batch {
			f, err := os.Open(path)
			if err != nil {
				continue
			}
			files = append(files, f)
			caption := ""
			if i == 0 {
				caption = config.BotTag
			}
			media = append(media, inputMediaFor(kind, f, fmt.Sprintf("attach://story%d", i), caption))
		}
		if len(media) > 0 {
			if _, err := telegramapi.SafeSendMediaGroup(ctx, h.b, &bot.SendMediaGroupParams{ChatID: chatID, Media: media, DisableNotification: true}); err != nil {
				telegramapi.SendErrorToAdmin(ctx, h.b, err, context, "", &chatID, &username)
			}
		}
		for _, f := range files {
			f.Close()
			os.Remove(f.Name())
		}
		*batch = nil
	}

	for _, item := range result.Stories.Stories {
		storyMedia, ok := storyMediaOf(item)
		if !ok {
			continue
		}
		loc, kind, size, extractOK := extractDownloadable(storyMedia)
		if !extractOK {
			continue
		}
		if err := checkSize(size); err != nil {
			h.sendText(ctx, chatID, fmt.Sprintf("Сторис слишком большой для загрузки (максимум 50MB).\n%s", config.BotTag))
			continue
		}

		path, err := downloadMediaToFile(ctx, h.client.API(), loc)
		if err != nil {
			var tooLarge *telegramapi.FileTooLargeError
			if errors.As(err, &tooLarge) {
				h.sendText(ctx, chatID, fmt.Sprintf("Сторис слишком большой для загрузки (максимум 50MB).\n%s", config.BotTag))
			} else {
				telegramapi.SendErrorToAdmin(ctx, h.b, err, "telegram stories download", "", &chatID, &username)
			}
			continue
		}

		successCount++
		if kind == mediaPhoto {
			photoBatch = append(photoBatch, path)
			if len(photoBatch) == 10 {
				flush(&photoBatch, mediaPhoto, "sendMediaGroup photos")
			}
		} else {
			videoBatch = append(videoBatch, path)
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
