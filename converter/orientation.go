package converter

import (
	"image"

	"golang.org/x/image/draw"
)

// applyOrientation turns and mirrors img the way its EXIF orientation tag says it should be displayed
// cameras often store a photo sideways and rely on the tag, which is lost on conversion
func applyOrientation(img image.Image, orientation int) image.Image {
	if orientation < 2 || orientation > 8 {
		return img
	}

	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	src := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(src, src.Bounds(), img, b.Min, draw.Src)

	//orientations 5 to 8 turn the image on its side
	dstW, dstH := w, h
	if orientation >= 5 {
		dstW, dstH = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var dx, dy int
			switch orientation {
			case 2: //mirrored left to right
				dx, dy = w-1-x, y
			case 3: //upside down
				dx, dy = w-1-x, h-1-y
			case 4: //mirrored top to bottom
				dx, dy = x, h-1-y
			case 5: //mirrored, then turned a quarter anticlockwise
				dx, dy = y, x
			case 6: //turned a quarter clockwise
				dx, dy = h-1-y, x
			case 7: //mirrored, then turned a quarter clockwise
				dx, dy = h-1-y, w-1-x
			case 8: //turned a quarter anticlockwise
				dx, dy = y, w-1-x
			}
			s, d := src.PixOffset(x, y), dst.PixOffset(dx, dy)
			copy(dst.Pix[d:d+4], src.Pix[s:s+4])
		}
	}

	return dst
}
