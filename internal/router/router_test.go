package router

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/grepfruitx/instgobot/internal/admin"
	"github.com/grepfruitx/instgobot/internal/config"
	"github.com/grepfruitx/instgobot/internal/ratelimit"
	"github.com/grepfruitx/instgobot/internal/store"
	"github.com/grepfruitx/instgobot/internal/youtube"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

type sentMessage struct {
	ChatID int64
	Text   string
}

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func newTestBotCapturingMessages(t *testing.T) (*bot.Bot, *[]sentMessage) {
	t.Helper()
	var sent []sentMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/sendMessage") {
			var chatID int64
			fmt.Sscanf(r.FormValue("chat_id"), "%d", &chatID)
			sent = append(sent, sentMessage{ChatID: chatID, Text: r.FormValue("text")})
		}
		fmt.Fprint(w, `{"ok":true,"result":{"message_id":1,"date":0,"chat":{"id":1,"type":"private"},"text":"ok"}}`)
	}))
	t.Cleanup(srv.Close)

	b, err := bot.New("test-token", bot.WithServerURL(srv.URL), bot.WithSkipGetMe())
	if err != nil {
		t.Fatalf("bot.New: %v", err)
	}
	return b, &sent
}

func newTestRouter(t *testing.T, st *store.Store, b *bot.Bot) *Router {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })

	limiter := ratelimit.New(rdb)
	adminHandler := admin.New(b, st)
	ytHandler := youtube.New(b, st, rdb, limiter, &config.Config{YtDlpPath: "yt-dlp"})

	return New(st, nil, ytHandler, adminHandler, limiter, "@someadmin")
}

func textMessage(chatID int64, userID int64, text string) *models.Message {
	return &models.Message{
		Chat: models.Chat{ID: chatID},
		From: &models.User{ID: userID},
		Text: text,
	}
}

func TestTelegramUsernameRegex(t *testing.T) {
	cases := map[string]bool{
		"@someuser":  true,
		"@abcde":     true,
		"@abcd":      false,
		"@":          false,
		"someuser":   false,
		"@user name": false,
	}
	for text, want := range cases {
		if got := telegramUsernameRe.MatchString(text); got != want {
			t.Errorf("telegramUsernameRe.MatchString(%q) = %v, want %v", text, got, want)
		}
	}
}

func TestHandleMessageStart(t *testing.T) {
	st := newTestStore(t)
	b, sent := newTestBotCapturingMessages(t)
	r := newTestRouter(t, st, b)

	msg := textMessage(1, 1, "/start")
	r.handleMessage(t.Context(), b, msg)

	if len(*sent) != 1 || !strings.Contains((*sent)[0].Text, "Я бот для скачивания медиа") {
		t.Fatalf("unexpected response: %v", *sent)
	}
}

func TestHandleMessageHelp(t *testing.T) {
	st := newTestStore(t)
	b, sent := newTestBotCapturingMessages(t)
	r := newTestRouter(t, st, b)

	r.handleMessage(t.Context(), b, textMessage(1, 1, "/help"))
	if len(*sent) != 1 || !strings.Contains((*sent)[0].Text, "Поддерживаемые платформы") {
		t.Fatalf("unexpected response: %v", *sent)
	}
}

func TestHandleMessageUnrecognizedTextFallsBackToHelp(t *testing.T) {
	st := newTestStore(t)
	b, sent := newTestBotCapturingMessages(t)
	r := newTestRouter(t, st, b)

	r.handleMessage(t.Context(), b, textMessage(1, 1, "just some random text"))
	if len(*sent) != 1 || !strings.Contains((*sent)[0].Text, "Поддерживаемые платформы") {
		t.Fatalf("unexpected response: %v", *sent)
	}
}

func TestRejectIfPlatformDisabled(t *testing.T) {
	st := newTestStore(t)
	b, sent := newTestBotCapturingMessages(t)
	r := newTestRouter(t, st, b)

	if err := st.SetPlatformDisabled("instagram", true); err != nil {
		t.Fatalf("SetPlatformDisabled: %v", err)
	}

	rejected := r.rejectIfPlatformDisabled(t.Context(), b, 1, "https://www.instagram.com/reel/abc/", 999)
	if !rejected {
		t.Fatal("expected platform-disabled rejection")
	}
	if len(*sent) != 1 || !strings.Contains((*sent)[0].Text, "временно не работает") {
		t.Fatalf("unexpected response: %v", *sent)
	}
}

func TestRejectIfPlatformDisabledAdminBypass(t *testing.T) {
	st := newTestStore(t)
	b, sent := newTestBotCapturingMessages(t)
	r := newTestRouter(t, st, b)

	if err := st.SetPlatformDisabled("instagram", true); err != nil {
		t.Fatalf("SetPlatformDisabled: %v", err)
	}

	adminID := config.AdminUserIDs[0]
	rejected := r.rejectIfPlatformDisabled(t.Context(), b, 1, "https://www.instagram.com/reel/abc/", adminID)
	if rejected {
		t.Fatal("expected admin to bypass platform-disabled check")
	}
	if len(*sent) != 0 {
		t.Fatalf("expected no message sent, got %v", *sent)
	}
}

func TestHandleMessageAdminCommandDispatchesToAdminHandler(t *testing.T) {
	st := newTestStore(t)
	b, sent := newTestBotCapturingMessages(t)
	r := newTestRouter(t, st, b)

	adminID := config.AdminUserIDs[0]
	r.handleMessage(t.Context(), b, textMessage(1, adminID, "/stats"))

	if len(*sent) != 1 || !strings.Contains((*sent)[0].Text, "Общая статистика бота") {
		t.Fatalf("expected admin /stats response, got %v", *sent)
	}
}
