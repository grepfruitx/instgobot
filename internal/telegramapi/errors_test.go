package telegramapi

import (
	"errors"
	"fmt"
	"testing"

	"github.com/go-telegram/bot"
)

func TestIsBotBlockedError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"forbidden sentinel", fmt.Errorf("%w, bot was blocked by the user", bot.ErrorForbidden), true},
		{"chat not found substring", errors.New("Bad Request: chat not found"), true},
		{"user deactivated substring", errors.New("Forbidden: user is deactivated"), true},
		{"unrelated error", errors.New("network timeout"), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsBotBlockedError(tc.err); got != tc.want {
				t.Fatalf("IsBotBlockedError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestFileTooLargeErrorMessage(t *testing.T) {
	err := &FileTooLargeError{SizeBytes: 60 * 1024 * 1024}
	want := "file too large: 60MB (limit: 50MB)"
	if err.Error() != want {
		t.Fatalf("got %q, want %q", err.Error(), want)
	}
}

func TestShouldSkipReport(t *testing.T) {
	if !shouldSkipReport(&FileTooLargeError{SizeBytes: 100}) {
		t.Fatal("expected FileTooLargeError to be skipped")
	}
	if !shouldSkipReport(&MediaFetchError{Reason: "boom"}) {
		t.Fatal("expected MediaFetchError to be skipped")
	}
	if !shouldSkipReport(errors.New("413 Request Entity Too Large")) {
		t.Fatal("expected 413 error to be skipped")
	}
	if shouldSkipReport(errors.New("some other failure")) {
		t.Fatal("expected unrelated error not to be skipped")
	}
}
