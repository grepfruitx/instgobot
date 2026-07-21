package userbot

import (
	"errors"
	"testing"

	"github.com/gotd/td/tg"
)

func TestRawChannelID(t *testing.T) {
	got, err := rawChannelID(-1001724666497)
	if err != nil {
		t.Fatalf("rawChannelID: %v", err)
	}
	if got != 1724666497 {
		t.Fatalf("got %d, want 1724666497", got)
	}
}

func TestRawChannelIDRejectsNonChannelStyle(t *testing.T) {
	if _, err := rawChannelID(-542142955); err == nil {
		t.Fatal("expected error for a plain user-style chat id")
	}
}

func TestBestPhotoSizeTypePicksLargest(t *testing.T) {
	sizes := []tg.PhotoSizeClass{
		&tg.PhotoSize{Type: "s", W: 100, H: 100},
		&tg.PhotoSize{Type: "y", W: 1280, H: 720},
		&tg.PhotoSize{Type: "m", W: 320, H: 180},
		&tg.PhotoStrippedSize{Type: "i"},
	}
	if got := bestPhotoSizeType(sizes); got != "y" {
		t.Fatalf("got %q, want %q", got, "y")
	}
}

func TestBestPhotoSizeTypeNoUsableSizes(t *testing.T) {
	sizes := []tg.PhotoSizeClass{&tg.PhotoStrippedSize{Type: "i"}}
	if got := bestPhotoSizeType(sizes); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestIsNoAccessError(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{errors.New("rpc error: CHANNEL_PRIVATE"), true},
		{errors.New("Could not find the input entity"), true},
		{errors.New("some unrelated network error"), false},
	}
	for _, c := range cases {
		if got := isNoAccessError(c.err); got != c.want {
			t.Errorf("isNoAccessError(%v) = %v, want %v", c.err, got, c.want)
		}
	}
}

func TestExtractDownloadablePhoto(t *testing.T) {
	media := &tg.MessageMediaPhoto{}
	media.SetPhoto(&tg.Photo{
		ID: 1, AccessHash: 2,
		Sizes: []tg.PhotoSizeClass{&tg.PhotoSize{Type: "y", W: 800, H: 600}},
	})
	loc, kind, ok := extractDownloadable(media)
	if !ok {
		t.Fatal("expected ok")
	}
	if kind != mediaPhoto {
		t.Fatalf("expected mediaPhoto, got %v", kind)
	}
	if _, isPhotoLoc := loc.(*tg.InputPhotoFileLocation); !isPhotoLoc {
		t.Fatalf("expected *tg.InputPhotoFileLocation, got %T", loc)
	}
}

func TestExtractDownloadableDocument(t *testing.T) {
	media := &tg.MessageMediaDocument{}
	media.SetDocument(&tg.Document{ID: 1, AccessHash: 2})
	loc, kind, ok := extractDownloadable(media)
	if !ok {
		t.Fatal("expected ok")
	}
	if kind != mediaVideo {
		t.Fatalf("expected mediaVideo, got %v", kind)
	}
	if _, isDocLoc := loc.(*tg.InputDocumentFileLocation); !isDocLoc {
		t.Fatalf("expected *tg.InputDocumentFileLocation, got %T", loc)
	}
}

func TestExtractDownloadableUnsupported(t *testing.T) {
	if _, _, ok := extractDownloadable(&tg.MessageMediaEmpty{}); ok {
		t.Fatal("expected not ok for empty media")
	}
}
