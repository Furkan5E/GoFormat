package converter

import (
	"image"
	"image/color"
	stddraw "image/draw"
	"image/gif"
	"io"
	"os"

	"github.com/Furkan5E/GoFormat/v2/format"
)

func loadGIF(path string) (*gif.GIF, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return gif.DecodeAll(file)
}

// preserve every frame of GIF
func processGIFToGIF(inputPath, outPath string, opts Options) (srcSize, outSize image.Point, err error) {
	g, err := loadGIF(inputPath)
	if err != nil {
		return srcSize, outSize, err
	}
	srcSize = image.Pt(g.Config.Width, g.Config.Height)

	//no resize
	if opts.Width == 0 && opts.Height == 0 {
		return srcSize, srcSize, writeGIF(outPath, g)
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

		resized, err := resizeImage(canvas, opts.Width, opts.Height, opts.Pixelart)
		if err != nil {
			return srcSize, outSize, err
		}
		outSize = resized.Bounds().Size()

		//dithering smooths the blended colours bilinear scaling produces; pixel art skips it to keep hard edges
		frames[i] = format.Quantise(format.Flatten(resized), pal, !opts.Pixelart)

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
		BackgroundIndex: format.TransparentIndex,
	}

	return srcSize, outSize, writeGIF(outPath, out)
}

// gather every opaque colour used by the source frames so resized output stays true to the original
func buildPalette(g *gif.GIF) color.Palette {
	b := format.NewPaletteBuilder()
	for _, frame := range g.Image {
		for _, c := range frame.Palette {
			b.Add(c)
		}
	}

	pal, _ := b.Palette()
	return pal
}

func writeGIF(outPath string, g *gif.GIF) error {
	return writeFile(outPath, func(w io.Writer) error {
		return format.GifEncoder{}.EncodeAnimated(w, g)
	})
}
