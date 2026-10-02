package ratelimit

import (
	"fmt"
	"time"

	"github.com/grepfruitx/instgobot/internal/cache"
)

const (
	GeneralLimit  = 5
	GeneralWindow = time.Minute

	TelegramStoriesWindow = 3 * time.Minute
	YouTubeWindow         = 3 * time.Minute
)

type Limiter struct {
	c *cache.Cache
}

func New(c *cache.Cache) *Limiter {
	return &Limiter{c: c}
}

type Result struct {
	Allowed   bool
	ResetTime time.Time
}

func (l *Limiter) CheckGeneral(userID int64) Result {
	count, ttl := l.c.Incr(fmt.Sprintf("rl:general:%d", userID), GeneralWindow)
	if count > GeneralLimit {
		return Result{Allowed: false, ResetTime: time.Now().Add(ttl)}
	}
	return Result{Allowed: true}
}

func (l *Limiter) checkOncePer(key string, window time.Duration, isAdmin bool) Result {
	if isAdmin {
		return Result{Allowed: true}
	}
	if ok, ttl := l.c.SetNX(key, true, window); !ok {
		return Result{Allowed: false, ResetTime: time.Now().Add(ttl)}
	}
	return Result{Allowed: true}
}

func (l *Limiter) CheckTelegramStories(userID int64, isAdmin bool) Result {
	return l.checkOncePer(fmt.Sprintf("rl:tgstories:%d", userID), TelegramStoriesWindow, isAdmin)
}

func (l *Limiter) CheckYouTube(userID int64, isAdmin bool) Result {
	return l.checkOncePer(fmt.Sprintf("rl:youtube:%d", userID), YouTubeWindow, isAdmin)
}

func (l *Limiter) PeekYouTube(userID int64, isAdmin bool) Result {
	if isAdmin {
		return Result{Allowed: true}
	}
	ttl := l.c.TTL(fmt.Sprintf("rl:youtube:%d", userID))
	if ttl <= 0 {
		return Result{Allowed: true}
	}
	return Result{Allowed: false, ResetTime: time.Now().Add(ttl)}
}
