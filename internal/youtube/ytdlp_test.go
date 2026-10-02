package youtube

import "testing"

func TestChooseVideoFormatPrefersAdaptive(t *testing.T) {
	formats := []ytDlpFormat{
		{FormatID: "137", VCodec: "avc1", ACodec: "none", Ext: "mp4", Height: 1080, Width: 1920, TBR: 5000},
		{FormatID: "18", VCodec: "avc1", ACodec: "mp4a", Ext: "mp4", Height: 360, Width: 640},
		{FormatID: "140", VCodec: "none", ACodec: "mp4a", Ext: "m4a", ABR: 128},
	}
	chosen := chooseVideoFormat(formats, 1080)
	if chosen == nil {
		t.Fatal("expected a match")
	}
	if chosen.Kind != kindAdaptive {
		t.Fatalf("expected adaptive, got %v", chosen.Kind)
	}
	if chosen.VideoFormatID != "137" || chosen.AudioFormatID != "140" {
		t.Fatalf("unexpected format ids: %+v", chosen)
	}
}

func TestChooseVideoFormatFallsBackToMuxed(t *testing.T) {
	formats := []ytDlpFormat{
		{FormatID: "18", VCodec: "avc1", ACodec: "mp4a", Ext: "mp4", Height: 360, Width: 640},
		{FormatID: "36", VCodec: "mp4v", ACodec: "mp4a", Ext: "3gp", Height: 240, Width: 320},
	}
	chosen := chooseVideoFormat(formats, 480)
	if chosen == nil {
		t.Fatal("expected a match")
	}
	if chosen.Kind != kindMuxed {
		t.Fatalf("expected muxed, got %v", chosen.Kind)
	}
	if chosen.FormatID != "18" {
		t.Fatalf("expected mp4 muxed format preferred, got %s", chosen.FormatID)
	}
}

func TestChooseVideoFormatRespectsQualityCeiling(t *testing.T) {
	formats := []ytDlpFormat{
		{FormatID: "137", VCodec: "avc1", ACodec: "none", Ext: "mp4", Height: 1080, Width: 1920},
		{FormatID: "135", VCodec: "avc1", ACodec: "none", Ext: "mp4", Height: 480, Width: 854},
		{FormatID: "140", VCodec: "none", ACodec: "mp4a", Ext: "m4a", ABR: 128},
	}
	chosen := chooseVideoFormat(formats, 480)
	if chosen == nil {
		t.Fatal("expected a match")
	}
	if chosen.Height != 480 {
		t.Fatalf("expected 480p (ceiling), got %d", chosen.Height)
	}
}

func TestChooseVideoFormatPortraitUsesShortEdge(t *testing.T) {
	formats := []ytDlpFormat{
		{FormatID: "portrait720", VCodec: "avc1", ACodec: "none", Ext: "mp4", Height: 1280, Width: 720, TBR: 3000},
		{FormatID: "portrait360", VCodec: "avc1", ACodec: "none", Ext: "mp4", Height: 640, Width: 360, TBR: 1000},
		{FormatID: "140", VCodec: "none", ACodec: "mp4a", Ext: "m4a", ABR: 128},
	}
	chosen := chooseVideoFormat(formats, 720)
	if chosen == nil {
		t.Fatal("expected a match")
	}
	if chosen.VideoFormatID != "portrait720" {
		t.Fatalf("expected the 720-short-edge portrait format despite height=1280, got %s", chosen.VideoFormatID)
	}
}

func TestChooseVideoFormatNoMatch(t *testing.T) {
	formats := []ytDlpFormat{
		{FormatID: "1", VCodec: "none", ACodec: "none", Ext: "mp4", Height: 0},
	}
	if chosen := chooseVideoFormat(formats, 720); chosen != nil {
		t.Fatalf("expected nil, got %+v", chosen)
	}
}

func TestChooseVideoFormatSizeSumsAdaptiveParts(t *testing.T) {
	formats := []ytDlpFormat{
		{FormatID: "137", VCodec: "avc1", ACodec: "none", Ext: "mp4", Height: 1080, Width: 1920, Filesize: maxUploadBytes},
		{FormatID: "140", VCodec: "none", ACodec: "mp4a", Ext: "m4a", ABR: 128, FilesizeApprox: 1},
	}
	chosen := chooseVideoFormat(formats, 1080)
	if chosen == nil || chosen.Size != maxUploadBytes+1 || !chosen.tooLarge() {
		t.Fatalf("expected oversize adaptive pick, got %+v", chosen)
	}
}

func TestChooseVideoFormatUnknownSizeAllowed(t *testing.T) {
	formats := []ytDlpFormat{
		{FormatID: "18", VCodec: "avc1", ACodec: "mp4a", Ext: "mp4", Height: 360, Width: 640},
	}
	chosen := chooseVideoFormat(formats, 360)
	if chosen == nil || chosen.tooLarge() {
		t.Fatalf("unknown size must not be rejected, got %+v", chosen)
	}
}
