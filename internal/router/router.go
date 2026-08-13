package router

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/grepfruitx/instgobot/internal/admin"
	"github.com/grepfruitx/instgobot/internal/ratelimit"
	"github.com/grepfruitx/instgobot/internal/store"
	"github.com/grepfruitx/instgobot/internal/userbot"
	"github.com/grepfruitx/instgobot/internal/youtube"
)

type Router struct {
	st            *store.Store
	userHandler   *userbot.Handler
	ytHandler     *youtube.Handler
	adminHandler  *admin.Handler
	limiter       *ratelimit.Limiter
	adminUsername string
}

func New(st *store.Store, userHandler *userbot.Handler, ytHandler *youtube.Handler, adminHandler *admin.Handler, limiter *ratelimit.Limiter, adminUsername string) *Router {
	return &Router{
		st: st, userHandler: userHandler, ytHandler: ytHandler,
		adminHandler: adminHandler, limiter: limiter, adminUsername: adminUsername,
	}
}

func (r *Router) Handle(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message != nil {
		r.handleMessage(ctx, b, update.Message)
	}
	if update.CallbackQuery != nil {
		r.handleCallback(ctx, b, update.CallbackQuery)
	}
}

func (r *Router) handleCallback(ctx context.Context, b *bot.Bot, cq *models.CallbackQuery) {
	_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{CallbackQueryID: cq.ID})

	var chatID int64
	var messageID int
	switch cq.Message.Type {
	case models.MaybeInaccessibleMessageTypeMessage:
		if cq.Message.Message == nil {
			return
		}
		chatID = cq.Message.Message.Chat.ID
		messageID = cq.Message.Message.ID
	case models.MaybeInaccessibleMessageTypeInaccessibleMessage:
		if cq.Message.InaccessibleMessage == nil {
			return
		}
		chatID = cq.Message.InaccessibleMessage.Chat.ID
	default:
		return
	}

	originChatID, kind, quality, ok := youtube.ParseCallbackData(cq.Data)
	if !ok || originChatID != chatID {
		return
	}

	var username *string
	if cq.From.Username != "" {
		username = &cq.From.Username
	}

	r.ytHandler.HandleCallback(ctx, chatID, kind, quality, cq.From.ID, messageID, username)
}
