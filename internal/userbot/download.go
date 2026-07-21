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

// downloadMediaStream streams loc directly through an io.Pipe — no
// buffering, for the single-item send path. The returned reader must be
// closed by the caller; closing propagates any download error into the
// concurrent read (e.g. multipart upload) consuming it.
func downloadMediaStream(ctx context.Context, api *tg.Client, loc tg.InputFileLocationClass) io.ReadCloser {
	pr, pw := io.Pipe()
	go func() {
		_, err := dl.Download(api, loc).Stream(ctx, pw)
		pw.CloseWithError(err)
	}()
	return pr
}

// downloadMediaToFile writes loc to a temp file instead of buffering in
// memory, for batch/album sends: we need to know per-item success before
// deciding what goes into the group, which a live io.Pipe can't tell us
// without either buffering or serializing the whole batch. Caller must
// remove the returned path.
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
