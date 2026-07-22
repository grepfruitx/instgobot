package media

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
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

func strp(s string) *string { return &s }

// fakeTelegramServer answers every Bot API method with a generic success
// envelope carrying a fixed file_id, so we can exercise the real send path
// (including multipart upload marshaling) without hitting real Telegram.
func fakeTelegramServer(t *testing.T, fileID string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/sendVideo"):
			fmt.Fprintf(w, `{"ok":true,"result":{"message_id":1,"date":0,"chat":{"id":1,"type":"private"},"video":{"file_id":%q,"file_unique_id":"u","width":1,"height":1,"duration":1}}}`, fileID)
		case strings.HasSuffix(r.URL.Path, "/sendPhoto"):
			fmt.Fprintf(w, `{"ok":true,"result":{"message_id":1,"date":0,"chat":{"id":1,"type":"private"},"photo":[{"file_id":%q,"file_unique_id":"u","width":1,"height":1}]}}`, fileID)
		case strings.HasSuffix(r.URL.Path, "/sendMediaGroup"):
			fmt.Fprintf(w, `{"ok":true,"result":[{"message_id":1,"date":0,"chat":{"id":1,"type":"private"},"photo":[{"file_id":%q,"file_unique_id":"u","width":1,"height":1}]},{"message_id":2,"date":0,"chat":{"id":1,"type":"private"},"photo":[{"file_id":%q,"file_unique_id":"u2","width":1,"height":1}]}]}`, fileID, fileID+"-2")
		default:
			fmt.Fprint(w, `{"ok":true,"result":true}`)
		}
	}))
}

func newTestBot(t *testing.T, tgServer *httptest.Server) *bot.Bot {
	t.Helper()
	b, err := bot.New("test-token", bot.WithServerURL(tgServer.URL), bot.WithSkipGetMe())
	if err != nil {
		t.Fatalf("bot.New: %v", err)
	}
	return b
}

func fakeMediaServer(t *testing.T, body []byte) (*httptest.Server, *int32) {
	t.Helper()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
		w.Write(body)
	}))
	return srv, &hits
}

func TestProcessSingleVideoNoCache(t *testing.T) {
	st := newTestStore(t)
	tg := fakeTelegramServer(t, "file123")
	defer tg.Close()
	b := newTestBot(t, tg)

	media, hits := fakeMediaServer(t, []byte("fake video bytes"))
	defer media.Close()

	ctx := t.Context()
	postURL := "https://example.com/post"
	ok, err := ProcessSingleVideo(ctx, b, st, 1, media.URL, "test", strp("user"), &postURL)
	if err != nil {
		t.Fatalf("ProcessSingleVideo: %v", err)
	}
	if !ok {
		t.Fatal("expected success")
	}
	if atomic.LoadInt32(hits) != 1 {
		t.Fatalf("expected 1 media fetch, got %d", *hits)
	}

	fileID, cached, err := st.GetCachedFileID(postURL, "video", 0)
	if err != nil || !cached || fileID != "file123" {
		t.Fatalf("expected cached file123, got %q cached=%v err=%v", fileID, cached, err)
	}
}

func TestProcessSingleVideoUsesCache(t *testing.T) {
	st := newTestStore(t)
	tg := fakeTelegramServer(t, "file123")
	defer tg.Close()
	b := newTestBot(t, tg)

	media, hits := fakeMediaServer(t, []byte("fake video bytes"))
	defer media.Close()

	postURL := "https://example.com/post"
	if err := st.SetCachedFileID(postURL, "video", 0, "cached-file-id"); err != nil {
		t.Fatalf("SetCachedFileID: %v", err)
	}

	ok, err := ProcessSingleVideo(t.Context(), b, st, 1, media.URL, "test", nil, &postURL)
	if err != nil || !ok {
		t.Fatalf("expected success, got ok=%v err=%v", ok, err)
	}
	if atomic.LoadInt32(hits) != 0 {
		t.Fatalf("expected cache hit to skip media fetch, got %d hits", *hits)
	}
}

func TestProcessSingleMediaEmptyURL(t *testing.T) {
	st := newTestStore(t)
	tg := fakeTelegramServer(t, "file123")
	defer tg.Close()
	b := newTestBot(t, tg)

	ok, err := ProcessSingleVideo(t.Context(), b, st, 1, "", "test", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected false for empty url")
	}
}

func TestProcessMediaGroupCachesAllIndices(t *testing.T) {
	st := newTestStore(t)
	tg := fakeTelegramServer(t, "file123")
	defer tg.Close()
	b := newTestBot(t, tg)

	media, _ := fakeMediaServer(t, []byte("small photo bytes"))
	defer media.Close()

	postURL := "https://example.com/post"
	ok, err := ProcessMediaGroup(t.Context(), b, st, 1, []string{media.URL, media.URL}, KindPhoto, "test", nil, &postURL)
	if err != nil || !ok {
		t.Fatalf("expected success, got ok=%v err=%v", ok, err)
	}

	fileID0, cached0, _ := st.GetCachedFileID(postURL, "photo", 0)
	fileID1, cached1, _ := st.GetCachedFileID(postURL, "photo", 1)
	if !cached0 || !cached1 {
		t.Fatalf("expected both indices cached, got 0=%v(%q) 1=%v(%q)", cached0, fileID0, cached1, fileID1)
	}
}

func TestProcessMediaGroupUsesFullCache(t *testing.T) {
	st := newTestStore(t)
	tg := fakeTelegramServer(t, "file123")
	defer tg.Close()
	b := newTestBot(t, tg)

	media, hits := fakeMediaServer(t, []byte("small photo bytes"))
	defer media.Close()

	postURL := "https://example.com/post"
	st.SetCachedFileID(postURL, "photo", 0, "cached-0")
	st.SetCachedFileID(postURL, "photo", 1, "cached-1")

	ok, err := ProcessMediaGroup(t.Context(), b, st, 1, []string{media.URL, media.URL}, KindPhoto, "test", nil, &postURL)
	if err != nil || !ok {
		t.Fatalf("expected success, got ok=%v err=%v", ok, err)
	}
	if atomic.LoadInt32(hits) != 0 {
		t.Fatalf("expected no media fetches when fully cached, got %d", *hits)
	}
}

func TestFetchMediaResponseRejectsOversized(t *testing.T) {
	big := make([]byte, 0)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", MaxFileSize+1))
		w.Write(big)
	}))
	defer srv.Close()

	_, err := FetchMediaResponse(t.Context(), srv.URL, false)
	if err == nil {
		t.Fatal("expected error for oversized content")
	}
}

func TestFetchMediaResponseNonOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := FetchMediaResponse(t.Context(), srv.URL, false)
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

func TestFetchMediaResponseOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	resp, err := FetchMediaResponse(t.Context(), srv.URL, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
}
