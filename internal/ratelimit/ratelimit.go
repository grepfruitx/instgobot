package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	GeneralLimit  = 5
	GeneralWindow = time.Minute

	TelegramStoriesWindow = 3 * time.Minute
	YouTubeWindow         = 3 * time.Minute
)

type Limiter struct {
	rdb *redis.Client
}

func New(rdb *redis.Client) *Limiter {
	return &Limiter{rdb: rdb}
}

type Result struct {
	Allowed   bool
	ResetTime time.Time
}

func (l *Limiter) CheckGeneral(ctx context.Context, userID int64) (Result, error) {
	key := fmt.Sprintf("rl:general:%d", userID)

	count, err := l.rdb.Incr(ctx, key).Result()
	if err != nil {
		return Result{}, err
	}
	if count == 1 {
		if err := l.rdb.Expire(ctx, key, GeneralWindow).Err(); err != nil {
			return Result{}, err
		}
	}
	if count > GeneralLimit {
		ttl, err := l.rdb.TTL(ctx, key).Result()
		if err != nil {
			return Result{}, err
		}
		return Result{Allowed: false, ResetTime: time.Now().Add(ttl)}, nil
	}
	return Result{Allowed: true}, nil
}

func (l *Limiter) checkOncePer(ctx context.Context, key string, window time.Duration, isAdmin bool) (Result, error) {
	if isAdmin {
		return Result{Allowed: true}, nil
	}

	ok, err := l.rdb.SetNX(ctx, key, "1", window).Result()
	if err != nil {
		return Result{}, err
	}
	if ok {
		return Result{Allowed: true}, nil
	}

	ttl, err := l.rdb.TTL(ctx, key).Result()
	if err != nil {
		return Result{}, err
	}
	return Result{Allowed: false, ResetTime: time.Now().Add(ttl)}, nil
}

func (l *Limiter) CheckTelegramStories(ctx context.Context, userID int64, isAdmin bool) (Result, error) {
	return l.checkOncePer(ctx, fmt.Sprintf("rl:tgstories:%d", userID), TelegramStoriesWindow, isAdmin)
}

func (l *Limiter) CheckYouTube(ctx context.Context, userID int64, isAdmin bool) (Result, error) {
	return l.checkOncePer(ctx, fmt.Sprintf("rl:youtube:%d", userID), YouTubeWindow, isAdmin)
}

func (l *Limiter) peekOncePer(ctx context.Context, key string) (Result, error) {
	ttl, err := l.rdb.TTL(ctx, key).Result()
	if err != nil {
		return Result{}, err
	}
	if ttl <= 0 {
		return Result{Allowed: true}, nil
	}
	return Result{Allowed: false, ResetTime: time.Now().Add(ttl)}, nil
}

func (l *Limiter) PeekYouTube(ctx context.Context, userID int64, isAdmin bool) (Result, error) {
	if isAdmin {
		return Result{Allowed: true}, nil
	}
	return l.peekOncePer(ctx, fmt.Sprintf("rl:youtube:%d", userID))
}
