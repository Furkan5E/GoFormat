package format

import (
	"image"
	"image/jpeg"
	"io"
)

type JpegEncoder struct{}

func (JpegEncoder) Encode(w io.Writer, img image.Image, quality int) error {
	return jpeg.Encode(w, img, &jpeg.Options{Quality: quality})
}