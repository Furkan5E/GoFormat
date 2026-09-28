package format

import (
	"image"
	"io"

	"github.com/gen2brain/avif"
)

type AvifEncoder struct{}

func (AvifEncoder) Encode(w io.Writer, img image.Image, quality int) error {
	//every field is set because zero values are not defaults here (Speed 0 is the slowest setting)
	return avif.Encode(w, img, avif.Options{
		Quality:           quality,
		QualityAlpha:      quality,
		Speed:             avif.DefaultSpeed,
		ChromaSubsampling: image.YCbCrSubsampleRatio420,
	})
}
