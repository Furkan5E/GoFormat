package format

import (
	"image"
	"image/png"
	"io"
)

type PngEncoder struct{}

func (PngEncoder) Encode(w io.Writer, img image.Image, quality int) error {
	return png.Encode(w, img)
}