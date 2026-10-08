package format

import (
	"image"
	"io"

	"github.com/KarpelesLab/gowebp"
)

type WebpEncoder struct{}

func (WebpEncoder) Encode(w io.Writer, img image.Image, opts Options) error {
	//lossless keeps every pixel exactly, which suits pixel art and screenshots but makes photos much larger
	return gowebp.Encode(w, img, &gowebp.Options{Lossy: !opts.Lossless, Quality: float32(opts.Quality)})
}
