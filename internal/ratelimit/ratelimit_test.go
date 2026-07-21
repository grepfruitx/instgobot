package ratelimit

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestLimiter(t *testing.T) *Limiter {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	t.Cleanup(mr.Close)

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })
	return New(rdb)
}

func TestCheckGeneralAllowsUpToLimit(t *testing.T) {
	l := newTestLimiter(t)
	ctx := t.Context()

	for i := 0; i < GeneralLimit; i++ {
		res, err := l.CheckGeneral(ctx, 1)
		if err != nil {
			t.Fatalf("CheckGeneral: %v", err)
		}
		if !res.Allowed {
			t.Fatalf("expected request %d to be allowed", i+1)
		}
	}

	res, err := l.CheckGeneral(ctx, 1)
	if err != nil {
		t.Fatalf("CheckGeneral: %v", err)
	}
	if res.Allowed {
		t.Fatal("expected request over the limit to be denied")
	}
	if res.ResetTime.IsZero() {
		t.Fatal("expected non-zero reset time")
	}
}

func TestCheckGeneralPerUserIsolation(t *testing.T) {
	l := newTestLimiter(t)
	ctx := t.Context()

	for i := 0; i < GeneralLimit; i++ {
		if res, err := l.CheckGeneral(ctx, 1); err != nil || !res.Allowed {
			t.Fatalf("user 1 request %d: allowed=%v err=%v", i, res.Allowed, err)
		}
	}

	res, err := l.CheckGeneral(ctx, 2)
	if err != nil {
		t.Fatalf("CheckGeneral: %v", err)
	}
	if !res.Allowed {
		t.Fatal("expected different user to have its own limit")
	}
}

func TestCheckOncePerBlocksSecondRequest(t *testing.T) {
	l := newTestLimiter(t)
	ctx := t.Context()

	res, err := l.CheckYouTube(ctx, 1, false)
	if err != nil || !res.Allowed {
		t.Fatalf("first request should be allowed: allowed=%v err=%v", res.Allowed, err)
	}

	res, err = l.CheckYouTube(ctx, 1, false)
	if err != nil {
		t.Fatalf("CheckYouTube: %v", err)
	}
	if res.Allowed {
		t.Fatal("expected second request within window to be denied")
	}
}

func TestCheckOncePerAdminBypass(t *testing.T) {
	l := newTestLimiter(t)
	ctx := t.Context()

	for i := 0; i < 3; i++ {
		res, err := l.CheckTelegramStories(ctx, 1, true)
		if err != nil || !res.Allowed {
			t.Fatalf("admin request %d should always be allowed: allowed=%v err=%v", i, res.Allowed, err)
		}
	}
}
