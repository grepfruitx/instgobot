package userbot

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/gotd/td/telegram/peers"
	"github.com/gotd/td/tg"

	"github.com/grepfruitx/instgobot/internal/config"
	"github.com/grepfruitx/instgobot/internal/telegramapi"
)

func messagesOf(v tg.MessagesMessagesClass) []tg.MessageClass {
	switch m := v.(type) {
	case *tg.MessagesMessages:
		return m.Messages
	case *tg.MessagesMessagesSlice:
		return m.Messages
	case *tg.MessagesChannelMessages:
		return m.Messages
	default:
		return nil
	}
}

func getPostMessages(ctx context.Context, api *tg.Client, channel tg.InputChannelClass, peer tg.InputPeerClass, messageID int) ([]*tg.Message, error) {
	resp, err := api.ChannelsGetMessages(ctx, &tg.ChannelsGetMessagesRequest{
		Channel: channel,
		ID:      []tg.InputMessageClass{&tg.InputMessageID{ID: messageID}},
	})
	if err != nil {
		return nil, err
	}

	all := messagesOf(resp)
	if len(all) == 0 {
		return nil, nil
	}
	first, ok := all[0].(*tg.Message)
	if !ok {
		return nil, nil
	}

	groupID, hasGroup := first.GetGroupedID()
	if !hasGroup || groupID == 0 {
		return []*tg.Message{first}, nil
	}

	history, err := api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
		Peer: peer, OffsetID: messageID + 10, Limit: 20,
	})
	if err != nil {
		return []*tg.Message{first}, nil
	}

	var album []*tg.Message
	for _, m := range messagesOf(history) {
		if full, ok := m.(*tg.Message); ok {
			if gid, ok := full.GetGroupedID(); ok && gid == groupID {
				album = append(album, full)
			}
		}
	}
	if len(album) == 0 {
		return []*tg.Message{first}, nil
	}
	return album, nil
}

func (h *Handler) sendPostMessages(ctx context.Context, chatID int64, api *tg.Client, messages []*tg.Message) bool {
	type withMedia struct {
		loc  tg.InputFileLocationClass
		kind mediaKind
	}
	var items []withMedia
	for _, m := range messages {
		if m.Media == nil {
			continue
		}
		if loc, kind, ok := extractDownloadable(m.Media); ok {
			items = append(items, withMedia{loc, kind})
		}
	}
	if len(items) == 0 {
		return false
	}

	if len(items) == 1 {
		data, err := downloadMediaBytes(ctx, api, items[0].loc)
		if err != nil || len(data) == 0 {
			return false
		}
		_, err = h.sendDownloadedMedia(ctx, chatID, items[0].kind, data, config.BotTag)
		return err == nil
	}

	media := make([]models.InputMedia, 0, len(items))
	for i, it := range items {
		data, err := downloadMediaBytes(ctx, api, it.loc)
		if err != nil || len(data) == 0 {
			continue
		}
		caption := ""
		if i == 0 {
			caption = config.BotTag
		}
		media = append(media, inputMediaFor(it.kind, data, fmt.Sprintf("attach://post%d", i), caption))
	}
	if len(media) == 0 {
		return false
	}

	_, err := telegramapi.SafeSendMediaGroup(ctx, h.b, &bot.SendMediaGroupParams{ChatID: chatID, Media: media, DisableNotification: true})
	return err == nil
}

func (h *Handler) genericPostFailure(ctx context.Context, chatID int64, loading *loadingHandle, sourceURL string, username *string, err error) bool {
	loading.delete(ctx)
	if isNoAccessError(err) {
		h.sendText(ctx, chatID, fmt.Sprintf("Нет доступа к каналу. Возможно, канал приватный или бот не является участником.\n%s", config.BotTag))
		h.st.RecordDownloadLogged(chatID, sourceURL, "telegram", "post", false, username, nil)
		return false
	}
	h.sendText(ctx, chatID, fmt.Sprintf("Ошибка при загрузке поста. Попробуйте позже.\n%s", config.BotTag))
	telegramapi.SendErrorToAdmin(ctx, h.b, err, "telegram post download", "", &chatID, username)
	h.st.RecordDownloadLogged(chatID, sourceURL, "telegram", "post", false, username, nil)
	return false
}

func (h *Handler) DownloadTelegramPost(ctx context.Context, chatID int64, username string, postID int) bool {
	sourceURL := fmt.Sprintf("t.me/%s/%d", username, postID)
	loading := h.startLoading(ctx, chatID, "Загружаю пост...")

	peer, err := h.client.Peers().Resolve(ctx, username)
	if err != nil {
		return h.genericPostFailure(ctx, chatID, loading, sourceURL, &username, err)
	}
	channel, ok := peer.(peers.Channel)
	if !ok {
		return h.genericPostFailure(ctx, chatID, loading, sourceURL, &username, fmt.Errorf("@%s is not a channel", username))
	}

	messages, err := getPostMessages(ctx, h.client.API(), channel.InputChannel(), peer.InputPeer(), postID)
	if err != nil {
		return h.genericPostFailure(ctx, chatID, loading, sourceURL, &username, err)
	}
	if len(messages) == 0 {
		loading.delete(ctx)
		h.sendText(ctx, chatID, fmt.Sprintf("Пост не найден у @%s.\n%s", username, config.BotTag))
		h.st.RecordDownloadLogged(chatID, sourceURL, "telegram", "post", false, &username, nil)
		return false
	}

	sent := h.sendPostMessages(ctx, chatID, h.client.API(), messages)
	if !sent {
		h.sendText(ctx, chatID, fmt.Sprintf("Пост не содержит медиа.\n%s", config.BotTag))
	}
	loading.delete(ctx)
	h.st.RecordDownloadLogged(chatID, sourceURL, "telegram", "post", sent, &username, nil)
	return sent
}

func (h *Handler) DownloadPrivateTelegramPost(ctx context.Context, chatID int64, channelChatID int64, messageID int) bool {
	// channelChatID is the Bot-API-style "-100<id>" form from platform.ParseTelegramLink,
	// converted below to the bare MTProto id that ResolveChannelID expects
	sourceURL := fmt.Sprintf("t.me/c/%d/%d", -channelChatID, messageID)
	loading := h.startLoading(ctx, chatID, "Загружаю пост...")

	rawID, err := rawChannelID(channelChatID)
	if err != nil {
		return h.genericPostFailure(ctx, chatID, loading, sourceURL, nil, err)
	}

	channel, err := h.client.Peers().ResolveChannelID(ctx, rawID)
	if err != nil {
		return h.genericPostFailure(ctx, chatID, loading, sourceURL, nil, err)
	}

	messages, err := getPostMessages(ctx, h.client.API(), channel.InputChannel(), channel.InputPeer(), messageID)
	if err != nil {
		return h.genericPostFailure(ctx, chatID, loading, sourceURL, nil, err)
	}
	if len(messages) == 0 {
		loading.delete(ctx)
		h.sendText(ctx, chatID, fmt.Sprintf("Пост не найден.\n%s", config.BotTag))
		h.st.RecordDownloadLogged(chatID, sourceURL, "telegram", "post", false, nil, nil)
		return false
	}

	sent := h.sendPostMessages(ctx, chatID, h.client.API(), messages)
	if !sent {
		h.sendText(ctx, chatID, fmt.Sprintf("Пост не содержит медиа.\n%s", config.BotTag))
	}
	loading.delete(ctx)
	h.st.RecordDownloadLogged(chatID, sourceURL, "telegram", "post", sent, nil, nil)
	return sent
}

func rawChannelID(chatStyleID int64) (int64, error) {
	// inverts "-100"+digits string concatenation — NOT arithmetic, don't "simplify"
	s := strconv.FormatInt(-chatStyleID, 10)
	if !strings.HasPrefix(s, "100") || len(s) <= 3 {
		return 0, fmt.Errorf("not a channel-style id: %d", chatStyleID)
	}
	return strconv.ParseInt(s[3:], 10, 64)
}
