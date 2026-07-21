package youtube

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	pendingTTL = 5 * time.Minute
	metaTTL    = 5 * time.Minute
)

func pendingKey(chatID int64) string {
	return fmt.Sprintf("yt:pending:%d", chatID)
}

func setPendingURL(ctx context.Context, rdb *redis.Client, chatID int64, url string) error {
	return rdb.Set(ctx, pendingKey(chatID), url, pendingTTL).Err()
}

func getPendingURL(ctx context.Context, rdb *redis.Client, chatID int64) (string, bool, error) {
	url, err := rdb.Get(ctx, pendingKey(chatID)).Result()
	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return url, true, nil
}

func deletePendingURL(ctx context.Context, rdb *redis.Client, chatID int64) error {
	return rdb.Del(ctx, pendingKey(chatID)).Err()
}

func metaKey(url string) string {
	return "yt:meta:" + url
}

func getCachedMeta(ctx context.Context, rdb *redis.Client, url string) (*YtMeta, bool, error) {
	raw, err := rdb.Get(ctx, metaKey(url)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var meta YtMeta
	if err := json.Unmarshal(raw, &meta); err != nil {
		return nil, false, err
	}
	return &meta, true, nil
}

func setCachedMeta(ctx context.Context, rdb *redis.Client, url string, meta *YtMeta) error {
	raw, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	return rdb.Set(ctx, metaKey(url), raw, metaTTL).Err()
}

func getYtMeta(ctx context.Context, rdb *redis.Client, ytDlpPath, url string) (*YtMeta, error) {
	if meta, ok, err := getCachedMeta(ctx, rdb, url); err == nil && ok {
		return meta, nil
	}

	meta, err := runYtDlpJSON(ctx, ytDlpPath, url)
	if err != nil {
		return nil, err
	}
	_ = setCachedMeta(ctx, rdb, url, meta)
	return meta, nil
}
