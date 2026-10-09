package converter

import (
	"image"
	"image/color"
	stddraw "image/draw"
	"image/gif"
	"io"
	"os"
	"sort"

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

	plan, err := planResize(g.Config.Width, g.Config.Height, opts)
	if err != nil {
		return srcSize, outSize, err
	}
	outSize = image.Pt(plan.width, plan.height)

	if opts.Pixelart {
		if out, ok := scaleFramesExact(g, plan); ok {
			return srcSize, outSize, writeGIF(outPath, out)
		}
	}

	out, err := scaleFramesBlended(g, opts)
	if err != nil {
		return srcSize, outSize, err
	}
	return srcSize, outSize, writeGIF(outPath, out)
}

// scaleFramesExact resizes with nearest neighbour straight on the palette indices of each frame
// no colour is ever looked up or matched, so every pixel keeps its exact colour and nothing is quantised again
// frames keep their own palette, position, delay and disposal
func scaleFramesExact(g *gif.GIF, plan resizePlan) (*gif.GIF, bool) {
	//the source pixel each output column and row is taken from
	mapX := make([]int, plan.width)
	for x := range mapX {
		mapX[x] = plan.crop.Min.X + (2*x+1)*plan.crop.Dx()/(2*plan.width)
	}
	mapY := make([]int, plan.height)
	for y := range mapY {
		mapY[y] = plan.crop.Min.Y + (2*y+1)*plan.crop.Dy()/(2*plan.height)
	}
	//first output position taken from source position v or beyond
	first := func(m []int, v int) int {
		return sort.Search(len(m), func(i int) bool { return m[i] >= v })
	}

	frames := make([]*image.Paletted, len(g.Image))
	for i, frame := range g.Image {
		//a frame may cover only part of the canvas, its output is the part of the canvas that maps into it
		fb := frame.Bounds()
		rect := image.Rect(first(mapX, fb.Min.X), first(mapY, fb.Min.Y), first(mapX, fb.Max.X), first(mapY, fb.Max.Y))
		if rect.Empty() {
			//shrinking can leave a small frame with no pixels at all, which a GIF cannot store
			return nil, false
		}

		dst := image.NewPaletted(rect, frame.Palette)
		for y := rect.Min.Y; y < rect.Max.Y; y++ {
			for x := rect.Min.X; x < rect.Max.X; x++ {
				dst.Pix[dst.PixOffset(x, y)] = frame.Pix[frame.PixOffset(mapX[x], mapY[y])]
			}
		}
		frames[i] = dst
	}

	return &gif.GIF{
		Image:           frames,
		Delay:           g.Delay,
		Disposal:        g.Disposal,
		LoopCount:       g.LoopCount,
		BackgroundIndex: g.BackgroundIndex,
		Config: image.Config{
			ColorModel: g.Config.ColorModel,
			Width:      plan.width,
			Height:     plan.height,
		},
	}, true
}

// scaleFramesBlended draws the animation frame by frame, resizes each finished picture and maps it back onto a palette
// smooth scaling blends neighbouring pixels into colours the source never had, so this step cannot be avoided
func scaleFramesBlended(g *gif.GIF, opts Options) (*gif.GIF, error) {
	shared, fits := sourcePalette(g)
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

		resized, err := resizeImage(canvas, opts)
		if err != nil {
			return nil, err
		}
		flat := format.Flatten(resized)

		//the source's own colours keep the output true to the original and steady from frame to frame
		//when the frames use more colours than one palette holds, each frame gets a palette fitted to it instead
		pal, dither := shared, !opts.Pixelart
		if !fits {
			var exact bool
			pal, exact = format.PaletteFor(flat)
			dither = !exact
		}
		//dithering smooths the blended colours bilinear scaling produces; pixel art skips it to keep hard edges
		frames[i] = format.Quantise(flat, pal, dither)

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

	return &gif.GIF{
		Image:           frames,
		Delay:           g.Delay,
		Disposal:        disposal,
		LoopCount:       g.LoopCount,
		BackgroundIndex: format.TransparentIndex,
	}, nil
}

// gather every opaque colour used by the source frames, fits is false when one palette cannot hold them all
func sourcePalette(g *gif.GIF) (pal color.Palette, fits bool) {
	b := format.NewPaletteBuilder()
	for _, frame := range g.Image {
		for _, c := range frame.Palette {
			b.Add(c)
		}
	}
	return b.Palette()
}

func writeGIF(outPath string, g *gif.GIF) error {
	return writeFile(outPath, func(w io.Writer) error {
		return format.GifEncoder{}.EncodeAnimated(w, g)
	})
}
