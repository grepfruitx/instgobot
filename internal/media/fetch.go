package media

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/grepfruitx/instgobot/internal/messages"
	"github.com/grepfruitx/instgobot/internal/telegramapi"
)

const MaxFileSize = 50 * 1024 * 1024
const maxGroupSize = 40 * 1024 * 1024

var httpClient = &http.Client{
	Transport: &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return (&net.Dialer{Timeout: 30 * time.Second}).DialContext(ctx, "tcp4", addr)
		},
	},
}

type cancelOnCloseBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *cancelOnCloseBody) Close() error {
	err := b.ReadCloser.Close()
	b.cancel()
	return err
}

func PostJSON(ctx context.Context, url string, payload any, timeout time.Duration) (*http.Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	reqCtx, cancel := context.WithTimeout(ctx, timeout)

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		cancel()
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		cancel()
		if reqCtx.Err() == context.DeadlineExceeded {
			return nil, &telegramapi.MediaFetchError{Reason: messages.FetchResponseTimeout}
		}
		return nil, err
	}

	resp.Body = &cancelOnCloseBody{ReadCloser: resp.Body, cancel: cancel}
	return resp, nil
}

func FetchWithTimeout(ctx context.Context, url string, timeout time.Duration) (*http.Response, error) {
	reqCtx, cancel := context.WithTimeout(ctx, timeout)

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		cancel()
		return nil, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		cancel()
		if reqCtx.Err() == context.DeadlineExceeded {
			return nil, &telegramapi.MediaFetchError{Reason: messages.FetchBodyTimeout}
		}
		if strings.Contains(err.Error(), "stopped after") && strings.Contains(err.Error(), "redirect") {
			return nil, &telegramapi.MediaFetchError{Reason: messages.FetchTooManyRedirects}
		}
		return nil, err
	}

	resp.Body = &cancelOnCloseBody{ReadCloser: resp.Body, cancel: cancel}
	return resp, nil
}

func FetchMediaResponse(ctx context.Context, url string, skipSizeCheck bool) (*http.Response, error) {
	resp, err := FetchWithTimeout(ctx, url, 30*time.Second)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		return nil, &telegramapi.MediaFetchError{Reason: fmt.Sprintf(messages.FetchBadStatusFmt, resp.StatusCode)}
	}
	if !skipSizeCheck {
		if cl := resp.Header.Get("Content-Length"); cl != "" {
			if size, err := strconv.ParseInt(cl, 10, 64); err == nil && size > MaxFileSize {
				resp.Body.Close()
				return nil, &telegramapi.FileTooLargeError{SizeBytes: size}
			}
		}
	}
	return resp, nil
}
