package format

import (
	"fmt"
	"image"
	"io"
)

type Encoder interface {
	Encode(w io.Writer, img image.Image, quality int) error
}

func GetEncoder(ext string) (Encoder, error) {
	switch ext {
	case "jpeg", "jpg":
		return JpegEncoder{}, nil
	case "png":
		return PngEncoder{}, nil
	case "webp":
		return WebpEncoder{}, nil
	default:
		return nil, fmt.Errorf("error: unsupported output format '%s'", ext)
	}
}