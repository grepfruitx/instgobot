package admin

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const (
	serviceName     = "instgobot"
	defaultLogLines = 200
	maxLogLines     = 5000
)

var (
	startedAt      = time.Now()
	journalctlPath = "journalctl"
	botTokenRe     = regexp.MustCompile(`\d{6,}:[A-Za-z0-9_-]{30,}`)
)

func buildVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "неизвестно"
	}
	var rev, at string
	var dirty bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.time":
			at = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if rev == "" {
		return "неизвестно"
	}
	rev = rev[:min(7, len(rev))]
	if dirty {
		rev += "-dirty"
	}
	if at != "" {
		rev += " от " + at
	}
	return rev
}

func uptime() string {
	return time.Since(startedAt).Round(time.Second).String()
}

func runCmd(ctx context.Context, timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func (h *Handler) handleRestart(ctx context.Context, chatID int64) {
	if h.restart == nil {
		h.send(ctx, chatID, "Перезапуск недоступен.")
		return
	}
	h.send(ctx, chatID, "Перезапускаюсь: дождусь активных загрузок (до 15 мин), потом systemd поднимет бота заново.")
	h.restart()
}

func (h *Handler) handleYtDlp(ctx context.Context, chatID int64, args []string) {
	if len(args) > 0 && strings.EqualFold(args[0], "update") {
		h.send(ctx, chatID, "Обновляю yt-dlp...")
		out, err := runCmd(ctx, 2*time.Minute, h.ytDlpPath, "-U")
		if err != nil {
			h.sendChunked(ctx, chatID, fmt.Sprintf("yt-dlp -U завершился с ошибкой: %v\n\n%s", err, out))
			return
		}
		h.sendChunked(ctx, chatID, out)
		return
	}

	out, err := runCmd(ctx, 10*time.Second, h.ytDlpPath, "--version")
	if err != nil {
		h.send(ctx, chatID, fmt.Sprintf("Не удалось получить версию yt-dlp: %v\n%s", err, out))
		return
	}
	h.send(ctx, chatID, fmt.Sprintf("yt-dlp: %s\n\nОбновить: /ytdlp update", out))
}

func logLines(args []string) int {
	if len(args) == 0 {
		return defaultLogLines
	}
	n, err := strconv.Atoi(args[0])
	if err != nil || n <= 0 {
		return defaultLogLines
	}
	return min(n, maxLogLines)
}

func (h *Handler) handleLogs(ctx context.Context, chatID int64, args []string) {
	n := logLines(args)
	out, err := runCmd(ctx, 15*time.Second, journalctlPath, "-u", serviceName, "-n", strconv.Itoa(n), "--no-pager", "-o", "short-iso")
	if err != nil {
		h.send(ctx, chatID, fmt.Sprintf("journalctl завершился с ошибкой: %v\n%s", err, out))
		return
	}
	if out == "" {
		h.send(ctx, chatID, "Логи пусты.")
		return
	}
	_, err = h.b.SendDocument(ctx, &bot.SendDocumentParams{
		ChatID:   chatID,
		Document: &models.InputFileUpload{Filename: fmt.Sprintf("%s-%s.log", serviceName, time.Now().Format("20060102-150405")), Data: bytes.NewReader([]byte(botTokenRe.ReplaceAllString(out, "<redacted>")))},
		Caption:  fmt.Sprintf("Последние %d строк журнала %s", n, serviceName),
	})
	if err != nil {
		h.send(ctx, chatID, fmt.Sprintf("Не удалось отправить логи: %v", err))
	}
}
