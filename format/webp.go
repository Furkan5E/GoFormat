package format

import (
	"image"
	"io"

	"github.com/KarpelesLab/gowebp"
)

type WebpEncoder struct{}

func (WebpEncoder) Encode(w io.Writer, img image.Image, quality int) error {
	return gowebp.Encode(w, img, &gowebp.Options{Lossy: true, Quality: float32(quality)})
}