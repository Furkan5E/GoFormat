package format

import (
	"image"
	"image/gif"
	"io"
)

type GifEncoder struct{}

func (GifEncoder) Encode(w io.Writer, img image.Image, quality int) error {
	return gif.Encode(w, img, nil)
}