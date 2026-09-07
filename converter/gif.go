package converter

import (
	"image"
	"image/color"
	"image/color/palette"
	stddraw "image/draw"
	"image/gif"
	"os"

	"goformat/format"
)

func loadGIF(path string) (*gif.GIF, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return gif.DecodeAll(file)
}

//preserve every frame of GIF
func processGIFToGIF(inputPath, outPath string, targetWidth, targetHeight int, pixelart bool) error {
	g, err := loadGIF(inputPath)
	if err != nil {
		return err
	}

	//no resize
	if targetWidth == 0 && targetHeight == 0 {
		return writeGIF(outPath, g)
	}

	pal := buildPalette(g)
	canvas := image.NewRGBA(image.Rect(0, 0, g.Config.Width, g.Config.Height))
	var prevCanvas *image.RGBA
	frames := make([]*image.Paletted, len(g.Image))

	for i, frame := range g.Image {
		if g.Disposal[i] == gif.DisposalPrevious {
			snapshot := image.NewRGBA(canvas.Bounds())
			stddraw.Draw(snapshot, snapshot.Bounds(), canvas, image.Point{}, stddraw.Src)
			prevCanvas = snapshot
		}

		stddraw.Draw(canvas, frame.Bounds(), frame, frame.Bounds().Min, stddraw.Over)

		resized, err := resizeImage(canvas, targetWidth, targetHeight, pixelart)
		if err != nil {
			return err
		}

		frames[i] = quantise(resized, pal, !pixelart)

		switch g.Disposal[i] {
		case gif.DisposalBackground:
			stddraw.Draw(canvas, frame.Bounds(), image.Transparent, image.Point{}, stddraw.Src)
		case gif.DisposalPrevious:
			if prevCanvas != nil {
				canvas = prevCanvas
			}
		}
	}

	//every output frame is a full composite, so clear it before the next one
	//otherwise transparent pixels would show the previous frame through
	disposal := make([]byte, len(frames))
	for i := range disposal {
		disposal[i] = gif.DisposalBackground
	}

	out := &gif.GIF{
		Image:           frames,
		Delay:           g.Delay,
		Disposal:        disposal,
		LoopCount:       g.LoopCount,
		BackgroundIndex: transparentIndex,
	}

	return writeGIF(outPath, out)
}

//palette index reserved for fully transparent pixels
const transparentIndex = 0

//gather every opaque colour used by the source frames so resized output stays true to the original
//falls back to Plan9 when the source uses more colours than a single GIF palette can hold
func buildPalette(g *gif.GIF) color.Palette {
	pal := color.Palette{color.NRGBA{}}
	seen := make(map[color.NRGBA]bool)

	for _, frame := range g.Image {
		for _, c := range frame.Palette {
			n := color.NRGBAModel.Convert(c).(color.NRGBA)
			if n.A == 0 || seen[n] {
				continue
			}
			seen[n] = true
			pal = append(pal, n)
		}
	}

	if len(pal) > 256 {
		return append(color.Palette{color.NRGBA{}}, palette.Plan9[:255]...)
	}
	return pal
}

//map a resized frame onto pal, using binary transparency since GIF has no partial alpha
//dithering smooths the blended colours bilinear scaling produces; pixel art skips it to keep hard edges
func quantise(src image.Image, pal color.Palette, dither bool) *image.Paletted {
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

	dst := image.NewPaletted(bounds, pal)
	if dither {
		stddraw.FloydSteinberg.Draw(dst, bounds, flat, bounds.Min)
	} else {
		stddraw.Src.Draw(dst, bounds, flat, bounds.Min)
	}

	//error diffusion must not punch holes or bleed into transparent areas, so reapply the alpha mask
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			transparent := flat.NRGBAAt(x, y).A == 0
			if transparent {
				dst.SetColorIndex(x, y, transparentIndex)
			} else if dst.ColorIndexAt(x, y) == transparentIndex {
				dst.SetColorIndex(x, y, uint8(nearestOpaque(pal, flat.NRGBAAt(x, y))))
			}
		}
	}

	return dst
}

//like color.Palette.Index but never returns the transparent entry
func nearestOpaque(pal color.Palette, c color.Color) int {
	return pal[1:].Index(c) + 1
}

func writeGIF(outPath string, g *gif.GIF) error {
	outFile, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	return format.GifEncoder{}.EncodeAnimated(outFile, g)
}
