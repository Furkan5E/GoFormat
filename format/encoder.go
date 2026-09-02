package format

import (
	"fmt"
	"image"
	"io"
)

type Encoder interface {
	Encode(w io.Writer, img image.Image, quality int) error
}

var supportedExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".webp": true,
	".tiff": true,
	".bmp":  true,
}

func IsSupported(ext string) bool {
	return supportedExtensions[ext]
}

func GetEncoder(ext string) (Encoder, error) {
	switch ext {
	case "jpeg", "jpg":
		return JpegEncoder{}, nil
	case "png":
		return PngEncoder{}, nil
	case "webp":
		return WebpEncoder{}, nil
	case "tiff", "tif":
		return TiffEncoder{}, nil
	case "bmp":
		return BmpEncoder{}, nil
	default:
		return nil, fmt.Errorf("error: unsupported output format '%s'", ext)
	}
}