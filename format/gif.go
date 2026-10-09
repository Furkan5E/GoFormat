package format

import (
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"io"
)

type GifEncoder struct{}

func (GifEncoder) Encode(w io.Writer, img image.Image, opts Options) error {
	flat := Flatten(img)

	//images that fit in one palette are mapped exactly, anything else is dithered onto a palette fitted to it
	pal, exact := PaletteFor(flat)
	return gif.Encode(w, Quantise(flat, pal, !exact), nil)
}

// EncodeAnimated writes a multi-frame GIF, preserving per-frame delay, disposal and loop count.
func (GifEncoder) EncodeAnimated(w io.Writer, g *gif.GIF) error {
	return gif.EncodeAll(w, g)
}

// TransparentIndex is the palette slot reserved for fully transparent pixels.
const TransparentIndex = 0

// PaletteBuilder collects the distinct opaque colours of an image into a GIF palette.
type PaletteBuilder struct {
	pal  color.Palette
	seen map[color.NRGBA]bool
}

func NewPaletteBuilder() *PaletteBuilder {
	return &PaletteBuilder{
		pal:  color.Palette{color.NRGBA{}},
		seen: make(map[color.NRGBA]bool),
	}
}

func (b *PaletteBuilder) Add(c color.Color) {
	n := color.NRGBAModel.Convert(c).(color.NRGBA)
	if n.A == 0 || b.seen[n] {
		return
	}
	b.seen[n] = true
	b.pal = append(b.pal, n)
}

// Full reports whether more colours were added than a single GIF palette can hold.
func (b *PaletteBuilder) Full() bool {
	return len(b.pal) > 256
}

// Palette returns the collected colours.
// exact is false when there were too many to fit, and only the first 255 are returned.
func (b *PaletteBuilder) Palette() (pal color.Palette, exact bool) {
	if b.Full() {
		return b.pal[:256], false
	}
	return b.pal, true
}

// Flatten converts img to binary transparency, since GIF has no partial alpha.
func Flatten(src image.Image) *image.NRGBA {
	bounds := src.Bounds()
	flat := image.NewNRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := color.NRGBAModel.Convert(src.At(x, y)).(color.NRGBA)
			if c.A < 128 {
				c = color.NRGBA{}
			} else {
				c.A = 255
			}
			flat.SetNRGBA(x, y, c)
		}
	}
	return flat
}

// Quantise maps a flattened image onto pal, keeping transparent pixels on TransparentIndex.
func Quantise(flat *image.NRGBA, pal color.Palette, dither bool) *image.Paletted {
	bounds := flat.Bounds()
	dst := image.NewPaletted(bounds, pal)
	if dither {
		draw.FloydSteinberg.Draw(dst, bounds, flat, bounds.Min)
	} else {
		draw.Src.Draw(dst, bounds, flat, bounds.Min)
	}

	//error diffusion must not punch holes or bleed into transparent areas, so reapply the alpha mask
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := flat.NRGBAAt(x, y)
			if c.A == 0 {
				dst.SetColorIndex(x, y, TransparentIndex)
			} else if dst.ColorIndexAt(x, y) == TransparentIndex {
				dst.SetColorIndex(x, y, uint8(pal[1:].Index(c)+1))
			}
		}
	}

	return dst
}
