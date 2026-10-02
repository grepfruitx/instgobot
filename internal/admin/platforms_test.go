package admin

import (
	"strings"
	"testing"

	"github.com/grepfruitx/instgobot/internal/config"
)

func TestPlatformToggleNotifiesWaitlistOnPon(t *testing.T) {
	st := newTestStore(t)
	b, sent := newTestBotCapturingMessagesConcurrent(t)
	h := New(b, st, nil, nil)
	adminID := config.AdminUserIDs[0]

	if err := st.SetPlatformDisabled("tiktok", true); err != nil {
		t.Fatalf("SetPlatformDisabled: %v", err)
	}
	if err := st.JoinWaitlist(100, "tiktok"); err != nil {
		t.Fatalf("JoinWaitlist: %v", err)
	}
	if err := st.JoinWaitlist(101, "tiktok"); err != nil {
		t.Fatalf("JoinWaitlist: %v", err)
	}

	h.HandleCommand(t.Context(), adminID, "/pon tiktok", adminID)

	msgs := sent.all()
	var notified []int64
	for _, m := range msgs {
		if m.ChatID == 100 || m.ChatID == 101 {
			if !strings.Contains(m.Text, "TIKTOK") {
				t.Fatalf("unexpected waitlist notification text: %q", m.Text)
			}
			notified = append(notified, m.ChatID)
		}
	}
	if len(notified) != 2 {
		t.Fatalf("expected 2 waitlist notifications, got %d: %+v", len(notified), msgs)
	}

	waiting, err := st.GetWaitlist("tiktok")
	if err != nil {
		t.Fatalf("GetWaitlist: %v", err)
	}
	if len(waiting) != 0 {
		t.Fatalf("expected waitlist cleared after notification, got %v", waiting)
	}
}

func TestPlatformToggleSkipsWaitlistOnPoff(t *testing.T) {
	st := newTestStore(t)
	b, sent := newTestBotCapturingMessagesConcurrent(t)
	h := New(b, st, nil, nil)
	adminID := config.AdminUserIDs[0]

	if err := st.JoinWaitlist(200, "instagram"); err != nil {
		t.Fatalf("JoinWaitlist: %v", err)
	}

	h.HandleCommand(t.Context(), adminID, "/poff instagram", adminID)

	for _, m := range sent.all() {
		if m.ChatID == 200 {
			t.Fatalf("did not expect a waitlist notification on /poff, got: %+v", m)
		}
	}

	waiting, err := st.GetWaitlist("instagram")
	if err != nil {
		t.Fatalf("GetWaitlist: %v", err)
	}
	if len(waiting) != 1 {
		t.Fatalf("expected waitlist untouched by /poff, got %v", waiting)
	}
}
