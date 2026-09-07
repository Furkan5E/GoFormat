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

// EncodeAnimated writes a multi-frame GIF, preserving per-frame delay, disposal and loop count.
func (GifEncoder) EncodeAnimated(w io.Writer, g *gif.GIF) error {
	return gif.EncodeAll(w, g)
}