package format

import (
	"fmt"
	"image"
	"image/draw"
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

// onWhite places img on a white background, for formats that cannot store transparency
// without it transparent areas come out black
func onWhite(img image.Image) image.Image {
	if o, ok := img.(interface{ Opaque() bool }); ok && o.Opaque() {
		return img
	}

	bounds := img.Bounds()
	dst := image.NewRGBA(bounds)
	draw.Draw(dst, bounds, image.White, image.Point{}, draw.Src)
	draw.Draw(dst, bounds, img, bounds.Min, draw.Over)
	return dst
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
