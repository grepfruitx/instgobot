package youtube

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"math/rand"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/redis/go-redis/v9"

	"github.com/grepfruitx/instgobot/internal/config"
	"github.com/grepfruitx/instgobot/internal/media"
	"github.com/grepfruitx/instgobot/internal/ratelimit"
	"github.com/grepfruitx/instgobot/internal/store"
	"github.com/grepfruitx/instgobot/internal/telegramapi"
)

var staticVideoQualities = []int{144, 240, 360, 480, 720}

const shortsDefaultQuality = 720

type Handler struct {
	b       *bot.Bot
	st      *store.Store
	rdb     *redis.Client
	limiter *ratelimit.Limiter
	cfg     *config.Config
}

func New(b *bot.Bot, st *store.Store, rdb *redis.Client, limiter *ratelimit.Limiter, cfg *config.Config) *Handler {
	return &Handler{b: b, st: st, rdb: rdb, limiter: limiter, cfg: cfg}
}

func videoCacheType(quality int) string { return fmt.Sprintf("yt_v_%d", quality) }

const audioCacheType = "yt_a_best"

func (h *Handler) SendQualityPicker(ctx context.Context, chatID, userID int64, url string, username *string) {
	isAdmin := config.IsAdmin(userID)
	rl, err := h.limiter.PeekYouTube(ctx, userID, isAdmin)
	if err != nil {
		telegramapi.SendErrorToAdmin(ctx, h.b, err, "youtube rate limit peek", url, &chatID, username)
	} else if !rl.Allowed {
		sec := int(math.Ceil(time.Until(rl.ResetTime).Seconds()))
		_, _ = telegramapi.SendText(ctx, h.b, chatID, fmt.Sprintf("⚡ Лимит: 1 загрузка в 3 минуты. Повторите через %d сек.", sec))
		return
	}

	if err := setPendingURL(ctx, h.rdb, chatID, url); err != nil {
		telegramapi.SendErrorToAdmin(ctx, h.b, err, "youtube quality picker", url, &chatID, username)
		return
	}

	var videoButtons [][]models.InlineKeyboardButton
	for _, q := range staticVideoQualities {
		prefix := ""
		if _, ok, _ := h.st.GetCachedFileID(url, videoCacheType(q), 0); ok {
			prefix = "⚡ "
		}
		videoButtons = append(videoButtons, []models.InlineKeyboardButton{{
			Text:         fmt.Sprintf("%s🎬 %dp", prefix, q),
			CallbackData: fmt.Sprintf("yt:%d:v:%d", chatID, q),
		}})
	}

	audioPrefix := ""
	if _, ok, _ := h.st.GetCachedFileID(url, audioCacheType, 0); ok {
		audioPrefix = "⚡ "
	}

	_, err = h.b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      "🎬 <b>YouTube видео</b>\n\nВыберите формат:\n<i>Максимум в Telegram — 2 ГБ</i>",
		ParseMode: models.ParseModeHTML,
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: append(videoButtons, []models.InlineKeyboardButton{{
				Text: fmt.Sprintf("%s🎵 Аудио", audioPrefix), CallbackData: fmt.Sprintf("yt:%d:a:0", chatID),
			}}),
		},
	})
	if err != nil && !telegramapi.IsBotBlockedError(err) {
		_, _ = telegramapi.SendText(ctx, h.b, chatID, "Не удалось отправить меню.")
		telegramapi.SendErrorToAdmin(ctx, h.b, err, "youtube quality picker", url, &chatID, username)
	}
}

var callbackDataRe = regexp.MustCompile(`^yt:(\d+):(v|a):(\d+)$`)

func ParseCallbackData(data string) (chatID int64, kind string, quality int, ok bool) {
	m := callbackDataRe.FindStringSubmatch(data)
	if m == nil {
		return 0, "", 0, false
	}
	chatID, _ = strconv.ParseInt(m[1], 10, 64)
	quality, _ = strconv.Atoi(m[3])
	return chatID, m[2], quality, true
}

func (h *Handler) setPickerLoading(ctx context.Context, chatID int64, messageID int) {
	if messageID == 0 {
		return
	}
	_, _ = h.b.EditMessageReplyMarkup(ctx, &bot.EditMessageReplyMarkupParams{
		ChatID:    chatID,
		MessageID: messageID,
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{{{Text: "⏳ Загрузка...", CallbackData: "yt:noop"}}},
		},
	})
}

func (h *Handler) clearPickerKeyboard(ctx context.Context, chatID int64, messageID int) {
	if messageID == 0 {
		return
	}
	_, _ = h.b.EditMessageReplyMarkup(ctx, &bot.EditMessageReplyMarkupParams{
		ChatID:      chatID,
		MessageID:   messageID,
		ReplyMarkup: &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{}},
	})
}

func (h *Handler) HandleCallback(ctx context.Context, chatID int64, kind string, quality int, userID int64, messageID int, username *string) {
	isAdmin := config.IsAdmin(userID)

	if !isAdmin && !markDownloadActive(userID) {
		_, _ = telegramapi.SendText(ctx, h.b, chatID, "⏳ Дождитесь окончания текущей загрузки.")
		return
	}

	rl, err := h.limiter.CheckYouTube(ctx, userID, isAdmin)
	if err != nil {
		markDownloadDone(userID)
		telegramapi.SendErrorToAdmin(ctx, h.b, err, "youtube download", "", &chatID, username)
		return
	}
	if !rl.Allowed {
		markDownloadDone(userID)
		sec := int(math.Ceil(time.Until(rl.ResetTime).Seconds()))
		_, _ = telegramapi.SendText(ctx, h.b, chatID, fmt.Sprintf("⚡ Лимит: 1 загрузка в 3 минуты. Повторите через %d сек.", sec))
		return
	}

	defer markDownloadDone(userID)

	url, ok, err := getPendingURL(ctx, h.rdb, chatID)
	if err != nil {
		telegramapi.SendErrorToAdmin(ctx, h.b, err, "youtube download", "", &chatID, username)
		return
	}
	if !ok {
		_, _ = telegramapi.SendText(ctx, h.b, chatID, "Сессия истекла. Отправьте ссылку заново.")
		return
	}
	_ = deletePendingURL(ctx, h.rdb, chatID)

	h.setPickerLoading(ctx, chatID, messageID)
	defer h.clearPickerKeyboard(ctx, chatID, messageID)

	if kind == "a" {
		sent := h.sendAudio(ctx, chatID, url, username)
		h.st.RecordDownloadLogged(chatID, url, "youtube", "audio", sent, username, nil)
		return
	}
	sent := h.downloadAndSendVideo(ctx, chatID, url, quality, username, true)
	h.st.RecordDownloadLogged(chatID, url, "youtube", "video", sent, username, nil)
}

func (h *Handler) sendAudio(ctx context.Context, chatID int64, url string, username *string) bool {
	activeJobs.Add(1)
	defer activeJobs.Done()

	if cached, ok, _ := h.st.GetCachedFileID(url, audioCacheType, 0); ok {
		_, err := h.b.SendAudio(ctx, &bot.SendAudioParams{ChatID: chatID, Audio: &models.InputFileString{Data: cached}, Caption: config.BotTag, DisableNotification: true})
		if err == nil {
			_ = h.st.RecordCacheEvent("youtube", true)
			return true
		}
		if telegramapi.IsBotBlockedError(err) {
			return false
		}
		_ = h.st.RecordCacheEvent("youtube", false)
		// stale file_id — fall through to re-download
	} else {
		_ = h.st.RecordCacheEvent("youtube", false)
	}

	sent := false
	err := telegramapi.WithChatActionErr(ctx, h.b, chatID, models.ChatActionUploadDocument, func() error {
		stream, err := ytDlpStream(ctx, h.cfg.YtDlpPath, []string{"-f", "bestaudio[ext=m4a]/bestaudio", "--no-playlist", "-o", "-", url})
		if err != nil {
			return err
		}
		defer stream.Close()

		msg, err := h.b.SendAudio(ctx, &bot.SendAudioParams{
			ChatID: chatID, Audio: &models.InputFileUpload{Filename: "audio.m4a", Data: stream},
			Caption: config.BotTag, DisableNotification: true,
		})
		if err != nil {
			return err
		}
		sent = true
		if msg.Audio != nil {
			_ = h.st.SetCachedFileID(url, audioCacheType, 0, msg.Audio.FileID)
		}
		return nil
	})
	if err != nil && !telegramapi.IsBotBlockedError(err) {
		_, _ = telegramapi.SendText(ctx, h.b, chatID, "Не удалось скачать. Попробуйте ещё раз.")
		telegramapi.SendErrorToAdmin(ctx, h.b, err, "youtube download", url, &chatID, username)
	}
	return sent
}

func fetchThumbnail(ctx context.Context, url string) []byte {
	if url == "" {
		return nil
	}
	resp, err := media.FetchWithTimeout(ctx, url, 5*time.Second)
	if err != nil {
		slog.Warn("thumbnail fetch failed", "url", url, "error", err)
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slog.Warn("thumbnail fetch non-2xx", "url", url, "status", resp.StatusCode)
		return nil
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Warn("thumbnail read failed", "url", url, "error", err)
		return nil
	}
	return data
}

func (h *Handler) downloadAndSendVideo(ctx context.Context, chatID int64, url string, quality int, username *string, notifyFallback bool) bool {
	activeJobs.Add(1)
	defer activeJobs.Done()

	sent := false

	err := telegramapi.WithChatActionErr(ctx, h.b, chatID, models.ChatActionUploadVideo, func() error {
		meta, err := getYtMeta(ctx, h.rdb, h.cfg.YtDlpPath, url)
		if err != nil {
			return err
		}
		chosen := chooseVideoFormat(meta.Formats, quality)
		if chosen == nil {
			return errors.New("no video format found")
		}

		if notifyFallback && chosen.Height > 0 && chosen.Height < quality {
			_, _ = telegramapi.SendText(ctx, h.b, chatID, fmt.Sprintf("ℹ️ %dp недоступно, скачиваю лучшее: %dp", quality, chosen.Height))
		}

		cacheType := videoCacheType(quality)
		caption := fmt.Sprintf("%s\n\n%s", meta.Title, config.BotTag)
		videoOpts := &bot.SendVideoParams{
			ChatID: chatID, Caption: caption, DisableNotification: true, SupportsStreaming: true,
			Width: chosen.Width, Height: chosen.Height, Duration: meta.Duration,
		}
		if thumb := fetchThumbnail(ctx, meta.ThumbnailURL); thumb != nil {
			videoOpts.Thumbnail = &models.InputFileUpload{Filename: "thumb.jpg", Data: bytes.NewReader(thumb)}
		}

		if cached, ok, _ := h.st.GetCachedFileID(url, cacheType, 0); ok {
			videoOpts.Video = &models.InputFileString{Data: cached}
			_, sendErr := h.b.SendVideo(ctx, videoOpts)
			if sendErr == nil {
				sent = true
				_ = h.st.RecordCacheEvent("youtube", true)
				return nil
			}
			if telegramapi.IsBotBlockedError(sendErr) {
				return sendErr
			}
			_ = h.st.RecordCacheEvent("youtube", false)
			// stale file_id — fall through to re-download
		} else {
			_ = h.st.RecordCacheEvent("youtube", false)
		}

		rnd := rand.Intn(100000) + 1

		var sendErr error
		if chosen.Kind == kindAdaptive {
			sendErr = h.downloadAdaptive(ctx, chatID, url, cacheType, chosen, videoOpts, rnd, &sent)
		} else {
			sendErr = h.downloadMuxed(ctx, url, cacheType, chosen, videoOpts, rnd, &sent)
		}
		return sendErr
	})

	if err != nil && !telegramapi.IsBotBlockedError(err) {
		_, _ = telegramapi.SendText(ctx, h.b, chatID, "Не удалось скачать. Попробуйте ещё раз.")
		telegramapi.SendErrorToAdmin(ctx, h.b, err, "youtube download", url, &chatID, username)
	}
	return sent
}

func (h *Handler) downloadAdaptive(ctx context.Context, chatID int64, url, cacheType string, chosen *chosenVideo, videoOpts *bot.SendVideoParams, rnd int, sent *bool) error {
	if !acquireAdaptiveSlot() {
		_, _ = telegramapi.SendText(ctx, h.b, chatID, "⏳ Сервер сейчас занят обработкой видео в высоком качестве. Попробуйте через минуту.")
		return nil
	}
	if !hasEnoughDiskSpace() {
		releaseAdaptiveSlot()
		_, _ = telegramapi.SendText(ctx, h.b, chatID, "⏳ Сервер сейчас занят обработкой видео в высоком качестве. Попробуйте через минуту.")
		return nil
	}
	defer releaseAdaptiveSlot()

	tmpPath := fmt.Sprintf("%s/yt_%d_%d.mp4", os.TempDir(), time.Now().UnixMilli(), rnd)
	defer os.Remove(tmpPath)

	if err := ytDlpMergeToDisk(ctx, h.cfg.YtDlpPath, []string{
		"-f", fmt.Sprintf("%s+%s", chosen.VideoFormatID, chosen.AudioFormatID),
		"--no-playlist", "--merge-output-format", "mp4", url,
	}, tmpPath); err != nil {
		return err
	}

	f, err := os.Open(tmpPath)
	if err != nil {
		return err
	}
	defer f.Close()

	videoOpts.Video = &models.InputFileUpload{Filename: fmt.Sprintf("video_%d.mp4", rnd), Data: f}
	msg, err := h.b.SendVideo(ctx, videoOpts)
	if err != nil {
		return err
	}
	*sent = true
	if msg.Video != nil {
		_ = h.st.SetCachedFileID(url, cacheType, 0, msg.Video.FileID)
	}
	return nil
}

func (h *Handler) downloadMuxed(ctx context.Context, url, cacheType string, chosen *chosenVideo, videoOpts *bot.SendVideoParams, rnd int, sent *bool) error {
	stream, err := ytDlpStream(ctx, h.cfg.YtDlpPath, []string{"-f", chosen.FormatID, "--no-playlist", "-o", "-", url})
	if err != nil {
		return err
	}
	defer stream.Close()

	videoOpts.Video = &models.InputFileUpload{Filename: fmt.Sprintf("video_%d.mp4", rnd), Data: stream}
	msg, err := h.b.SendVideo(ctx, videoOpts)
	if err != nil {
		return err
	}
	*sent = true
	if msg.Video != nil {
		_ = h.st.SetCachedFileID(url, cacheType, 0, msg.Video.FileID)
	}
	return nil
}

func (h *Handler) ProcessShorts(ctx context.Context, chatID int64, url string, username, firstName *string) {
	sent := h.downloadAndSendVideo(ctx, chatID, url, shortsDefaultQuality, username, false)
	h.st.RecordDownloadLogged(chatID, url, "youtube", "video", sent, username, firstName)
}
