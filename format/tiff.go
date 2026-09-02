package format

import (
	"image"
	"io"

	"golang.org/x/image/tiff"
)

type TiffEncoder struct{}

func (TiffEncoder) Encode(w io.Writer, img image.Image, quality int) error {
	options := &tiff.Options{
		Compression: tiff.Deflate,
	}
	return tiff.Encode(w, img, options)
}