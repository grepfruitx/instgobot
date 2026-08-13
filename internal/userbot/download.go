package userbot

import (
	"context"
	"io"
	"os"

	"github.com/gotd/td/telegram/downloader"
	"github.com/gotd/td/tg"

	"github.com/grepfruitx/instgobot/internal/media"
	"github.com/grepfruitx/instgobot/internal/telegramapi"
)

var dl = downloader.NewDownloader()

func checkSize(size int64) error {
	if size > media.MaxFileSize {
		return &telegramapi.FileTooLargeError{SizeBytes: size}
	}
	return nil
}

func downloadMediaStream(ctx context.Context, api *tg.Client, loc tg.InputFileLocationClass) io.ReadCloser {
	pr, pw := io.Pipe()
	go func() {
		_, err := dl.Download(api, loc).Stream(ctx, pw)
		pw.CloseWithError(err)
	}()
	return pr
}

func downloadMediaToFile(ctx context.Context, api *tg.Client, loc tg.InputFileLocationClass) (string, error) {
	f, err := os.CreateTemp("", "tgmedia_*")
	if err != nil {
		return "", err
	}
	defer f.Close()

	if _, err := dl.Download(api, loc).Stream(ctx, f); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}
