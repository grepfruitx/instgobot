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

type Post struct {
	Platform string
	Username *string
	URL      *string
	Caption  string
}

func (p Post) captionText() string {
	if p.Caption == "" {
		return config.BotTag
	}
	return p.Caption
}

func firstItemCaption(p Post, groupIndex, localIndex int) string {
	if groupIndex != 0 || localIndex != 0 {
		return ""
	}
	return p.captionText()
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

func ProcessSingleVideo(ctx context.Context, b *bot.Bot, st *store.Store, chatID int64, url string, p Post) (bool, error) {
	return ProcessSingleMedia(ctx, b, st, chatID, url, KindVideo, p)
}

func ProcessSinglePhoto(ctx context.Context, b *bot.Bot, st *store.Store, chatID int64, url string, p Post) (bool, error) {
	return ProcessSingleMedia(ctx, b, st, chatID, url, KindPhoto, p)
}

func ProcessSingleMedia(ctx context.Context, b *bot.Bot, st *store.Store, chatID int64, url string, kind Kind, p Post) (bool, error) {
	if url == "" {
		sent, _ := telegramapi.SafeSendMessage(ctx, b, &bot.SendMessageParams{
			ChatID: chatID, Text: fmt.Sprintf("Не удалось получить URL %s.", kind.ru()),
		})
		if sent != nil {
			telegramapi.SendErrorToAdmin(ctx, b, fmt.Errorf("no %s url", kind), fmt.Sprintf("single %s", kind), "", &chatID, p.Username)
		}
		return false, nil
	}

	if p.URL != nil {
		if fileID, ok, err := st.GetCachedFileID(*p.URL, string(kind), 0); err == nil && ok {
			_, sendErr := sendFromFileID(ctx, b, chatID, kind, fileID, p.captionText())
			if sendErr == nil {
				_ = st.RecordCacheEvent(p.Platform, true)
				return true, nil
			}
			if telegramapi.IsBotBlockedError(sendErr) {
				return false, nil
			}
			_ = st.RecordCacheEvent(p.Platform, false)
			// stale file_id — fall through to re-download
		} else {
			_ = st.RecordCacheEvent(p.Platform, false)
		}
	}

	for attempt := 0; attempt < 2; attempt++ {
		result, retry := attemptSingleDownloadAndSend(ctx, b, st, chatID, url, kind, p, attempt)
		if !retry {
			return result, nil
		}
		time.Sleep(1500 * time.Millisecond)
	}
	return false, nil
}

func attemptSingleDownloadAndSend(ctx context.Context, b *bot.Bot, st *store.Store, chatID int64, url string, kind Kind, p Post, attempt int) (result bool, retry bool) {
	resp, err := FetchMediaResponse(ctx, url, false)
	if err != nil {
		return classifyMediaError(ctx, b, chatID, kind, p.Username, err, attempt)
	}
	defer resp.Body.Close()

	sendErr := telegramapi.WithChatActionErr(ctx, b, chatID, kind.chatAction(), func() error {
		msg, err := uploadMedia(ctx, b, chatID, kind, resp.Body, p.captionText())
		if err != nil {
			return err
		}
		if p.URL != nil {
			if fileID := messageFileID(kind, msg); fileID != "" {
				st.SetCachedFileID(*p.URL, string(kind), 0, fileID)
			}
		}
		return nil
	})

	if sendErr == nil {
		return true, false
	}
	return classifyMediaError(ctx, b, chatID, kind, p.Username, sendErr, attempt)
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

func ProcessMediaGroup(ctx context.Context, b *bot.Bot, st *store.Store, chatID int64, urls []string, kind Kind, p Post) (bool, error) {
	var validURLs []string
	for _, u := range urls {
		if u != "" {
			validURLs = append(validURLs, u)
		}
	}
	if len(validURLs) == 0 {
		return false, nil
	}

	groupSize := groupSizeFor(kind)
	var groups [][]string
	for i := 0; i < len(validURLs); i += groupSize {
		end := min(i+groupSize, len(validURLs))
		groups = append(groups, validURLs[i:end])
	}

	if p.URL != nil && allCached(st, *p.URL, kind, len(validURLs)) {
		if sendCachedGroups(ctx, b, st, chatID, groups, groupSize, kind, *p.URL, p) {
			_ = st.RecordCacheEvent(p.Platform, true)
			return true, nil
		}
		_ = st.RecordCacheEvent(p.Platform, false)
		// stale file_ids — fall through to re-download
	} else if p.URL != nil {
		_ = st.RecordCacheEvent(p.Platform, false)
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
			err = sendGroupIndividually(ctx, b, st, chatID, results, groupIndex, groupSize, kind, p)
		} else {
			err = sendGroupAsMediaGroup(ctx, b, st, chatID, results, groupIndex, groupSize, kind, p)
		}
		if err != nil {
			result, _ := classifyMediaError(ctx, b, chatID, kind, p.Username, err, 1)
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

func sendCachedGroups(ctx context.Context, b *bot.Bot, st *store.Store, chatID int64, groups [][]string, groupSize int, kind Kind, postURL string, p Post) bool {
	for groupIndex, group := range groups {
		type cachedItem struct {
			fileID  string
			caption string
		}
		items := make([]cachedItem, len(group))
		for localIndex := range group {
			globalIndex := groupIndex*groupSize + localIndex
			fileID, _, _ := st.GetCachedFileID(postURL, string(kind), globalIndex)
			items[localIndex] = cachedItem{fileID, firstItemCaption(p, groupIndex, localIndex)}
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

func sendGroupIndividually(ctx context.Context, b *bot.Bot, st *store.Store, chatID int64, results []groupFetchResult, groupIndex, groupSize int, kind Kind, p Post) error {
	for i, r := range results {
		globalIndex := groupIndex*groupSize + r.localIndex
		caption := firstItemCaption(p, groupIndex, r.localIndex)

		err := telegramapi.WithChatActionErr(ctx, b, chatID, kind.chatAction(), func() error {
			msg, sendErr := uploadMedia(ctx, b, chatID, kind, r.resp.Body, caption)
			if sendErr != nil {
				return sendErr
			}
			if p.URL != nil {
				if fileID := messageFileID(kind, msg); fileID != "" {
					st.SetCachedFileID(*p.URL, string(kind), globalIndex, fileID)
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

func sendGroupAsMediaGroup(ctx context.Context, b *bot.Bot, st *store.Store, chatID int64, results []groupFetchResult, groupIndex, groupSize int, kind Kind, p Post) error {
	media := make([]models.InputMedia, len(results))
	for i, r := range results {
		media[i] = newUploadInputMedia(kind, fmt.Sprintf("attach://media%d", i), r.resp.Body, firstItemCaption(p, groupIndex, r.localIndex))
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

	if p.URL != nil {
		for i, msg := range msgs {
			if i >= len(results) {
				break
			}
			globalIndex := groupIndex*groupSize + results[i].localIndex
			if fileID := messageFileID(kind, msg); fileID != "" {
				st.SetCachedFileID(*p.URL, string(kind), globalIndex, fileID)
			}
		}
	}
	return nil
}

func groupsForCount(count, groupSize int) [][]string {
	var groups [][]string
	for i := 0; i < count; i += groupSize {
		end := min(i+groupSize, count)
		groups = append(groups, make([]string, end-i))
	}
	return groups
}

func groupSizeFor(kind Kind) int {
	if kind == KindVideo {
		return 3
	}
	return 10
}

func SendCachedPost(ctx context.Context, b *bot.Bot, st *store.Store, chatID int64, postURL string, photoCount, videoCount int, p Post) bool {
	if photoCount == 0 && videoCount == 0 {
		return false
	}
	if !allCached(st, postURL, KindPhoto, photoCount) || !allCached(st, postURL, KindVideo, videoCount) {
		return false
	}

	photoPost, videoPost := p, p
	if photoCount > 0 {
		videoPost.Caption = ""
	}

	if photoCount > 0 {
		if !sendCachedGroups(ctx, b, st, chatID, groupsForCount(photoCount, groupSizeFor(KindPhoto)), groupSizeFor(KindPhoto), KindPhoto, postURL, photoPost) {
			return false
		}
	}
	if videoCount > 0 {
		if !sendCachedGroups(ctx, b, st, chatID, groupsForCount(videoCount, groupSizeFor(KindVideo)), groupSizeFor(KindVideo), KindVideo, postURL, videoPost) {
			return false
		}
	}
	return true
}
