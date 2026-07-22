package media

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/grepfruitx/instgobot/internal/config"
	"github.com/grepfruitx/instgobot/internal/store"
	"github.com/grepfruitx/instgobot/internal/telegramapi"
)

type Kind string

const (
	KindPhoto Kind = "photo"
	KindVideo Kind = "video"
)

func (k Kind) chatAction() models.ChatAction {
	if k == KindVideo {
		return models.ChatActionUploadVideo
	}
	return models.ChatActionUploadPhoto
}

func (k Kind) ru() string {
	if k == KindVideo {
		return "видео"
	}
	return "фото"
}

func sendFromFileID(ctx context.Context, b *bot.Bot, chatID int64, kind Kind, fileID string, caption string) (*models.Message, error) {
	if kind == KindVideo {
		return telegramapi.SafeSendVideo(ctx, b, &bot.SendVideoParams{
			ChatID: chatID, Video: &models.InputFileString{Data: fileID},
			Caption: caption, DisableNotification: true, SupportsStreaming: true,
		})
	}
	return telegramapi.SafeSendPhoto(ctx, b, &bot.SendPhotoParams{
		ChatID: chatID, Photo: &models.InputFileString{Data: fileID},
		Caption: caption, DisableNotification: true,
	})
}

func uploadMedia(ctx context.Context, b *bot.Bot, chatID int64, kind Kind, body io.Reader, caption string) (*models.Message, error) {
	if kind == KindVideo {
		return telegramapi.SafeSendVideo(ctx, b, &bot.SendVideoParams{
			ChatID: chatID, Video: &models.InputFileUpload{Filename: "video.mp4", Data: body},
			Caption: caption, DisableNotification: true, SupportsStreaming: true,
		})
	}
	return telegramapi.SafeSendPhoto(ctx, b, &bot.SendPhotoParams{
		ChatID: chatID, Photo: &models.InputFileUpload{Filename: "photo.jpg", Data: body},
		Caption: caption, DisableNotification: true,
	})
}

func messageFileID(kind Kind, msg *models.Message) string {
	if msg == nil {
		return ""
	}
	if kind == KindVideo {
		if msg.Video != nil {
			return msg.Video.FileID
		}
		return ""
	}
	if len(msg.Photo) > 0 {
		return msg.Photo[len(msg.Photo)-1].FileID
	}
	return ""
}

func ProcessSingleVideo(ctx context.Context, b *bot.Bot, st *store.Store, chatID int64, url string, platform string, username *string, postURL *string) (bool, error) {
	return ProcessSingleMedia(ctx, b, st, chatID, url, KindVideo, platform, username, postURL)
}

func ProcessSinglePhoto(ctx context.Context, b *bot.Bot, st *store.Store, chatID int64, url string, platform string, username *string, postURL *string) (bool, error) {
	return ProcessSingleMedia(ctx, b, st, chatID, url, KindPhoto, platform, username, postURL)
}

func ProcessSingleMedia(ctx context.Context, b *bot.Bot, st *store.Store, chatID int64, url string, kind Kind, platform string, username *string, postURL *string) (bool, error) {
	if url == "" {
		sent, _ := telegramapi.SafeSendMessage(ctx, b, &bot.SendMessageParams{
			ChatID: chatID, Text: fmt.Sprintf("Не удалось получить URL %s.", kind.ru()),
		})
		if sent != nil {
			telegramapi.SendErrorToAdmin(ctx, b, fmt.Errorf("no %s url", kind), fmt.Sprintf("single %s", kind), "", &chatID, username)
		}
		return false, nil
	}

	if postURL != nil {
		if fileID, ok, err := st.GetCachedFileID(*postURL, string(kind), 0); err == nil && ok {
			_, sendErr := sendFromFileID(ctx, b, chatID, kind, fileID, config.BotTag)
			if sendErr == nil {
				_ = st.RecordCacheEvent(platform, true)
				return true, nil
			}
			if telegramapi.IsBotBlockedError(sendErr) {
				return false, nil
			}
			_ = st.RecordCacheEvent(platform, false)
			// stale file_id — fall through to re-download
		} else {
			_ = st.RecordCacheEvent(platform, false)
		}
	}

	for attempt := 0; attempt < 2; attempt++ {
		result, retry := attemptSingleDownloadAndSend(ctx, b, st, chatID, url, kind, username, postURL, attempt)
		if !retry {
			return result, nil
		}
		time.Sleep(1500 * time.Millisecond)
	}
	return false, nil
}

// retry=true means sleep and try again; result is only meaningful when retry=false.
func attemptSingleDownloadAndSend(ctx context.Context, b *bot.Bot, st *store.Store, chatID int64, url string, kind Kind, username *string, postURL *string, attempt int) (result bool, retry bool) {
	resp, err := FetchMediaResponse(ctx, url, false)
	if err != nil {
		return classifyMediaError(ctx, b, chatID, kind, username, err, attempt)
	}
	defer resp.Body.Close()

	sendErr := telegramapi.WithChatActionErr(ctx, b, chatID, kind.chatAction(), func() error {
		msg, err := uploadMedia(ctx, b, chatID, kind, resp.Body, config.BotTag)
		if err != nil {
			return err
		}
		if postURL != nil {
			if fileID := messageFileID(kind, msg); fileID != "" {
				st.SetCachedFileID(*postURL, string(kind), 0, fileID)
			}
		}
		return nil
	})

	if sendErr == nil {
		return true, false
	}
	return classifyMediaError(ctx, b, chatID, kind, username, sendErr, attempt)
}

func classifyMediaError(ctx context.Context, b *bot.Bot, chatID int64, kind Kind, username *string, err error, attempt int) (result bool, retry bool) {
	var tooLarge *telegramapi.FileTooLargeError
	if errors.As(err, &tooLarge) {
		_, _ = telegramapi.SafeSendMessage(ctx, b, &bot.SendMessageParams{
			ChatID: chatID, Text: "Слишком большой файл для загрузки. Максимальный размер: 50MB.",
		})
		return true, false
	}
	var fetchErr *telegramapi.MediaFetchError
	if errors.As(err, &fetchErr) {
		_, _ = telegramapi.SafeSendMessage(ctx, b, &bot.SendMessageParams{
			ChatID: chatID, Text: fmt.Sprintf("Не удалось загрузить файл: %s", fetchErr.Reason),
		})
		return false, false
	}
	if telegramapi.IsBotBlockedError(err) {
		return false, false
	}
	if attempt == 0 {
		return false, true
	}
	telegramapi.SendErrorToAdmin(ctx, b, err, fmt.Sprintf("single %s", kind), "", &chatID, username)
	return false, false
}

type groupFetchResult struct {
	resp       *http.Response
	localIndex int
}

func contentLength(resp *http.Response) int64 {
	cl := resp.Header.Get("Content-Length")
	if cl == "" {
		return 0
	}
	size, err := strconv.ParseInt(cl, 10, 64)
	if err != nil {
		return 0
	}
	return size
}

func ProcessMediaGroup(ctx context.Context, b *bot.Bot, st *store.Store, chatID int64, urls []string, kind Kind, platform string, username *string, postURL *string) (bool, error) {
	var validURLs []string
	for _, u := range urls {
		if u != "" {
			validURLs = append(validURLs, u)
		}
	}
	if len(validURLs) == 0 {
		return false, nil
	}

	groupSize := 10
	if kind == KindVideo {
		groupSize = 3
	}
	var groups [][]string
	for i := 0; i < len(validURLs); i += groupSize {
		end := min(i+groupSize, len(validURLs))
		groups = append(groups, validURLs[i:end])
	}

	if postURL != nil && allCached(st, *postURL, kind, len(validURLs)) {
		if sendCachedGroups(ctx, b, st, chatID, groups, groupSize, kind, *postURL) {
			_ = st.RecordCacheEvent(platform, true)
			return true, nil
		}
		_ = st.RecordCacheEvent(platform, false)
		// stale file_ids — fall through to re-download
	} else if postURL != nil {
		_ = st.RecordCacheEvent(platform, false)
	}

	for groupIndex, group := range groups {
		results := fetchGroupConcurrently(ctx, group)
		if len(results) == 0 {
			continue
		}

		var totalSize int64
		for _, r := range results {
			totalSize += contentLength(r.resp)
		}

		var err error
		if totalSize > maxGroupSize {
			err = sendGroupIndividually(ctx, b, st, chatID, results, groupIndex, groupSize, kind, postURL)
		} else {
			err = sendGroupAsMediaGroup(ctx, b, st, chatID, results, groupIndex, groupSize, kind, postURL)
		}
		if err != nil {
			result, _ := classifyMediaError(ctx, b, chatID, kind, username, err, 1)
			return result, nil
		}

		if groupIndex < len(groups)-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}

	return true, nil
}

func allCached(st *store.Store, postURL string, kind Kind, count int) bool {
	for i := 0; i < count; i++ {
		if _, ok, err := st.GetCachedFileID(postURL, string(kind), i); err != nil || !ok {
			return false
		}
	}
	return true
}

func sendCachedGroups(ctx context.Context, b *bot.Bot, st *store.Store, chatID int64, groups [][]string, groupSize int, kind Kind, postURL string) bool {
	for groupIndex, group := range groups {
		type cachedItem struct {
			fileID  string
			caption string
		}
		items := make([]cachedItem, len(group))
		for localIndex := range group {
			globalIndex := groupIndex*groupSize + localIndex
			fileID, _, _ := st.GetCachedFileID(postURL, string(kind), globalIndex)
			caption := ""
			if groupIndex == 0 && localIndex == 0 {
				caption = config.BotTag
			}
			items[localIndex] = cachedItem{fileID, caption}
		}

		var sendErr error
		if len(items) == 1 {
			_, sendErr = sendFromFileID(ctx, b, chatID, kind, items[0].fileID, items[0].caption)
		} else {
			media := make([]models.InputMedia, len(items))
			for i, it := range items {
				media[i] = newFileIDInputMedia(kind, it.fileID, it.caption)
			}
			_, sendErr = telegramapi.SafeSendMediaGroup(ctx, b, &bot.SendMediaGroupParams{ChatID: chatID, Media: media, DisableNotification: true})
		}
		if sendErr != nil {
			return telegramapi.IsBotBlockedError(sendErr)
		}

		if groupIndex < len(groups)-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}
	return true
}

func newFileIDInputMedia(kind Kind, fileID string, caption string) models.InputMedia {
	if kind == KindVideo {
		return &models.InputMediaVideo{Media: fileID, Caption: caption, SupportsStreaming: true}
	}
	return &models.InputMediaPhoto{Media: fileID, Caption: caption}
}

func newUploadInputMedia(kind Kind, attachName string, body io.Reader, caption string) models.InputMedia {
	if kind == KindVideo {
		return &models.InputMediaVideo{Media: attachName, MediaAttachment: body, Caption: caption, SupportsStreaming: true}
	}
	return &models.InputMediaPhoto{Media: attachName, MediaAttachment: body, Caption: caption}
}

func fetchGroupConcurrently(ctx context.Context, group []string) []groupFetchResult {
	results := make([]*groupFetchResult, len(group))
	var wg sync.WaitGroup
	for localIndex, url := range group {
		wg.Add(1)
		go func(localIndex int, url string) {
			defer wg.Done()
			resp, err := FetchMediaResponse(ctx, url, false)
			if err != nil {
				return
			}
			results[localIndex] = &groupFetchResult{resp: resp, localIndex: localIndex}
		}(localIndex, url)
	}
	wg.Wait()

	var valid []groupFetchResult
	for _, r := range results {
		if r != nil {
			valid = append(valid, *r)
		}
	}
	return valid
}

func sendGroupIndividually(ctx context.Context, b *bot.Bot, st *store.Store, chatID int64, results []groupFetchResult, groupIndex, groupSize int, kind Kind, postURL *string) error {
	for i, r := range results {
		globalIndex := groupIndex*groupSize + r.localIndex
		caption := ""
		if groupIndex == 0 && r.localIndex == 0 {
			caption = config.BotTag
		}

		err := telegramapi.WithChatActionErr(ctx, b, chatID, kind.chatAction(), func() error {
			msg, sendErr := uploadMedia(ctx, b, chatID, kind, r.resp.Body, caption)
			if sendErr != nil {
				return sendErr
			}
			if postURL != nil {
				if fileID := messageFileID(kind, msg); fileID != "" {
					st.SetCachedFileID(*postURL, string(kind), globalIndex, fileID)
				}
			}
			return nil
		})
		r.resp.Body.Close()
		if err != nil {
			return err
		}
		if i < len(results)-1 {
			time.Sleep(200 * time.Millisecond)
		}
	}
	return nil
}

func sendGroupAsMediaGroup(ctx context.Context, b *bot.Bot, st *store.Store, chatID int64, results []groupFetchResult, groupIndex, groupSize int, kind Kind, postURL *string) error {
	media := make([]models.InputMedia, len(results))
	for i, r := range results {
		caption := ""
		if groupIndex == 0 && r.localIndex == 0 {
			caption = config.BotTag
		}
		media[i] = newUploadInputMedia(kind, fmt.Sprintf("attach://media%d", i), r.resp.Body, caption)
	}

	msgs, err := telegramapi.WithChatAction(ctx, b, chatID, kind.chatAction(), func() ([]*models.Message, error) {
		return telegramapi.SafeSendMediaGroup(ctx, b, &bot.SendMediaGroupParams{ChatID: chatID, Media: media, DisableNotification: true})
	})
	for _, r := range results {
		r.resp.Body.Close()
	}
	if err != nil {
		return err
	}

	if postURL != nil {
		for i, msg := range msgs {
			if i >= len(results) {
				break
			}
			globalIndex := groupIndex*groupSize + results[i].localIndex
			if fileID := messageFileID(kind, msg); fileID != "" {
				st.SetCachedFileID(*postURL, string(kind), globalIndex, fileID)
			}
		}
	}
	return nil
}
