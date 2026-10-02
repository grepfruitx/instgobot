package ratelimit

import (
	"testing"

	"github.com/grepfruitx/instgobot/internal/cache"
)

func newTestLimiter(t *testing.T) *Limiter {
	t.Helper()
	return New(cache.New(t.Context()))
}

func TestCheckGeneralAllowsUpToLimit(t *testing.T) {
	l := newTestLimiter(t)

	for i := 0; i < GeneralLimit; i++ {
		if !l.CheckGeneral(1).Allowed {
			t.Fatalf("expected request %d to be allowed", i+1)
		}
	}

	res := l.CheckGeneral(1)
	if res.Allowed {
		t.Fatal("expected request over the limit to be denied")
	}
	if res.ResetTime.IsZero() {
		t.Fatal("expected non-zero reset time")
	}
}

func TestCheckGeneralPerUserIsolation(t *testing.T) {
	l := newTestLimiter(t)

	for i := 0; i < GeneralLimit; i++ {
		if !l.CheckGeneral(1).Allowed {
			t.Fatalf("user 1 request %d denied", i)
		}
	}

	if !l.CheckGeneral(2).Allowed {
		t.Fatal("expected different user to have its own limit")
	}
}

func TestCheckOncePerBlocksSecondRequest(t *testing.T) {
	l := newTestLimiter(t)

	if !l.CheckYouTube(1, false).Allowed {
		t.Fatal("first request should be allowed")
	}
	if l.CheckYouTube(1, false).Allowed {
		t.Fatal("expected second request within window to be denied")
	}
}

func TestPeekYouTubeDoesNotConsume(t *testing.T) {
	l := newTestLimiter(t)

	if !l.PeekYouTube(1, false).Allowed {
		t.Fatal("peek before any request should allow")
	}
	if !l.CheckYouTube(1, false).Allowed {
		t.Fatal("peek must not consume the slot")
	}
	if l.PeekYouTube(1, false).Allowed {
		t.Fatal("peek after request should deny")
	}
}

func TestCheckOncePerAdminBypass(t *testing.T) {
	l := newTestLimiter(t)

	for i := 0; i < 3; i++ {
		if !l.CheckTelegramStories(1, true).Allowed {
			t.Fatalf("admin request %d should always be allowed", i)
		}
	}
}
