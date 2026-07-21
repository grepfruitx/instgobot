package userbot

import (
	"bytes"
	"context"

	"github.com/gotd/td/telegram/downloader"
	"github.com/gotd/td/tg"
)

var dl = downloader.NewDownloader()

func downloadMediaBytes(ctx context.Context, api *tg.Client, loc tg.InputFileLocationClass) ([]byte, error) {
	var buf bytes.Buffer
	if _, err := dl.Download(api, loc).Stream(ctx, &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
