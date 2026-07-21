package userbot

import (
	"github.com/gotd/td/tg"
)

type mediaKind int

const (
	mediaPhoto mediaKind = iota
	mediaVideo
)

func bestPhotoSizeType(sizes []tg.PhotoSizeClass) string {
	bestType := ""
	bestArea := -1
	for _, s := range sizes {
		var w, h int
		var typ string
		switch v := s.(type) {
		case *tg.PhotoSize:
			w, h, typ = v.W, v.H, v.Type
		case *tg.PhotoSizeProgressive:
			w, h, typ = v.W, v.H, v.Type
		default:
			continue
		}
		if area := w * h; area > bestArea {
			bestArea = area
			bestType = typ
		}
	}
	return bestType
}

// extractDownloadable maps a message/story's media to a downloader-ready
// file location, mirroring the TS code's binary split: photo vs "everything
// else is sent as video" (it never inspects document mime types).
func extractDownloadable(m tg.MessageMediaClass) (loc tg.InputFileLocationClass, kind mediaKind, ok bool) {
	switch mm := m.(type) {
	case *tg.MessageMediaPhoto:
		photoClass, has := mm.GetPhoto()
		if !has {
			return nil, 0, false
		}
		photo, ok := photoClass.AsNotEmpty()
		if !ok {
			return nil, 0, false
		}
		return photo.AsInputPhotoFileLocation(bestPhotoSizeType(photo.Sizes)), mediaPhoto, true

	case *tg.MessageMediaDocument:
		docClass, has := mm.GetDocument()
		if !has {
			return nil, 0, false
		}
		doc, ok := docClass.AsNotEmpty()
		if !ok {
			return nil, 0, false
		}
		return doc.AsInputDocumentFileLocation(""), mediaVideo, true

	default:
		return nil, 0, false
	}
}
