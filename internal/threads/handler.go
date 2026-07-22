package threads

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/go-telegram/bot"

	"github.com/grepfruitx/instgobot/internal/media"
	"github.com/grepfruitx/instgobot/internal/platform"
	"github.com/grepfruitx/instgobot/internal/store"
	"github.com/grepfruitx/instgobot/internal/telegramapi"
)

type apiResponse struct {
	ImageURLs []string          `json:"image_urls"`
	VideoURLs []json.RawMessage `json:"video_urls"`
}

func normalizeVideoURLs(raw []json.RawMessage) []string {
	urls := make([]string, 0, len(raw))
	for _, r := range raw {
		var s string
		if err := json.Unmarshal(r, &s); err == nil {
			urls = append(urls, s)
			continue
		}
		var obj struct {
			DownloadURL string `json:"download_url"`
		}
		if err := json.Unmarshal(r, &obj); err == nil {
			urls = append(urls, obj.DownloadURL)
		}
	}
	return urls
}

var apiBaseURL = "https://api.threadsphotodownloader.com/v2/media"

func getDownloadLinks(ctx context.Context, threadsURL string) (photos []string, videos []string, err error) {
	apiURL := fmt.Sprintf("%s?url=%s", apiBaseURL, url.QueryEscape(threadsURL))
	resp, err := media.FetchWithTimeout(ctx, apiURL, 15*time.Second)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, fmt.Errorf("HTTP error! status: %d", resp.StatusCode)
	}

	var data apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, nil, err
	}

	return data.ImageURLs, normalizeVideoURLs(data.VideoURLs), nil
}

func Process(ctx context.Context, b *bot.Bot, st *store.Store, chatID int64, message string, username, firstName *string) {
	plat := platform.DetectPlatform(message)

	photos, videos, err := getDownloadLinks(ctx, message)
	if err != nil {
		_, _ = telegramapi.SafeSendMessage(ctx, b, &bot.SendMessageParams{
			ChatID: chatID, Text: "Не удалось скачать медиа с Threads. Попробуйте еще раз.",
		})
		telegramapi.SendErrorToAdmin(ctx, b, err, "threads download", message, &chatID, username)
		st.RecordDownloadLogged(chatID, message, plat, "unknown", false, username, firstName)
		return
	}

	if len(photos) == 0 && len(videos) == 0 {
		_, _ = telegramapi.SafeSendMessage(ctx, b, &bot.SendMessageParams{
			ChatID: chatID, Text: "Не удалось получить медиафайлы из Threads.",
		})
		st.RecordDownloadLogged(chatID, message, plat, "unknown", false, username, firstName)
		return
	}

	postURLStr := strings.TrimSuffix(strings.Split(message, "?")[0], "/")
	postURL := &postURLStr

	var photoOK, videoOK bool
	switch len(photos) {
	case 1:
		photoOK, _ = media.ProcessSinglePhoto(ctx, b, st, chatID, photos[0], plat, username, postURL)
	default:
		if len(photos) > 1 {
			photoOK, _ = media.ProcessMediaGroup(ctx, b, st, chatID, photos, media.KindPhoto, plat, username, postURL)
		}
	}
	switch len(videos) {
	case 1:
		videoOK, _ = media.ProcessSingleVideo(ctx, b, st, chatID, videos[0], plat, username, postURL)
	default:
		if len(videos) > 1 {
			videoOK, _ = media.ProcessMediaGroup(ctx, b, st, chatID, videos, media.KindVideo, plat, username, postURL)
		}
	}

	mediaType := "video"
	if len(photos) > 0 {
		mediaType = "photo"
	}
	st.RecordDownloadLogged(chatID, message, plat, mediaType, photoOK || videoOK, username, firstName)
}
