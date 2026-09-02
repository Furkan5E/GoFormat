package format

import (
	"image"
	"io"

	"golang.org/x/image/bmp"
)

type BmpEncoder struct{}

func (BmpEncoder) Encode(w io.Writer, img image.Image, quality int) error {
	return bmp.Encode(w, img)
}