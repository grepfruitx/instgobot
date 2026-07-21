package youtube

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	t.Cleanup(mr.Close)

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })
	return rdb
}

func TestPendingURLRoundTrip(t *testing.T) {
	rdb := newTestRedis(t)
	ctx := t.Context()

	if _, ok, err := getPendingURL(ctx, rdb, 1); err != nil || ok {
		t.Fatalf("expected miss, got ok=%v err=%v", ok, err)
	}

	if err := setPendingURL(ctx, rdb, 1, "https://youtube.com/watch?v=x"); err != nil {
		t.Fatalf("setPendingURL: %v", err)
	}

	url, ok, err := getPendingURL(ctx, rdb, 1)
	if err != nil || !ok || url != "https://youtube.com/watch?v=x" {
		t.Fatalf("unexpected result: url=%q ok=%v err=%v", url, ok, err)
	}

	if err := deletePendingURL(ctx, rdb, 1); err != nil {
		t.Fatalf("deletePendingURL: %v", err)
	}
	if _, ok, _ := getPendingURL(ctx, rdb, 1); ok {
		t.Fatal("expected miss after delete")
	}
}

func TestMetaCacheRoundTrip(t *testing.T) {
	rdb := newTestRedis(t)
	ctx := t.Context()

	if _, ok, err := getCachedMeta(ctx, rdb, "https://youtube.com/watch?v=x"); err != nil || ok {
		t.Fatalf("expected miss, got ok=%v err=%v", ok, err)
	}

	meta := &YtMeta{Title: "Test video", Duration: 42, ThumbnailURL: "https://thumb.jpg", Formats: []ytDlpFormat{{FormatID: "18"}}}
	if err := setCachedMeta(ctx, rdb, "https://youtube.com/watch?v=x", meta); err != nil {
		t.Fatalf("setCachedMeta: %v", err)
	}

	got, ok, err := getCachedMeta(ctx, rdb, "https://youtube.com/watch?v=x")
	if err != nil || !ok {
		t.Fatalf("unexpected result: ok=%v err=%v", ok, err)
	}
	if got.Title != "Test video" || got.Duration != 42 || len(got.Formats) != 1 {
		t.Fatalf("unexpected meta: %+v", got)
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
