package format

import (
	"image"
	"io"

	"golang.org/x/image/bmp"
)

type BmpEncoder struct{}

func (BmpEncoder) Encode(w io.Writer, img image.Image, opts Options) error {
	return bmp.Encode(w, onWhite(img))
}
