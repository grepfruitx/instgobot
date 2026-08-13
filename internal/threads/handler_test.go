package threads

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

const profilePicURL = "https://scontent-bom5-1.cdninstagram.com/v/t51.82787-19/765694086_18076529336452977_7917588713912414658_n.jpg?stp=dst-jpg_s640x640_tt6&efg=eyJ2ZW5jb2RlX3RhZyI6InByb2ZpbGVfcGljLmRqYW5nby45NDQuYzIifQ&_nc_ht=scontent-bom5-1.cdninstagram.com"

const realPhotoURL = "https://scontent-bom5-2.cdninstagram.com/v/t51.82787-15/751784491_17978089551103484_304340551267970841_n.jpg?stp=dst-jpg_e35_tt6&efg=eyJ2ZW5jb2RlX3RhZyI6IkNBUk9VU0VMX0lURU0ueHBpZHMuNDAzMi5zZHIucmVndWxhcl9waG90by5DMyJ9"

const realVideoURL = "https://scontent-bom5-2.cdninstagram.com/o1/v/t16/f2/m84/AQMOBUjxxfh7uRj7G7La.mp4?efg=eyJ2ZW5jb2RlX3RhZyI6Inhwdl9wcm9ncmVzc2l2ZS5JTlNUQUdSQU0uQ0FST1VTRUxfSVRFTS5DMy43MjAuZGFzaF9iYXNlbGluZV8xX3YxIn0%3D"

func newAPIServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected application/json content type, got %q", ct)
		}
		raw, _ := io.ReadAll(r.Body)
		var payload map[string]string
		if err := json.Unmarshal(raw, &payload); err != nil {
			t.Errorf("request body is not json: %v", err)
		} else if payload["url"] == "" {
			t.Errorf("expected a url field in the request body, got %s", raw)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	orig := apiBaseURL
	apiBaseURL = srv.URL
	t.Cleanup(func() { apiBaseURL = orig })
	return srv
}

func TestGetPostContentCarousel(t *testing.T) {
	newAPIServer(t, http.StatusOK, `{
		"success": true, "type": "carousel", "author": "lanyuju",
		"postText": "  收到我的屎蛋大便狗了  ",
		"media": [
			{"type": "video", "url": "`+realVideoURL+`", "index": 1},
			{"type": "image", "url": "`+realPhotoURL+`", "index": 2}
		]
	}`)

	content, err := getPostContent(t.Context(), "https://www.threads.com/@lanyuju/post/Db8d1muFBaU")
	if err != nil {
		t.Fatalf("getPostContent: %v", err)
	}
	if len(content.Photos) != 1 || content.Photos[0] != realPhotoURL {
		t.Fatalf("unexpected photos: %v", content.Photos)
	}
	if len(content.Videos) != 1 || content.Videos[0] != realVideoURL {
		t.Fatalf("unexpected videos: %v", content.Videos)
	}
	if content.Text != "收到我的屎蛋大便狗了" {
		t.Fatalf("expected trimmed post text, got %q", content.Text)
	}
}

func TestGetPostContentDropsProfilePictureKeepsText(t *testing.T) {
	newAPIServer(t, http.StatusOK, `{
		"success": true, "type": "image", "author": "travelisgooood",
		"postText": "釜山機場接送",
		"media": [{"type": "image", "url": "`+profilePicURL+`"}]
	}`)

	content, err := getPostContent(t.Context(), "https://www.threads.com/@travelisgooood/post/Db8JdhQAXEl")
	if err != nil {
		t.Fatalf("a text-only post is content, not an error: %v", err)
	}
	if content.hasMedia() {
		t.Fatalf("expected the avatar to be dropped, got photos=%v videos=%v", content.Photos, content.Videos)
	}
	if content.Text != "釜山機場接送" {
		t.Fatalf("expected the post text to survive, got %q", content.Text)
	}
}

func TestGetPostContentEmptyPost(t *testing.T) {
	newAPIServer(t, http.StatusOK, `{"success": true, "type": "image", "media": []}`)

	content, err := getPostContent(t.Context(), "https://www.threads.com/@user/post/abc")
	if err != nil {
		t.Fatalf("getPostContent: %v", err)
	}
	if content.hasMedia() || content.Text != "" {
		t.Fatalf("expected wholly empty content, got %+v", content)
	}
}

func TestGetPostContentAPIRefusals(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantCode string
	}{
		{"text only", `{"success":false,"error":"This post is text-only.","code":"NO_MEDIA"}`, codeNoMedia},
		{"invalid url", `{"success":false,"error":"Please enter a valid Threads post URL.","code":"INVALID_URL"}`, codeInvalidURL},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			newAPIServer(t, http.StatusBadRequest, tc.body)

			_, err := getPostContent(t.Context(), "https://www.threads.com/@user/post/abc")

			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("expected an APIError, got %v", err)
			}
			if apiErr.Code != tc.wantCode {
				t.Fatalf("expected code %s, got %s", tc.wantCode, apiErr.Code)
			}
			if apiErr.UserFacing() == "" {
				t.Fatalf("expected %s to be explained to the user, not reported to the admin", tc.wantCode)
			}
		})
	}
}

func TestGetPostContentUnknownFailureIsNotUserFacing(t *testing.T) {
	newAPIServer(t, http.StatusInternalServerError, `{"success":false,"error":"boom","code":"WHO_KNOWS"}`)

	_, err := getPostContent(t.Context(), "https://www.threads.com/@user/post/abc")

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected an APIError, got %v", err)
	}
	if apiErr.UserFacing() != "" {
		t.Fatal("an unrecognized code should stay an admin-reported error, not a user-facing one")
	}
}

func TestGetPostContentNonJSONError(t *testing.T) {
	newAPIServer(t, http.StatusBadGateway, `<html>nginx</html>`)

	if _, err := getPostContent(t.Context(), "https://www.threads.com/@user/post/abc"); err == nil {
		t.Fatal("expected an error for a non-json 502 response")
	}
}

func TestIsProfilePicture(t *testing.T) {
	cases := []struct {
		name string
		url  string
		want bool
	}{
		{"avatar via efg tag", profilePicURL, true},
		{"avatar via path only", "https://cdn.example.com/v/t51.12345-19/pic.jpg", true},
		{"real carousel photo", realPhotoURL, false},
		{"real video", realVideoURL, false},
		{"no query at all", "https://cdn.example.com/v/t51.82787-15/pic.jpg", false},
		{"unparseable", "://nope", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isProfilePicture(tc.url); got != tc.want {
				t.Fatalf("isProfilePicture(%q) = %v, want %v", tc.url, got, tc.want)
			}
		})
	}
}

func TestSplitMediaSkipsEmptyURLs(t *testing.T) {
	photos, videos := splitMedia([]apiMedia{
		{Type: "image", URL: ""},
		{Type: "image", URL: realPhotoURL},
		{Type: "video", URL: ""},
		{Type: "audio", URL: "https://cdn.example.com/v/t51.82787-15/song.m4a"},
	})
	if len(photos) != 1 || photos[0] != realPhotoURL {
		t.Fatalf("unexpected photos: %v", photos)
	}
	if len(videos) != 0 {
		t.Fatalf("expected no videos, got %v", videos)
	}
}
