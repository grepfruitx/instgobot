package userbot

import (
	"github.com/gotd/td/tg"
)

type mediaKind int

const (
	mediaPhoto mediaKind = iota
	mediaVideo
)

func bestPhotoSize(sizes []tg.PhotoSizeClass) (typ string, size int64) {
	bestArea := -1
	for _, s := range sizes {
		var w, h, sz int
		var t string
		switch v := s.(type) {
		case *tg.PhotoSize:
			w, h, t, sz = v.W, v.H, v.Type, v.Size
		case *tg.PhotoSizeProgressive:
			w, h, t = v.W, v.H, v.Type
			if len(v.Sizes) > 0 {
				sz = v.Sizes[len(v.Sizes)-1]
			}
		default:
			continue
		}
		if area := w * h; area > bestArea {
			bestArea = area
			typ, size = t, int64(sz)
		}
	}
	return typ, size
}

func extractDownloadable(m tg.MessageMediaClass) (loc tg.InputFileLocationClass, kind mediaKind, size int64, ok bool) {
	// binary split only: photo vs everything-else-is-video, no mime sniffing
	switch mm := m.(type) {
	case *tg.MessageMediaPhoto:
		photoClass, has := mm.GetPhoto()
		if !has {
			return nil, 0, 0, false
		}
		photo, ok := photoClass.AsNotEmpty()
		if !ok {
			return nil, 0, 0, false
		}
		typ, sz := bestPhotoSize(photo.Sizes)
		return photo.AsInputPhotoFileLocation(typ), mediaPhoto, sz, true

	case *tg.MessageMediaDocument:
		docClass, has := mm.GetDocument()
		if !has {
			return nil, 0, 0, false
		}
		doc, ok := docClass.AsNotEmpty()
		if !ok {
			return nil, 0, 0, false
		}
		return doc.AsInputDocumentFileLocation(""), mediaVideo, doc.Size, true

	default:
		return nil, 0, 0, false
	}
}
