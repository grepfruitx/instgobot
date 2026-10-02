package admin

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/go-telegram/bot"

	"github.com/grepfruitx/instgobot/internal/config"
)

func fakeBin(t *testing.T, script string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "fake")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLogLinesClamp(t *testing.T) {
	cases := map[string]int{"": defaultLogLines, "50": 50, "abc": defaultLogLines, "-3": defaultLogLines, "99999": maxLogLines}
	for in, want := range cases {
		var args []string
		if in != "" {
			args = []string{in}
		}
		if got := logLines(args); got != want {
			t.Errorf("logLines(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestYtDlpVersionAndUpdate(t *testing.T) {
	st := newTestStore(t)
	b, sent := newTestBotCapturingMessages(t)
	bin := fakeBin(t, `if [ "$1" = "-U" ]; then echo "Updated yt-dlp to 2026.09.30"; else echo "2026.09.01"; fi`)
	h := New(b, st, nil, nil, bin)
	admin := config.AdminUserIDs[0]

	h.HandleCommand(t.Context(), 1, "/ytdlp", admin)
	if len(*sent) != 1 || !strings.Contains((*sent)[0].Text, "yt-dlp: 2026.09.01") {
		t.Fatalf("version: %v", *sent)
	}

	h.HandleCommand(t.Context(), 1, "/ytdlp update", admin)
	last := (*sent)[len(*sent)-1].Text
	if !strings.Contains(last, "Updated yt-dlp to 2026.09.30") {
		t.Fatalf("update: %v", *sent)
	}
}

func TestYtDlpUpdateFailureReportsOutput(t *testing.T) {
	st := newTestStore(t)
	b, sent := newTestBotCapturingMessages(t)
	h := New(b, st, nil, nil, fakeBin(t, `echo "ERROR: You installed yt-dlp with pip"; exit 1`))

	h.HandleCommand(t.Context(), 1, "/ytdlp update", config.AdminUserIDs[0])
	last := (*sent)[len(*sent)-1].Text
	if !strings.Contains(last, "с ошибкой") || !strings.Contains(last, "installed yt-dlp with pip") {
		t.Fatalf("unexpected: %v", *sent)
	}
}

func TestLogsSendsJournalAsDocument(t *testing.T) {
	var mu sync.Mutex
	var doc, gotArgs string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/sendDocument") {
			if f, _, err := r.FormFile("document"); err == nil {
				data, _ := io.ReadAll(f)
				mu.Lock()
				doc = string(data)
				mu.Unlock()
			}
		}
		fmt.Fprint(w, `{"ok":true,"result":{"message_id":1,"date":0,"chat":{"id":1,"type":"private"}}}`)
	}))
	t.Cleanup(srv.Close)
	b, err := bot.New("test-token", bot.WithServerURL(srv.URL), bot.WithSkipGetMe())
	if err != nil {
		t.Fatal(err)
	}

	argsFile := filepath.Join(t.TempDir(), "args")
	old := journalctlPath
	journalctlPath = fakeBin(t, `echo "$@" > `+argsFile+`; echo "line one"; echo "GET https://api.telegram.org/bot123456789:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsawQ/getUpdates"`)
	t.Cleanup(func() { journalctlPath = old })

	h := New(b, newTestStore(t), nil, nil, "")
	h.HandleCommand(t.Context(), 1, "/logs 50", config.AdminUserIDs[0])

	raw, _ := os.ReadFile(argsFile)
	gotArgs = strings.TrimSpace(string(raw))
	if gotArgs != "-u instgobot -n 50 --no-pager -o short-iso" {
		t.Fatalf("journalctl args: %q", gotArgs)
	}
	mu.Lock()
	defer mu.Unlock()
	if doc != "line one\nGET https://api.telegram.org/bot<redacted>/getUpdates" {
		t.Fatalf("document content: %q", doc)
	}
}
