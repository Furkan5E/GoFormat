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
	".tif":  true,
	".bmp":  true,
	".gif":  true,
	".ico":  true,
	".avif": true,
	".heic": true,
	".heif": true,
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
	case "gif":
		return GifEncoder{}, nil
	case "ico":
		return IcoEncoder{}, nil
	case "avif":
		return AvifEncoder{}, nil
	case "heic", "heif":
		return nil, fmt.Errorf("%s can be read but not written, choose another output format", ext)
	default:
		return nil, fmt.Errorf("unsupported output format '%s'", ext)
	}
}