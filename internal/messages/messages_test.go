package messages

import (
	"strings"
	"testing"
)

func TestSplitMessageShortText(t *testing.T) {
	got := SplitMessage("hello", 4096)
	if len(got) != 1 || got[0] != "hello" {
		t.Fatalf("got %v", got)
	}
}

func TestSplitMessageSplitsOnLines(t *testing.T) {
	text := strings.Repeat("a", 5) + "\n" + strings.Repeat("b", 5)
	chunks := SplitMessage(text, 7)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d: %v", len(chunks), chunks)
	}
	if chunks[0] != strings.Repeat("a", 5) || chunks[1] != strings.Repeat("b", 5) {
		t.Fatalf("unexpected chunks: %v", chunks)
	}
}

func TestSplitMessageHardBreaksLongLine(t *testing.T) {
	line := strings.Repeat("x", 20)
	chunks := SplitMessage(line, 8)
	var rebuilt strings.Builder
	for _, c := range chunks {
		rebuilt.WriteString(c)
	}
	if rebuilt.String() != line {
		t.Fatalf("chunks don't reconstruct original text: %v", chunks)
	}
	for _, c := range chunks {
		if len(c) > 8 {
			t.Fatalf("chunk exceeds max length: %q", c)
		}
	}
}

func TestSplitMessageNoDataLoss(t *testing.T) {
	var lines []string
	for i := 0; i < 200; i++ {
		lines = append(lines, strings.Repeat("z", 30))
	}
	text := strings.Join(lines, "\n")
	chunks := SplitMessage(text, 100)

	rebuilt := strings.Join(chunks, "\n")
	if rebuilt != text {
		t.Fatalf("rejoined chunks don't match original (lengths: got %d want %d)", len(rebuilt), len(text))
	}
}
