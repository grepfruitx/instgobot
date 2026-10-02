package admin

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/go-telegram/bot"

	"github.com/grepfruitx/instgobot/internal/config"
	"github.com/grepfruitx/instgobot/internal/store"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

type sentMessage struct {
	ChatID int64
	Text   string
}

type syncSentMessages struct {
	mu   sync.Mutex
	msgs []sentMessage
}

func (s *syncSentMessages) add(m sentMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.msgs = append(s.msgs, m)
}

func (s *syncSentMessages) all() []sentMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]sentMessage(nil), s.msgs...)
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

func newTestBotCapturingMessagesConcurrent(t *testing.T) (*bot.Bot, *syncSentMessages) {
	t.Helper()
	sent := &syncSentMessages{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/sendMessage") {
			var chatID int64
			fmt.Sscanf(r.FormValue("chat_id"), "%d", &chatID)
			sent.add(sentMessage{ChatID: chatID, Text: r.FormValue("text")})
		}
		fmt.Fprint(w, `{"ok":true,"result":{"message_id":1,"date":0,"chat":{"id":1,"type":"private"},"text":"ok"}}`)
	}))
	t.Cleanup(srv.Close)

	b, err := bot.New("test-token", bot.WithServerURL(srv.URL), bot.WithSkipGetMe())
	if err != nil {
		t.Fatalf("bot.New: %v", err)
	}
	return b, sent
}

func TestHandleCommandRejectsNonAdmin(t *testing.T) {
	st := newTestStore(t)
	b, sent := newTestBotCapturingMessages(t)
	h := New(b, st, nil)

	nonAdminID := int64(1)
	if handled := h.HandleCommand(t.Context(), 1, "/stats", nonAdminID); handled {
		t.Fatal("expected non-admin command to be unhandled")
	}
	if len(*sent) != 0 {
		t.Fatalf("expected no messages sent, got %v", *sent)
	}
}

func TestHandleCommandStats(t *testing.T) {
	st := newTestStore(t)
	b, sent := newTestBotCapturingMessages(t)
	h := New(b, st, nil)

	adminID := config.AdminUserIDs[0]
	if handled := h.HandleCommand(t.Context(), 1, "/stats", adminID); !handled {
		t.Fatal("expected /stats to be handled")
	}
	if len(*sent) != 1 || !strings.Contains((*sent)[0].Text, "Общая статистика бота") {
		t.Fatalf("unexpected sent messages: %v", *sent)
	}
}

func TestHandleCommandUsersEmpty(t *testing.T) {
	st := newTestStore(t)
	b, sent := newTestBotCapturingMessages(t)
	h := New(b, st, nil)

	adminID := config.AdminUserIDs[0]
	h.HandleCommand(t.Context(), 1, "/users", adminID)
	if len(*sent) != 1 || (*sent)[0].Text != "Пользователей пока нет" {
		t.Fatalf("unexpected sent messages: %v", *sent)
	}
}

func TestPlatformToggleAndStatus(t *testing.T) {
	st := newTestStore(t)
	b, sent := newTestBotCapturingMessages(t)
	h := New(b, st, nil)
	adminID := config.AdminUserIDs[0]

	h.HandleCommand(t.Context(), 1, "/poff instagram", adminID)
	if len(*sent) != 1 || !strings.Contains((*sent)[0].Text, "instagram выключена") {
		t.Fatalf("unexpected /poff response: %v", *sent)
	}
	*sent = nil

	h.HandleCommand(t.Context(), 1, "/pstatus", adminID)
	if len(*sent) != 1 || !strings.Contains((*sent)[0].Text, "instagram — выключена") {
		t.Fatalf("unexpected /pstatus response: %v", *sent)
	}
	*sent = nil

	h.HandleCommand(t.Context(), 1, "/pon instagram", adminID)
	if len(*sent) != 1 || !strings.Contains((*sent)[0].Text, "instagram включена") {
		t.Fatalf("unexpected /pon response: %v", *sent)
	}
}

func TestPlatformToggleRejectsUnknownPlatform(t *testing.T) {
	st := newTestStore(t)
	b, sent := newTestBotCapturingMessages(t)
	h := New(b, st, nil)
	adminID := config.AdminUserIDs[0]

	h.HandleCommand(t.Context(), 1, "/poff nonsense", adminID)
	if len(*sent) != 1 || !strings.Contains((*sent)[0].Text, "Укажите платформу") {
		t.Fatalf("unexpected response: %v", *sent)
	}
}

func TestHandleAnnounceNoUsers(t *testing.T) {
	st := newTestStore(t)
	b, sent := newTestBotCapturingMessages(t)
	h := New(b, st, nil)
	adminID := config.AdminUserIDs[0]

	h.HandleCommand(t.Context(), 1, "/announce hello world", adminID)
	if len(*sent) != 1 || (*sent)[0].Text != "В базе данных нет пользователей для отправки объявления." {
		t.Fatalf("unexpected response: %v", *sent)
	}
}

func TestHandleAnnounceEmptyText(t *testing.T) {
	st := newTestStore(t)
	b, sent := newTestBotCapturingMessages(t)
	h := New(b, st, nil)
	adminID := config.AdminUserIDs[0]

	h.HandleCommand(t.Context(), 1, "/announce", adminID)
	if len(*sent) != 1 || !strings.Contains((*sent)[0].Text, "добавьте текст объявления") {
		t.Fatalf("unexpected response: %v", *sent)
	}
}

func TestUnknownCommandNotHandled(t *testing.T) {
	st := newTestStore(t)
	b, _ := newTestBotCapturingMessages(t)
	h := New(b, st, nil)
	adminID := config.AdminUserIDs[0]

	if handled := h.HandleCommand(t.Context(), 1, "/notacommand", adminID); handled {
		t.Fatal("expected unknown command to be unhandled")
	}
}
