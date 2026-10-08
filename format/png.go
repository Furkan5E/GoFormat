package format

import (
	"image"
	"image/png"
	"io"
)

type PngEncoder struct{}

func (PngEncoder) Encode(w io.Writer, img image.Image, opts Options) error {
	return png.Encode(w, img)
}
