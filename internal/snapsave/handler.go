package snapsave

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	smd "github.com/grepfruitx/snapmedia-downloader"

	"github.com/grepfruitx/instgobot/internal/config"
	"github.com/grepfruitx/instgobot/internal/media"
	"github.com/grepfruitx/instgobot/internal/platform"
	"github.com/grepfruitx/instgobot/internal/store"
	"github.com/grepfruitx/instgobot/internal/telegramapi"
)

const chromeUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36"

func handleUnderlineEnding(text string) string {
	if strings.HasSuffix(text, "_") {
		return text + "/"
	}
	return text
}

var instagramStoriesPageRe = regexp.MustCompile(`instagram\.com/stories/[^/]+/?$`)

func Process(ctx context.Context, b *bot.Bot, st *store.Store, chatID int64, message string, adminUsername string, username, firstName *string) {
	plat := platform.DetectPlatform(message)

	downloadTarget := message
	if plat == "instagram" {
		if igUsername, ok := platform.GetInstagramProfileUsername(message); ok {
			downloadTarget = platform.ToInstagramStoriesLink(igUsername)
		}
	}

	formatted := handleUnderlineEnding(downloadTarget)
	resp := smd.Download(formatted, &smd.Options{Retry: 3, RetryDelay: 500 * time.Millisecond, UserAgent: chromeUA})

	if !resp.Success {
		if plat == "twitter" {
			processTweetImageFallback(ctx, b, st, chatID, message, plat, username, firstName)
			return
		}

		telegramapi.SendText(ctx, b, chatID, fmt.Sprintf(
			"Не удалось скачать медиафайл.\nУбедитесь, что медиафайл существует и не является приватным.\nЕсли ошибка возникает многократно, пишите %s",
			adminUsername,
		))
		telegramapi.SendErrorToAdmin(ctx, b, errors.New(resp.Message), "snapsave download", message, &chatID, username)
		st.RecordDownloadLogged(chatID, message, plat, "unknown", false, username, firstName)
		return
	}

	if resp.Data == nil || len(resp.Data.Media) == 0 {
		telegramapi.SendText(ctx, b, chatID, "Не удалось скачать медиа. Попробуйте еще раз.")
		telegramapi.SendErrorToAdmin(ctx, b, errors.New("no media in response"), "media check", message, &chatID, username)
		st.RecordDownloadLogged(chatID, message, plat, "unknown", false, username, firstName)
		return
	}

	var videos, photos []string
	for _, m := range resp.Data.Media {
		if m.URL == "" {
			continue
		}
		switch m.Type {
		case smd.MediaVideo:
			videos = append(videos, m.URL)
		case smd.MediaImage:
			photos = append(photos, m.URL)
		}
	}

	if len(videos) == 0 && len(photos) == 0 {
		if plat == "twitter" {
			processTweetImageFallback(ctx, b, st, chatID, message, plat, username, firstName)
			return
		}
		telegramapi.SendText(ctx, b, chatID, "Не удалось скачать медиа. Попробуйте еще раз.")
		telegramapi.SendErrorToAdmin(ctx, b, errors.New("media items had no usable url"), "media check", message, &chatID, username)
		st.RecordDownloadLogged(chatID, message, plat, "unknown", false, username, firstName)
		return
	}

	isEphemeralStoriesPage := instagramStoriesPageRe.MatchString(strings.Split(downloadTarget, "?")[0])
	var postURL *string
	if !isEphemeralStoriesPage {
		u := strings.TrimSuffix(strings.Split(message, "?")[0], "/")
		postURL = &u
	}

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

const tweetImageCacheType = "tweet_image"

func processTweetImageFallback(ctx context.Context, b *bot.Bot, st *store.Store, chatID int64, message, plat string, username, firstName *string) {
	postURL := strings.TrimSuffix(strings.Split(message, "?")[0], "/")

	if cached, ok, _ := st.GetCachedFileID(postURL, tweetImageCacheType, 0); ok {
		if _, err := telegramapi.SafeSendPhoto(ctx, b, &bot.SendPhotoParams{
			ChatID: chatID, Photo: &models.InputFileString{Data: cached},
			Caption: config.BotTag, DisableNotification: true,
		}); err == nil {
			_ = st.RecordCacheEvent(plat, true)
			st.RecordDownloadLogged(chatID, message, plat, "image", true, username, firstName)
			return
		}
		_ = st.RecordCacheEvent(plat, false)
		// stale file_id — fall through to re-render
	} else {
		_ = st.RecordCacheEvent(plat, false)
	}

	imgBuf, err := convertTweetToImage(ctx, message)
	if err != nil || len(imgBuf) == 0 {
		telegramapi.SendText(ctx, b, chatID, "Не удалось конвертировать твит в изображение.")
		if err == nil {
			err = errors.New("tweet to image conversion returned no data")
		}
		telegramapi.SendErrorToAdmin(ctx, b, err, "tweet to image", message, &chatID, username)
		st.RecordDownloadLogged(chatID, message, plat, "image", false, username, firstName)
		return
	}

	msg, _ := telegramapi.SafeSendPhoto(ctx, b, &bot.SendPhotoParams{
		ChatID: chatID, Photo: &models.InputFileUpload{Filename: "tweet.png", Data: bytes.NewReader(imgBuf)},
		Caption: config.BotTag, DisableNotification: true,
	})
	if msg != nil && len(msg.Photo) > 0 {
		st.SetCachedFileID(postURL, tweetImageCacheType, 0, msg.Photo[len(msg.Photo)-1].FileID)
	}
	st.RecordDownloadLogged(chatID, message, plat, "image", true, username, firstName)
}
