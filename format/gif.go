package format

import (
	"image"
	"image/color"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"io"
)

type GifEncoder struct{}

func (GifEncoder) Encode(w io.Writer, img image.Image, quality int) error {
	flat := Flatten(img)

	b := NewPaletteBuilder()
	for i := 0; i < len(flat.Pix) && !b.Full(); i += 4 {
		b.Add(color.NRGBA{flat.Pix[i], flat.Pix[i+1], flat.Pix[i+2], flat.Pix[i+3]})
	}

	//images that fit in one palette are mapped exactly, anything else is dithered onto the fallback
	pal, exact := b.Palette()
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

// Palette returns the collected colours, or Plan9 when they do not fit.
// exact is false when the fallback is used, as the image then needs dithering.
func (b *PaletteBuilder) Palette() (pal color.Palette, exact bool) {
	if b.Full() {
		return append(color.Palette{color.NRGBA{}}, palette.Plan9[:255]...), false
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
