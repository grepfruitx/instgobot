package snapsave

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/grepfruitx/instgobot/internal/media"
)

func extractTweetID(tweetURL string) string {
	parts := strings.Split(tweetURL, "/")
	last := parts[len(parts)-1]
	return strings.Split(last, "?")[0]
}

func convertTweetToImage(ctx context.Context, tweetURL string) ([]byte, error) {
	tweetID := extractTweetID(tweetURL)
	if tweetID == "" {
		return nil, fmt.Errorf("could not extract tweet id from url")
	}

	resp, err := media.FetchWithTimeout(ctx, fmt.Sprintf("https://twtoimage.vercel.app/api/tweet-to-image/%s", tweetID), 15*time.Second)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP error! status: %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
