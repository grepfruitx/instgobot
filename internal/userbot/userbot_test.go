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

func TestBestPhotoSizePicksLargest(t *testing.T) {
	sizes := []tg.PhotoSizeClass{
		&tg.PhotoSize{Type: "s", W: 100, H: 100, Size: 1000},
		&tg.PhotoSize{Type: "y", W: 1280, H: 720, Size: 50000},
		&tg.PhotoSize{Type: "m", W: 320, H: 180, Size: 5000},
		&tg.PhotoStrippedSize{Type: "i"},
	}
	typ, size := bestPhotoSize(sizes)
	if typ != "y" {
		t.Fatalf("got type %q, want %q", typ, "y")
	}
	if size != 50000 {
		t.Fatalf("got size %d, want 50000", size)
	}
}

func TestBestPhotoSizeNoUsableSizes(t *testing.T) {
	sizes := []tg.PhotoSizeClass{&tg.PhotoStrippedSize{Type: "i"}}
	typ, size := bestPhotoSize(sizes)
	if typ != "" || size != 0 {
		t.Fatalf("got (%q, %d), want (\"\", 0)", typ, size)
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
		Sizes: []tg.PhotoSizeClass{&tg.PhotoSize{Type: "y", W: 800, H: 600, Size: 12345}},
	})
	loc, kind, size, ok := extractDownloadable(media)
	if !ok {
		t.Fatal("expected ok")
	}
	if kind != mediaPhoto {
		t.Fatalf("expected mediaPhoto, got %v", kind)
	}
	if size != 12345 {
		t.Fatalf("expected size 12345, got %d", size)
	}
	if _, isPhotoLoc := loc.(*tg.InputPhotoFileLocation); !isPhotoLoc {
		t.Fatalf("expected *tg.InputPhotoFileLocation, got %T", loc)
	}
}

func TestExtractDownloadableDocument(t *testing.T) {
	media := &tg.MessageMediaDocument{}
	media.SetDocument(&tg.Document{ID: 1, AccessHash: 2, Size: 98765})
	loc, kind, size, ok := extractDownloadable(media)
	if !ok {
		t.Fatal("expected ok")
	}
	if kind != mediaVideo {
		t.Fatalf("expected mediaVideo, got %v", kind)
	}
	if size != 98765 {
		t.Fatalf("expected size 98765, got %d", size)
	}
	if _, isDocLoc := loc.(*tg.InputDocumentFileLocation); !isDocLoc {
		t.Fatalf("expected *tg.InputDocumentFileLocation, got %T", loc)
	}
}

func TestExtractDownloadableUnsupported(t *testing.T) {
	if _, _, _, ok := extractDownloadable(&tg.MessageMediaEmpty{}); ok {
		t.Fatal("expected not ok for empty media")
	}
}

func TestCheckSize(t *testing.T) {
	if err := checkSize(1024); err != nil {
		t.Fatalf("expected small size to pass, got %v", err)
	}
	if err := checkSize(60 * 1024 * 1024); err == nil {
		t.Fatal("expected oversized media to fail")
	}
}
