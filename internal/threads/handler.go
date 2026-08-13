package threads

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"
	"unicode/utf16"

	"github.com/go-telegram/bot"

	"github.com/grepfruitx/instgobot/internal/config"
	"github.com/grepfruitx/instgobot/internal/media"
	"github.com/grepfruitx/instgobot/internal/messages"
	"github.com/grepfruitx/instgobot/internal/platform"
	"github.com/grepfruitx/instgobot/internal/store"
	"github.com/grepfruitx/instgobot/internal/telegramapi"
)

var apiBaseURL = "https://threadsdownloads.com/api/info"

const apiTimeout = 25 * time.Second

func getPostContent(ctx context.Context, threadsURL string) (postContent, error) {
	resp, err := media.PostJSON(ctx, apiBaseURL, map[string]string{"url": threadsURL}, apiTimeout)
	if err != nil {
		return postContent{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return postContent{}, err
	}

	return parseAPIResponse(body, resp.StatusCode)
}

func sendPostText(ctx context.Context, b *bot.Bot, chatID int64, text string) {
	for _, chunk := range messages.SplitMessage(text, messages.DefaultMaxLength) {
		_, _ = telegramapi.SendText(ctx, b, chatID, chunk)
	}
}

const maxCaptionUnits = 1024

func buildCaption(text string) (caption string, sendSeparately bool) {
	if text == "" {
		return "", false
	}
	full := fmt.Sprintf("%s\n\n%s", text, config.BotTag)
	if len(utf16.Encode([]rune(full))) <= maxCaptionUnits {
		return full, false
	}
	return "", true
}

func Process(ctx context.Context, b *bot.Bot, st *store.Store, chatID int64, message string, username, firstName *string) {
	plat := platform.DetectPlatform(message)

	postURLStr := platform.NormalizePostURL(message)

	textSent := false
	if cached, ok, _ := st.GetPostCache(postURLStr); ok {
		if !cached.HasMedia() {
			if cached.PostText != "" {
				sendPostText(ctx, b, chatID, cached.PostText)
			}
			_ = st.RecordCacheEvent(plat, true)
			st.RecordDownloadLogged(chatID, message, plat, "text", true, username, firstName)
			return
		}

		caption, separate := buildCaption(cached.PostText)
		if separate {
			sendPostText(ctx, b, chatID, cached.PostText)
			textSent = true
		}
		cachedPost := media.Post{Platform: plat, Username: username, URL: &postURLStr, Caption: caption}
		if media.SendCachedPost(ctx, b, st, chatID, postURLStr, cached.PhotoCount, cached.VideoCount, cachedPost) {
			_ = st.RecordCacheEvent(plat, true)
			st.RecordDownloadLogged(chatID, message, plat, cachedMediaType(cached), true, username, firstName)
			return
		}
		_ = st.RecordCacheEvent(plat, false)
	} else {
		_ = st.RecordCacheEvent(plat, false)
	}

	content, err := getPostContent(ctx, message)
	if err != nil {
		text := "Не удалось скачать медиа с Threads. Попробуйте еще раз."
		reportToAdmin := true

		var apiErr *APIError
		if errors.As(err, &apiErr) {
			if userText := apiErr.UserFacing(); userText != "" {
				text = userText
				reportToAdmin = false
			}
		}

		_, _ = telegramapi.SafeSendMessage(ctx, b, &bot.SendMessageParams{ChatID: chatID, Text: text})
		if reportToAdmin {
			telegramapi.SendErrorToAdmin(ctx, b, err, "threads download", message, &chatID, username)
		}
		st.RecordDownloadLogged(chatID, message, plat, "unknown", false, username, firstName)
		return
	}

	if !content.hasMedia() {
		if content.Text != "" && !textSent {
			sendPostText(ctx, b, chatID, content.Text)
		}
		if content.Text == "" {
			_, _ = telegramapi.SafeSendMessage(ctx, b, &bot.SendMessageParams{
				ChatID: chatID, Text: "В этом посте нет ни медиафайлов, ни текста.",
			})
			st.RecordDownloadLogged(chatID, message, plat, "unknown", false, username, firstName)
			return
		}
		_ = st.SetPostCache(postURLStr, 0, 0, content.Text)
		st.RecordDownloadLogged(chatID, message, plat, "text", true, username, firstName)
		return
	}

	photos, videos := content.Photos, content.Videos

	caption := ""
	if !textSent {
		var separate bool
		caption, separate = buildCaption(content.Text)
		if separate {
			sendPostText(ctx, b, chatID, content.Text)
			caption = ""
		}
	}

	photoPost := media.Post{Platform: plat, Username: username, URL: &postURLStr, Caption: caption}
	videoPost := media.Post{Platform: plat, Username: username, URL: &postURLStr, Caption: caption}
	if len(photos) > 0 {
		videoPost.Caption = ""
	}

	var photoOK, videoOK bool
	switch len(photos) {
	case 1:
		photoOK, _ = media.ProcessSinglePhoto(ctx, b, st, chatID, photos[0], photoPost)
	default:
		if len(photos) > 1 {
			photoOK, _ = media.ProcessMediaGroup(ctx, b, st, chatID, photos, media.KindPhoto, photoPost)
		}
	}
	switch len(videos) {
	case 1:
		videoOK, _ = media.ProcessSingleVideo(ctx, b, st, chatID, videos[0], videoPost)
	default:
		if len(videos) > 1 {
			videoOK, _ = media.ProcessMediaGroup(ctx, b, st, chatID, videos, media.KindVideo, videoPost)
		}
	}

	mediaType := "video"
	if len(photos) > 0 {
		mediaType = "photo"
	}
	ok := photoOK || videoOK
	if ok {
		if photoOK == (len(photos) > 0) && videoOK == (len(videos) > 0) {
			_ = st.SetPostCache(postURLStr, len(photos), len(videos), content.Text)
		}
	}
	st.RecordDownloadLogged(chatID, message, plat, mediaType, ok, username, firstName)
}
