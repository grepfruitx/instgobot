package userbot

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/peers"
	"github.com/gotd/td/tg"

	"github.com/grepfruitx/instgobot/internal/config"
)

type Client struct {
	tg        *telegram.Client
	api       *tg.Client
	peers     *peers.Manager
	connected atomic.Bool
}

func NewClient(cfg *config.Config) *Client {
	tgClient := telegram.NewClient(cfg.TelegramAPIID, cfg.TelegramAPIHash, telegram.Options{
		SessionStorage: &session.FileStorage{Path: cfg.UserbotSessionPath},
	})
	api := tgClient.API()
	return &Client{
		tg:    tgClient,
		api:   api,
		peers: peers.Options{}.Build(api),
	}
}

func (c *Client) Run(ctx context.Context, ready func()) error {
	return c.tg.Run(ctx, func(ctx context.Context) error {
		status, err := c.tg.Auth().Status(ctx)
		if err != nil {
			return fmt.Errorf("check auth status: %w", err)
		}
		if !status.Authorized {
			return fmt.Errorf("userbot session not authorized — run the login flow first (see cmd/userbot-login)")
		}

		c.connected.Store(true)
		defer c.connected.Store(false)

		if ready != nil {
			ready()
		}

		<-ctx.Done()
		return ctx.Err()
	})
}

func (c *Client) API() *tg.Client       { return c.api }
func (c *Client) Peers() *peers.Manager { return c.peers }
func (c *Client) Connected() bool       { return c.connected.Load() }

func (c *Client) SyncDialogs(ctx context.Context) error {
	dialogs, err := c.api.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
		OffsetPeer: &tg.InputPeerEmpty{},
		Limit:      200,
	})
	if err != nil {
		return fmt.Errorf("get dialogs: %w", err)
	}

	var users []tg.UserClass
	var chats []tg.ChatClass
	switch d := dialogs.(type) {
	case *tg.MessagesDialogs:
		users, chats = d.Users, d.Chats
	case *tg.MessagesDialogsSlice:
		users, chats = d.Users, d.Chats
	}

	return c.peers.Apply(ctx, users, chats)
}
