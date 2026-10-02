package threads

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/grepfruitx/instgobot/internal/messages"
	"github.com/grepfruitx/instgobot/internal/store"
)

type apiMedia struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

type apiResponse struct {
	Success  bool       `json:"success"`
	Type     string     `json:"type"`
	Author   string     `json:"author"`
	PostText string     `json:"postText"`
	Media    []apiMedia `json:"media"`
	Error    string     `json:"error"`
	Code     string     `json:"code"`
}

type postContent struct {
	Photos []string
	Videos []string
	Text   string
}

func (p postContent) hasMedia() bool {
	return len(p.Photos) > 0 || len(p.Videos) > 0
}

func cachedMediaType(pc store.PostCache) string {
	if pc.PhotoCount > 0 {
		return "photo"
	}
	return "video"
}

const (
	codeNoMedia    = "NO_MEDIA"
	codeInvalidURL = "INVALID_URL"
)

type APIError struct {
	Code    string
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("threads api %s: %s", e.Code, e.Message)
}

func (e *APIError) UserFacing() string {
	switch e.Code {
	case codeNoMedia:
		return messages.ThreadsTextOnly
	case codeInvalidURL:
		return messages.ThreadsInvalidURL
	default:
		return ""
	}
}

var profilePicPathRe = regexp.MustCompile(`/t51\.\d+-19/`)

func isProfilePicture(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	if efg := u.Query().Get("efg"); efg != "" {
		if decoded, err := base64.RawStdEncoding.DecodeString(strings.TrimRight(efg, "=")); err == nil {
			if strings.Contains(string(decoded), "profile_pic") {
				return true
			}
		}
	}
	return profilePicPathRe.MatchString(u.Path)
}

func splitMedia(items []apiMedia) (photos, videos []string) {
	for _, m := range items {
		if m.URL == "" || isProfilePicture(m.URL) {
			continue
		}
		switch m.Type {
		case "video":
			videos = append(videos, m.URL)
		case "image":
			photos = append(photos, m.URL)
		}
	}
	return photos, videos
}

func parseAPIResponse(body []byte, statusCode int) (postContent, error) {
	var data apiResponse
	if jsonErr := json.Unmarshal(body, &data); jsonErr != nil {
		if statusCode < 200 || statusCode >= 300 {
			return postContent{}, fmt.Errorf("HTTP error! status: %d", statusCode)
		}
		return postContent{}, jsonErr
	}

	if !data.Success {
		if data.Code != "" {
			return postContent{}, &APIError{Code: data.Code, Message: data.Error}
		}
		if data.Error != "" {
			return postContent{}, fmt.Errorf("threads api: %s", data.Error)
		}
		return postContent{}, fmt.Errorf("HTTP error! status: %d", statusCode)
	}

	photos, videos := splitMedia(data.Media)
	return postContent{Photos: photos, Videos: videos, Text: strings.TrimSpace(data.PostText)}, nil
}
