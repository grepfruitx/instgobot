package threads

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/go-telegram/bot"

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

type captured struct {
	mu       sync.Mutex
	messages []string
	captions []string
}

func (c *captured) addMessage(s string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = append(c.messages, s)
}

func (c *captured) addCaption(s string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.captions = append(c.captions, s)
}

func (c *captured) snapshot() (messages, captions []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.messages...), append([]string(nil), c.captions...)
}

func fakeTelegram(t *testing.T) (*bot.Bot, *captured) {
	t.Helper()
	cap := &captured{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/sendPhoto"):
			cap.addCaption(r.FormValue("caption"))
			fmt.Fprint(w, `{"ok":true,"result":{"message_id":1,"date":0,"chat":{"id":1,"type":"private"},"photo":[{"file_id":"cached-photo","file_unique_id":"u","width":1,"height":1}]}}`)
		case strings.HasSuffix(r.URL.Path, "/sendMessage"):
			cap.addMessage(r.FormValue("text"))
			fmt.Fprint(w, `{"ok":true,"result":{"message_id":1,"date":0,"chat":{"id":1,"type":"private"},"text":"ok"}}`)
		default:
			fmt.Fprint(w, `{"ok":true,"result":true}`)
		}
	}))
	t.Cleanup(srv.Close)

	b, err := bot.New("test-token", bot.WithServerURL(srv.URL), bot.WithSkipGetMe())
	if err != nil {
		t.Fatalf("bot.New: %v", err)
	}
	return b, cap
}

func TestProcessRepeatLinkCostsNoUpstreamCalls(t *testing.T) {
	var cdnHits, apiHits int32

	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&cdnHits, 1)
		body := []byte("fake-jpeg-bytes")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
		w.Write(body)
	}))
	t.Cleanup(cdn.Close)

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&apiHits, 1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"success":true,"type":"image","author":"a","postText":"post body","media":[{"type":"image","url":%q}]}`, cdn.URL+"/pic.jpg")
	}))
	t.Cleanup(api.Close)

	orig := apiBaseURL
	apiBaseURL = api.URL
	t.Cleanup(func() { apiBaseURL = orig })

	st := newTestStore(t)
	b, cap := fakeTelegram(t)
	const link = "https://www.threads.com/@a/post/ABC?igsi=xyz"

	Process(t.Context(), b, st, 1, link, nil, nil)
	if got := atomic.LoadInt32(&apiHits); got != 1 {
		t.Fatalf("first request should hit the API once, got %d", got)
	}
	if got := atomic.LoadInt32(&cdnHits); got != 1 {
		t.Fatalf("first request should download from the CDN once, got %d", got)
	}

	Process(t.Context(), b, st, 1, link, nil, nil)
	if got := atomic.LoadInt32(&apiHits); got != 1 {
		t.Fatalf("repeat request must not call the API again, hits went to %d", got)
	}
	if got := atomic.LoadInt32(&cdnHits); got != 1 {
		t.Fatalf("repeat request must not re-download media, hits went to %d", got)
	}

	messages, captions := cap.snapshot()
	if len(messages) != 0 {
		t.Fatalf("a short post text belongs in the caption, not a separate message, got %q", messages)
	}
	if len(captions) != 2 {
		t.Fatalf("expected one captioned photo per request, got %q", captions)
	}
	for _, c := range captions {
		if !strings.HasPrefix(c, "post body") {
			t.Fatalf("expected the post text to lead the caption, got %q", c)
		}
	}
}

func TestProcessLongTextFallsBackToSeparateMessage(t *testing.T) {
	longText := strings.Repeat("длинный текст поста ", 120)

	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := []byte("fake-jpeg-bytes")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
		w.Write(body)
	}))
	t.Cleanup(cdn.Close)

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"success":true,"type":"image","postText":%q,"media":[{"type":"image","url":%q}]}`, longText, cdn.URL+"/pic.jpg")
	}))
	t.Cleanup(api.Close)

	orig := apiBaseURL
	apiBaseURL = api.URL
	t.Cleanup(func() { apiBaseURL = orig })

	st := newTestStore(t)
	b, cap := fakeTelegram(t)

	Process(t.Context(), b, st, 1, "https://www.threads.com/@a/post/LONG", nil, nil)

	messages, captions := cap.snapshot()
	if len(messages) == 0 {
		t.Fatal("an oversized post text must be sent as its own message")
	}
	for _, c := range captions {
		if strings.Contains(c, "длинный текст поста") {
			t.Fatalf("oversized text must not be crammed into the caption, got %q", c)
		}
	}
}

func TestProcessCarouselIndexVariantHitsSameCache(t *testing.T) {
	var apiHits int32

	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := []byte("fake-jpeg-bytes")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
		w.Write(body)
	}))
	t.Cleanup(cdn.Close)

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&apiHits, 1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"success":true,"type":"image","media":[{"type":"image","url":%q}]}`, cdn.URL+"/pic.jpg")
	}))
	t.Cleanup(api.Close)

	orig := apiBaseURL
	apiBaseURL = api.URL
	t.Cleanup(func() { apiBaseURL = orig })

	st := newTestStore(t)
	b, _ := fakeTelegram(t)

	Process(t.Context(), b, st, 1, "https://www.threads.com/@a/post/ABC?img_index=2", nil, nil)
	Process(t.Context(), b, st, 1, "https://www.threads.com/@a/post/ABC?img_index=5", nil, nil)

	if got := atomic.LoadInt32(&apiHits); got != 1 {
		t.Fatalf("index variants are one post — expected 1 API call total, got %d", got)
	}
}

func TestProcessTextOnlyPostIsCached(t *testing.T) {
	var apiHits int32
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&apiHits, 1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"success":true,"type":"image","postText":"just words","media":[]}`)
	}))
	t.Cleanup(api.Close)

	orig := apiBaseURL
	apiBaseURL = api.URL
	t.Cleanup(func() { apiBaseURL = orig })

	st := newTestStore(t)
	b, cap := fakeTelegram(t)
	const link = "https://www.threads.com/@a/post/TEXT"

	Process(t.Context(), b, st, 1, link, nil, nil)
	Process(t.Context(), b, st, 1, link, nil, nil)

	if got := atomic.LoadInt32(&apiHits); got != 1 {
		t.Fatalf("a cached text-only post must not re-hit the API, got %d calls", got)
	}
	messages, _ := cap.snapshot()
	if len(messages) != 2 {
		t.Fatalf("with no media the text has to be its own message both times, got %q", messages)
	}
}
