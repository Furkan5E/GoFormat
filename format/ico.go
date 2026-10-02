package format

import (
	"fmt"
	"image"
	"io"

	ico "github.com/sergeymakinen/go-ico"
)

type IcoEncoder struct{}

// the ico format cannot store images larger than this
const maxIcoSize = 256

func (IcoEncoder) Encode(w io.Writer, img image.Image, quality int) error {
	b := img.Bounds()
	if b.Dx() > maxIcoSize || b.Dy() > maxIcoSize {
		return fmt.Errorf("ico images can be at most %dx%d, got %dx%d (resize with -width or -height)", maxIcoSize, maxIcoSize, b.Dx(), b.Dy())
	}
	return ico.Encode(w, img)
}
