package telegramapi

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-telegram/bot"
)

type FileTooLargeError struct {
	SizeBytes int64
}

func (e *FileTooLargeError) Error() string {
	return fmt.Sprintf("file too large: %dMB (limit: 50MB)", e.SizeBytes/1024/1024)
}

type MediaFetchError struct {
	Reason string
}

func (e *MediaFetchError) Error() string {
	return e.Reason
}

func IsBotBlockedError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, bot.ErrorForbidden) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "bot was blocked by the user") ||
		strings.Contains(msg, "user is deactivated") ||
		strings.Contains(msg, "chat not found")
}
