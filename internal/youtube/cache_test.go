package youtube

import (
	"testing"

	"github.com/grepfruitx/instgobot/internal/cache"
)

func TestPendingURLTakeIsOneShot(t *testing.T) {
	c := cache.New(t.Context())

	if _, ok := takePendingURL(c, 1); ok {
		t.Fatal("expected miss")
	}

	setPendingURL(c, 1, "https://youtube.com/watch?v=x")

	url, ok := takePendingURL(c, 1)
	if !ok || url != "https://youtube.com/watch?v=x" {
		t.Fatalf("unexpected result: url=%q ok=%v", url, ok)
	}
	if _, ok := takePendingURL(c, 1); ok {
		t.Fatal("expected miss after take")
	}
}

func TestGetYtMetaServesFromCache(t *testing.T) {
	c := cache.New(t.Context())
	meta := &YtMeta{Title: "Test video", Duration: 42}
	c.Set(metaKey("https://youtube.com/watch?v=x"), meta, metaTTL)

	got, err := getYtMeta(t.Context(), c, "/nonexistent/yt-dlp", "https://youtube.com/watch?v=x")
	if err != nil || got != meta {
		t.Fatalf("expected cached meta, got %+v err=%v", got, err)
	}
}

func TestParseCallbackData(t *testing.T) {
	cases := []struct {
		data    string
		wantOK  bool
		chatID  int64
		kind    string
		quality int
	}{
		{"yt:123:v:720", true, 123, "v", 720},
		{"yt:456:a:0", true, 456, "a", 0},
		{"garbage", false, 0, "", 0},
		{"yt:123:x:720", false, 0, "", 0},
	}
	for _, c := range cases {
		chatID, kind, quality, ok := ParseCallbackData(c.data)
		if ok != c.wantOK {
			t.Errorf("ParseCallbackData(%q) ok = %v, want %v", c.data, ok, c.wantOK)
			continue
		}
		if !ok {
			continue
		}
		if chatID != c.chatID || kind != c.kind || quality != c.quality {
			t.Errorf("ParseCallbackData(%q) = (%d,%q,%d), want (%d,%q,%d)", c.data, chatID, kind, quality, c.chatID, c.kind, c.quality)
		}
	}
}
