package format

import (
	"image"
	"image/jpeg"
	"io"
)

type JpegEncoder struct{}

func (JpegEncoder) Encode(w io.Writer, img image.Image, quality int) error {
	return jpeg.Encode(w, onWhite(img), &jpeg.Options{Quality: quality})
}
