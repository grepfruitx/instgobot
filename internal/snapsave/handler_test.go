package snapsave

import "testing"

func TestHandleUnderlineEnding(t *testing.T) {
	if got := handleUnderlineEnding("https://instagram.com/someuser_"); got != "https://instagram.com/someuser_/" {
		t.Fatalf("got %q", got)
	}
	if got := handleUnderlineEnding("https://instagram.com/someuser"); got != "https://instagram.com/someuser" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractTweetID(t *testing.T) {
	cases := map[string]string{
		"https://x.com/user/status/12345":      "12345",
		"https://x.com/user/status/12345?s=20": "12345",
		"https://twitter.com/user/status/999/": "",
	}
	for url, want := range cases {
		if got := extractTweetID(url); got != want {
			t.Errorf("extractTweetID(%q) = %q, want %q", url, got, want)
		}
	}
}

func TestInstagramStoriesPageRe(t *testing.T) {
	if !instagramStoriesPageRe.MatchString("https://www.instagram.com/stories/someuser/") {
		t.Fatal("expected match for stories page")
	}
	if instagramStoriesPageRe.MatchString("https://www.instagram.com/reel/abc123/") {
		t.Fatal("expected no match for reel link")
	}
}
