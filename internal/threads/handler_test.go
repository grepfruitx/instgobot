package threads

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func rawMessages(t *testing.T, values ...any) []json.RawMessage {
	t.Helper()
	out := make([]json.RawMessage, len(values))
	for i, v := range values {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		out[i] = b
	}
	return out
}

func TestNormalizeVideoURLsStringArray(t *testing.T) {
	raw := rawMessages(t, "https://a.mp4", "https://b.mp4")
	got := normalizeVideoURLs(raw)
	if len(got) != 2 || got[0] != "https://a.mp4" || got[1] != "https://b.mp4" {
		t.Fatalf("unexpected result: %v", got)
	}
}

func TestNormalizeVideoURLsObjectArray(t *testing.T) {
	raw := rawMessages(t,
		map[string]string{"download_url": "https://a.mp4"},
		map[string]string{"download_url": "https://b.mp4"},
	)
	got := normalizeVideoURLs(raw)
	if len(got) != 2 || got[0] != "https://a.mp4" || got[1] != "https://b.mp4" {
		t.Fatalf("unexpected result: %v", got)
	}
}

func TestGetDownloadLinks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"image_urls":["https://img.jpg"],"video_urls":["https://vid.mp4"]}`))
	}))
	defer srv.Close()

	orig := apiBaseURL
	apiBaseURL = srv.URL
	defer func() { apiBaseURL = orig }()

	photos, videos, err := getDownloadLinks(t.Context(), "https://www.threads.com/@user/post/abc")
	if err != nil {
		t.Fatalf("getDownloadLinks: %v", err)
	}
	if len(photos) != 1 || photos[0] != "https://img.jpg" {
		t.Fatalf("unexpected photos: %v", photos)
	}
	if len(videos) != 1 || videos[0] != "https://vid.mp4" {
		t.Fatalf("unexpected videos: %v", videos)
	}
}

func TestGetDownloadLinksHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	orig := apiBaseURL
	apiBaseURL = srv.URL
	defer func() { apiBaseURL = orig }()

	if _, _, err := getDownloadLinks(t.Context(), "https://www.threads.com/@user/post/abc"); err == nil {
		t.Fatal("expected error for 500 response")
	}
}
