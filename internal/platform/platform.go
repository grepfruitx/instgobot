package platform

import (
	"fmt"
	"regexp"
	"strings"
)

func IsYoutubeShortsLink(url string) bool {
	return strings.Contains(url, "youtube.com/shorts/")
}

func IsYoutubeLink(url string) bool {
	return strings.Contains(url, "youtube.com/") || strings.Contains(url, "youtu.be/")
}

func IsThreadsLink(url string) bool {
	return strings.Contains(url, "threads.com")
}

var telegramLinkRe = regexp.MustCompile(`https?://(?:t|telegram)\.me/[A-Za-z0-9_]`)

func IsTelegramLink(url string) bool {
	return telegramLinkRe.MatchString(url)
}

type TelegramLinkType string

const (
	TelegramLinkStory       TelegramLinkType = "story"
	TelegramLinkPost        TelegramLinkType = "post"
	TelegramLinkPrivatePost TelegramLinkType = "private_post"
	TelegramLinkStoriesAll  TelegramLinkType = "stories_all"
)

type TelegramLinkInfo struct {
	Type      TelegramLinkType
	Username  string
	ID        int64
	ChannelID int64
	MessageID int64
}

var (
	privatePostRe = regexp.MustCompile(`(?:t|telegram)\.me/c/(\d+)/(\d+)`)
	storyRe       = regexp.MustCompile(`(?:t|telegram)\.me/([A-Za-z0-9_]+)/s/(\d+)`)
	postRe        = regexp.MustCompile(`(?:t|telegram)\.me/([A-Za-z0-9_]+)/(\d+)`)
	usernameRe    = regexp.MustCompile(`(?:t|telegram)\.me/([A-Za-z0-9_]+)/?$`)
)

func ParseTelegramLink(url string) (*TelegramLinkInfo, bool) {
	if m := privatePostRe.FindStringSubmatch(url); m != nil {
		var channelID, messageID int64
		fmt.Sscanf("-100"+m[1], "%d", &channelID)
		fmt.Sscanf(m[2], "%d", &messageID)
		return &TelegramLinkInfo{Type: TelegramLinkPrivatePost, ChannelID: channelID, MessageID: messageID}, true
	}
	if m := storyRe.FindStringSubmatch(url); m != nil {
		var id int64
		fmt.Sscanf(m[2], "%d", &id)
		return &TelegramLinkInfo{Type: TelegramLinkStory, Username: m[1], ID: id}, true
	}
	if m := postRe.FindStringSubmatch(url); m != nil {
		var id int64
		fmt.Sscanf(m[2], "%d", &id)
		return &TelegramLinkInfo{Type: TelegramLinkPost, Username: m[1], ID: id}, true
	}
	if m := usernameRe.FindStringSubmatch(url); m != nil {
		return &TelegramLinkInfo{Type: TelegramLinkStoriesAll, Username: m[1]}, true
	}
	return nil, false
}

var instagramReservedPaths = map[string]bool{
	"p": true, "reel": true, "reels": true, "tv": true, "stories": true, "explore": true,
	"accounts": true, "direct": true, "directory": true, "developer": true, "about": true, "legal": true, "api": true, "tags": true,
}

var instagramProfileRe = regexp.MustCompile(`instagram\.com/([A-Za-z0-9_.]+)/?(?:\?\S*)?$`)

func GetInstagramProfileUsername(url string) (string, bool) {
	m := instagramProfileRe.FindStringSubmatch(url)
	if m == nil {
		return "", false
	}
	username := m[1]
	if instagramReservedPaths[strings.ToLower(username)] {
		return "", false
	}
	return username, true
}

func ToInstagramStoriesLink(username string) string {
	return fmt.Sprintf("https://www.instagram.com/stories/%s/", username)
}

func NormalizePostURL(url string) string {
	return strings.TrimSuffix(strings.Split(url, "?")[0], "/")
}

var SupportedPlatforms = []string{"tiktok", "instagram", "facebook", "twitter", "youtube", "threads", "telegram"}

func DetectPlatform(url string) string {
	switch {
	case strings.Contains(url, "tiktok.com"):
		return "tiktok"
	case strings.Contains(url, "instagram.com"):
		return "instagram"
	case strings.Contains(url, "facebook.com") || strings.Contains(url, "fb.com"):
		return "facebook"
	case strings.Contains(url, "twitter.com") || strings.Contains(url, "x.com"):
		return "twitter"
	case strings.Contains(url, "youtube.com") || strings.Contains(url, "youtu.be"):
		return "youtube"
	case strings.Contains(url, "threads.com"):
		return "threads"
	case strings.HasPrefix(url, "@") || strings.Contains(url, "t.me/") || strings.Contains(url, "telegram.me/"):
		return "telegram"
	default:
		return "unknown"
	}
}
