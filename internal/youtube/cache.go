package youtube

import (
	"context"
	"fmt"
	"time"

	"github.com/grepfruitx/instgobot/internal/cache"
)

const (
	pendingTTL = 5 * time.Minute
	metaTTL    = 5 * time.Minute
)

func pendingKey(chatID int64) string {
	return fmt.Sprintf("yt:pending:%d", chatID)
}

func setPendingURL(c *cache.Cache, chatID int64, url string) {
	c.Set(pendingKey(chatID), url, pendingTTL)
}

func takePendingURL(c *cache.Cache, chatID int64) (string, bool) {
	v, ok := c.Get(pendingKey(chatID))
	if !ok {
		return "", false
	}
	c.Delete(pendingKey(chatID))
	return v.(string), true
}

func metaKey(url string) string {
	return "yt:meta:" + url
}

func getYtMeta(ctx context.Context, c *cache.Cache, ytDlpPath, url string) (*YtMeta, error) {
	if v, ok := c.Get(metaKey(url)); ok {
		return v.(*YtMeta), nil
	}

	meta, err := runYtDlpJSON(ctx, ytDlpPath, url)
	if err != nil {
		return nil, err
	}
	c.Set(metaKey(url), meta, metaTTL)
	return meta, nil
}
