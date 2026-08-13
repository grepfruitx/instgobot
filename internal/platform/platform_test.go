package platform

import "testing"

func TestDetectPlatform(t *testing.T) {
	cases := map[string]string{
		"https://www.tiktok.com/@user/video/123": "tiktok",
		"https://www.instagram.com/reel/abc/":    "instagram",
		"https://www.facebook.com/watch?v=1":     "facebook",
		"https://fb.com/watch?v=1":               "facebook",
		"https://twitter.com/user/status/1":      "twitter",
		"https://x.com/user/status/1":            "twitter",
		"https://www.youtube.com/shorts/abc":     "youtube",
		"https://youtu.be/abc":                   "youtube",
		"https://www.threads.com/@user/post/abc": "threads",
		"@someuser":                              "telegram",
		"https://t.me/someuser":                  "telegram",
		"https://telegram.me/someuser/5":         "telegram",
		"https://example.com/nothing":            "unknown",
	}
	for url, want := range cases {
		if got := DetectPlatform(url); got != want {
			t.Errorf("DetectPlatform(%q) = %q, want %q", url, got, want)
		}
	}
}

func TestParseTelegramLinkPrivatePost(t *testing.T) {
	info, ok := ParseTelegramLink("https://t.me/c/1724666497/5083")
	if !ok {
		t.Fatal("expected match")
	}
	if info.Type != TelegramLinkPrivatePost {
		t.Fatalf("expected private_post, got %s", info.Type)
	}
	if info.ChannelID != -1001724666497 {
		t.Fatalf("expected channel id -1001724666497, got %d", info.ChannelID)
	}
	if info.MessageID != 5083 {
		t.Fatalf("expected message id 5083, got %d", info.MessageID)
	}
}

func TestParseTelegramLinkStory(t *testing.T) {
	info, ok := ParseTelegramLink("https://t.me/someuser/s/300")
	if !ok {
		t.Fatal("expected match")
	}
	if info.Type != TelegramLinkStory || info.Username != "someuser" || info.ID != 300 {
		t.Fatalf("unexpected result: %+v", info)
	}
}

func TestParseTelegramLinkPost(t *testing.T) {
	info, ok := ParseTelegramLink("https://t.me/somechannel/300")
	if !ok {
		t.Fatal("expected match")
	}
	if info.Type != TelegramLinkPost || info.Username != "somechannel" || info.ID != 300 {
		t.Fatalf("unexpected result: %+v", info)
	}
}

func TestParseTelegramLinkStoriesAll(t *testing.T) {
	info, ok := ParseTelegramLink("https://t.me/someuser")
	if !ok {
		t.Fatal("expected match")
	}
	if info.Type != TelegramLinkStoriesAll || info.Username != "someuser" {
		t.Fatalf("unexpected result: %+v", info)
	}
}

func TestParseTelegramLinkNoMatch(t *testing.T) {
	if _, ok := ParseTelegramLink("https://example.com"); ok {
		t.Fatal("expected no match")
	}
}

func TestGetInstagramProfileUsername(t *testing.T) {
	username, ok := GetInstagramProfileUsername("https://www.instagram.com/someuser/")
	if !ok || username != "someuser" {
		t.Fatalf("expected someuser, got %q ok=%v", username, ok)
	}

	if _, ok := GetInstagramProfileUsername("https://www.instagram.com/reel/abc123/"); ok {
		t.Fatal("expected reserved path 'reel' to be rejected")
	}
	if _, ok := GetInstagramProfileUsername("https://www.instagram.com/p/abc123/"); ok {
		t.Fatal("expected reserved path 'p' to be rejected")
	}
}

func TestToInstagramStoriesLink(t *testing.T) {
	want := "https://www.instagram.com/stories/someuser/"
	if got := ToInstagramStoriesLink("someuser"); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestIsTelegramLink(t *testing.T) {
	if !IsTelegramLink("https://t.me/someuser") {
		t.Fatal("expected true")
	}
	if IsTelegramLink("https://example.com") {
		t.Fatal("expected false")
	}
}

func TestNormalizePostURL(t *testing.T) {
	cases := []struct{ in, want string }{
		// the case that motivated this: same post, different carousel index
		{"https://www.instagram.com/p/DbLVGJAk7WC/?img_index=2&igsi=MXBn", "https://www.instagram.com/p/DbLVGJAk7WC"},
		{"https://www.instagram.com/p/DbLVGJAk7WC/?img_index=3&igsi=MXBn", "https://www.instagram.com/p/DbLVGJAk7WC"},
		{"https://www.instagram.com/p/DbLVGJAk7WC/", "https://www.instagram.com/p/DbLVGJAk7WC"},
		{"https://www.instagram.com/p/DbLVGJAk7WC", "https://www.instagram.com/p/DbLVGJAk7WC"},
		{"https://www.threads.com/@a/post/XYZ/", "https://www.threads.com/@a/post/XYZ"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := NormalizePostURL(tc.in); got != tc.want {
			t.Fatalf("NormalizePostURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNormalizePostURLCollapsesCarouselIndexVariants(t *testing.T) {
	a := NormalizePostURL("https://www.instagram.com/p/ABC/?img_index=2&igsi=x")
	b := NormalizePostURL("https://www.instagram.com/p/ABC/?img_index=7&igsi=y")
	if a != b {
		t.Fatalf("carousel index variants must share one cache key, got %q vs %q", a, b)
	}
}
