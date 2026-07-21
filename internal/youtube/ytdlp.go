package youtube

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sort"
)

type ytDlpFormat struct {
	FormatID string  `json:"format_id"`
	VCodec   string  `json:"vcodec"`
	ACodec   string  `json:"acodec"`
	Ext      string  `json:"ext"`
	Height   int     `json:"height"`
	Width    int     `json:"width"`
	TBR      float64 `json:"tbr"`
	ABR      float64 `json:"abr"`
}

type YtMeta struct {
	Title        string        `json:"title"`
	Duration     int           `json:"duration"`
	ThumbnailURL string        `json:"thumbnailUrl"`
	Formats      []ytDlpFormat `json:"formats"`
}

func runYtDlpJSON(ctx context.Context, ytDlpPath, url string) (*YtMeta, error) {
	cmd := exec.CommandContext(ctx, ytDlpPath, "--dump-json", "--no-playlist", "--no-cache-dir", url)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("yt-dlp: %s: %w", stderr.String(), err)
	}

	var info struct {
		Title     string        `json:"title"`
		Duration  float64       `json:"duration"`
		Thumbnail string        `json:"thumbnail"`
		Formats   []ytDlpFormat `json:"formats"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &info); err != nil {
		return nil, err
	}

	title := info.Title
	if title == "" {
		title = "YouTube видео"
	}
	return &YtMeta{Title: title, Duration: int(info.Duration), ThumbnailURL: info.Thumbnail, Formats: info.Formats}, nil
}

type cmdReadCloser struct {
	io.ReadCloser
	cmd *exec.Cmd
}

func (c *cmdReadCloser) Close() error {
	err := c.ReadCloser.Close()
	if waitErr := c.cmd.Wait(); err == nil {
		err = waitErr
	}
	return err
}

func ytDlpStream(ctx context.Context, ytDlpPath string, args []string) (io.ReadCloser, error) {
	cmd := exec.CommandContext(ctx, ytDlpPath, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &cmdReadCloser{ReadCloser: stdout, cmd: cmd}, nil
}

func ytDlpMergeToDisk(ctx context.Context, ytDlpPath string, args []string, outPath string) error {
	cmd := exec.CommandContext(ctx, ytDlpPath, append(args, "-o", outPath)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("yt-dlp: %s: %w", stderr.String(), err)
	}
	return nil
}

type videoKind int

const (
	kindAdaptive videoKind = iota
	kindMuxed
)

type chosenVideo struct {
	Kind          videoKind
	VideoFormatID string
	AudioFormatID string
	FormatID      string
	Height        int
	Width         int
}

func shortEdge(f ytDlpFormat) int {
	if f.Width > 0 && f.Width < f.Height {
		return f.Width
	}
	return f.Height
}

func chooseVideoFormat(formats []ytDlpFormat, quality int) *chosenVideo {
	var videoOnly, audioOnly, muxed []ytDlpFormat

	for _, f := range formats {
		switch {
		case f.VCodec != "none" && f.VCodec != "" && f.ACodec == "none" && f.Ext == "mp4" && f.Height > 0 && shortEdge(f) <= quality:
			videoOnly = append(videoOnly, f)
		case f.VCodec == "none" && f.ACodec != "none" && f.ACodec != "" && f.Ext == "m4a":
			audioOnly = append(audioOnly, f)
		case f.VCodec != "none" && f.VCodec != "" && f.ACodec != "none" && f.ACodec != "" && f.Height > 0 && shortEdge(f) <= quality:
			muxed = append(muxed, f)
		}
	}

	sort.SliceStable(videoOnly, func(i, j int) bool {
		if videoOnly[i].Height != videoOnly[j].Height {
			return videoOnly[i].Height > videoOnly[j].Height
		}
		return videoOnly[i].TBR > videoOnly[j].TBR
	})
	sort.SliceStable(audioOnly, func(i, j int) bool {
		return audioRate(audioOnly[i]) > audioRate(audioOnly[j])
	})

	if len(videoOnly) > 0 && len(audioOnly) > 0 {
		v := videoOnly[0]
		return &chosenVideo{
			Kind: kindAdaptive, VideoFormatID: v.FormatID, AudioFormatID: audioOnly[0].FormatID,
			Height: v.Height, Width: v.Width,
		}
	}

	sort.SliceStable(muxed, func(i, j int) bool {
		if muxed[i].Height != muxed[j].Height {
			return muxed[i].Height > muxed[j].Height
		}
		return mp4Rank(muxed[i]) > mp4Rank(muxed[j])
	})
	if len(muxed) > 0 {
		m := muxed[0]
		return &chosenVideo{Kind: kindMuxed, FormatID: m.FormatID, Height: m.Height, Width: m.Width}
	}

	return nil
}

func audioRate(f ytDlpFormat) float64 {
	if f.ABR > 0 {
		return f.ABR
	}
	return f.TBR
}

func mp4Rank(f ytDlpFormat) int {
	if f.Ext == "mp4" {
		return 1
	}
	return 0
}
